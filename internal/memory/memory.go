package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/logger"
	"github.com/igm/igent/internal/storage"
)

// Manager handles context and memory optimization
type Manager struct {
	store         *storage.JSONStore
	provider      llm.Provider
	maxMessages   int
	maxTokens     int
	summarizeWhen int
	log           *slog.Logger
}

// NewManager creates a new memory manager
func NewManager(store *storage.JSONStore, provider llm.Provider, maxMessages, maxTokens, summarizeWhen int) *Manager {
	return &Manager{
		store:         store,
		provider:      provider,
		maxMessages:   maxMessages,
		maxTokens:     maxTokens,
		summarizeWhen: summarizeWhen,
		log:           logger.L().With("component", "memory"),
	}
}

// BuildContext builds the optimal context for a new query
func (m *Manager) BuildContext(conv *storage.Conversation, userMessage string) ([]llm.Message, error) {
	m.log.Debug("building context", "conversation_id", conv.ID)
	var context []llm.Message

	// 1. Start with relevant memories
	memories, err := m.getRelevantMemories(userMessage)
	if err == nil && len(memories) > 0 {
		m.log.Debug("relevant memories found", "count", len(memories))
		memoryContext := m.formatMemories(memories)
		if memoryContext != "" {
			context = append(context, llm.Message{
				Role:    "system",
				Content: "Relevant context from memory:\n" + memoryContext,
			})
		}
	}

	// 2. Add conversation summary if available
	if conv.Summary != "" {
		m.log.Debug("using conversation summary")
		context = append(context, llm.Message{
			Role:    "system",
			Content: "Previous conversation summary: " + conv.Summary,
		})
	}

	// 3. Add recent messages (sliding window)
	recentMessages := m.getRecentMessages(conv.Messages, userMessage)
	context = append(context, recentMessages...)
	m.log.Debug("recent messages added", "count", len(recentMessages))

	// 4. Check if we need summarization
	if len(conv.Messages) >= m.summarizeWhen {
		m.log.Info("summarization threshold reached, triggering async summarization",
			"message_count", len(conv.Messages),
			"threshold", m.summarizeWhen,
		)
		go m.summarizeConversation(conv) // Async summarization
	}

	return context, nil
}

// getRelevantMemories retrieves memories relevant to the query
func (m *Manager) getRelevantMemories(query string) ([]*storage.MemoryItem, error) {
	memories, err := m.store.LoadMemories()
	if err != nil {
		return nil, err
	}

	// Simple keyword-based relevance scoring
	// In production, this could use embeddings
	queryLower := strings.ToLower(query)
	var relevant []*storage.MemoryItem

	for _, mem := range memories {
		if mem.Relevance < 0.3 {
			continue
		}

		contentLower := strings.ToLower(mem.Content)
		score := 0.0

		// Check for keyword matches
		queryWords := strings.Fields(queryLower)
		for _, word := range queryWords {
			if len(word) > 3 && strings.Contains(contentLower, word) {
				score += 0.2
			}
		}

		// Boost by stored relevance
		score = score * mem.Relevance

		if score > 0.1 {
			relevant = append(relevant, mem)
		}
	}

	// Sort by relevance
	sort.Slice(relevant, func(i, j int) bool {
		return relevant[i].Relevance > relevant[j].Relevance
	})

	// Limit to top 5 memories
	if len(relevant) > 5 {
		relevant = relevant[:5]
	}

	return relevant, nil
}

// formatMemories formats memories for context
func (m *Manager) formatMemories(memories []*storage.MemoryItem) string {
	var parts []string
	for _, mem := range memories {
		parts = append(parts, fmt.Sprintf("- [%s] %s", mem.Type, mem.Content))
	}
	return strings.Join(parts, "\n")
}

// getRecentMessages returns the most recent messages within token limits
func (m *Manager) getRecentMessages(messages []llm.Message, newUserMessage string) []llm.Message {
	// Always include the new user message
	result := []llm.Message{{Role: "user", Content: newUserMessage}}

	// Calculate remaining token budget
	newMsgTokens := m.provider.CountTokens(result)
	budget := m.maxTokens - newMsgTokens - 500 // Reserve for response

	// Add messages from newest to oldest until budget is exceeded
	recent := make([]llm.Message, 0)
	tokenCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := m.provider.CountTokens([]llm.Message{messages[i]})
		if tokenCount+msgTokens > budget {
			break
		}
		recent = append([]llm.Message{messages[i]}, recent...)
		tokenCount += msgTokens

		if len(recent) >= m.maxMessages {
			break
		}
	}

	return append(recent, result...)
}

// summarizeConversation creates a summary of old messages
func (m *Manager) summarizeConversation(conv *storage.Conversation) {
	if len(conv.Messages) < m.summarizeWhen {
		return
	}

	m.log.Info("starting conversation summarization",
		"conversation_id", conv.ID,
		"message_count", len(conv.Messages),
	)

	// Keep last 10 messages, summarize the rest
	keepCount := 10
	if len(conv.Messages) <= keepCount {
		return
	}

	toSummarize := conv.Messages[:len(conv.Messages)-keepCount]
	m.log.Debug("messages to summarize", "count", len(toSummarize))

	summarizePrompt := []llm.Message{
		{
			Role:    "system",
			Content: "Summarize the following conversation concisely, preserving key facts, decisions, and context. Be brief but comprehensive.",
		},
		{
			Role:    "user",
			Content: formatMessagesForSummary(toSummarize),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	resp, err := m.provider.Complete(ctx, summarizePrompt)
	if err != nil {
		m.log.Error("summarization failed", "error", err)
		return
	}

	// Update conversation with summary
	conv.Summary = resp.Content
	conv.Messages = conv.Messages[len(conv.Messages)-keepCount:]
	m.store.SaveConversation(conv)

	m.log.Info("summarization completed",
		"conversation_id", conv.ID,
		"summary_length", len(resp.Content),
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	// Store important facts as memories
	m.extractMemories(conv, toSummarize)
}

// formatMessagesForSummary formats messages for summarization
func formatMessagesForSummary(messages []llm.Message) string {
	var parts []string
	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	return strings.Join(parts, "\n\n")
}

// extractMemories extracts important information from summarized messages
func (m *Manager) extractMemories(conv *storage.Conversation, messages []llm.Message) {
	m.log.Debug("extracting memories from summarized messages")

	extractPrompt := []llm.Message{
		{
			Role: "system",
			Content: `Extract important facts, preferences, or context from this conversation that should be remembered for future interactions.
Return each fact on a new line, prefixed with its type (fact/preference/context).
Example:
fact: User's name is Alice
preference: User prefers concise responses
context: Working on a Go project`,
		},
		{
			Role:    "user",
			Content: formatMessagesForSummary(messages),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := m.provider.Complete(ctx, extractPrompt)
	if err != nil {
		m.log.Error("memory extraction failed", "error", err)
		return
	}

	// Parse and store memories
	lines := strings.Split(resp.Content, "\n")
	extractedCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		memType := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])

		if memType != "fact" && memType != "preference" && memType != "context" {
			memType = "fact"
		}

		memory := &storage.MemoryItem{
			ID:        generateID(),
			Content:   content,
			Type:      memType,
			CreatedAt: time.Now(),
			Relevance: 0.7,
		}

		if err := m.store.SaveMemory(memory); err != nil {
			m.log.Error("failed to save memory", "error", err, "type", memType)
			continue
		}
		extractedCount++
	}

	if extractedCount > 0 {
		m.log.Info("memories extracted", "count", extractedCount)
	}
}

// AddMemory adds a new memory manually
func (m *Manager) AddMemory(content, memType string) error {
	memory := &storage.MemoryItem{
		ID:        generateID(),
		Content:   content,
		Type:      memType,
		CreatedAt: time.Now(),
		Relevance: 1.0,
	}
	if err := m.store.SaveMemory(memory); err != nil {
		return err
	}
	m.log.Info("memory added", "type", memType, "content_length", len(content))
	return nil
}

// generateID generates a simple unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
