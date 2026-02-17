package storage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/logger"
)

// JSONStore implements Storage using JSON files
type JSONStore struct {
	baseDir string
	mu      sync.RWMutex
	log     *slog.Logger
}

// NewJSONStore creates a new JSON-based storage
func NewJSONStore(baseDir string) (*JSONStore, error) {
	log := logger.L().With("component", "storage")

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}
	log.Debug("storage directory created", "path", baseDir)

	store := &JSONStore{
		baseDir: baseDir,
		log:     log,
	}

	// Ensure subdirectories exist
	for _, sub := range []string{"messages", "memory", "skills"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0755); err != nil {
			return nil, err
		}
	}
	log.Debug("storage subdirectories ensured")

	return store, nil
}

// Conversation holds a conversation's messages and metadata
type Conversation struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Messages  []llm.Message `json:"messages"`
	Summary   string        `json:"summary,omitempty"`
}

// MemoryItem represents a stored memory
type MemoryItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // fact, preference, context
	CreatedAt time.Time `json:"created_at"`
	Relevance float64   `json:"relevance"` // 0-1 relevance score
}

// Skill represents an agent skill
type Skill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Prompt      string            `json:"prompt"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// SaveConversation saves a conversation to storage
func (s *JSONStore) SaveConversation(conv *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv.UpdatedAt = time.Now()

	path := filepath.Join(s.baseDir, "messages", conv.ID+".json")
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling conversation: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.log.Debug("conversation saved", "id", conv.ID, "message_count", len(conv.Messages))
	return nil
}

// LoadConversation loads a conversation by ID
func (s *JSONStore) LoadConversation(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, "messages", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading conversation: %w", err)
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("unmarshaling conversation: %w", err)
	}

	s.log.Debug("conversation loaded", "id", id, "message_count", len(conv.Messages))
	return &conv, nil
}

// ListConversations returns all conversation IDs
func (s *JSONStore) ListConversations() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.baseDir, "messages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			ids = append(ids, entry.Name()[:len(entry.Name())-5])
		}
	}
	return ids, nil
}

// DeleteConversation removes a conversation
func (s *JSONStore) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, "messages", id+".json")
	if err := os.Remove(path); err != nil {
		return err
	}

	s.log.Info("conversation deleted", "id", id)
	return nil
}

// SaveMemory stores a memory item
func (s *JSONStore) SaveMemory(item *MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, "memory", item.ID+".json")
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.log.Debug("memory saved", "id", item.ID, "type", item.Type)
	return nil
}

// LoadMemories loads all memory items
func (s *JSONStore) LoadMemories() ([]*MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.baseDir, "memory")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var memories []*MemoryItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var item MemoryItem
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}

		memories = append(memories, &item)
	}

	s.log.Debug("memories loaded", "count", len(memories))
	return memories, nil
}

// DeleteMemory removes a memory item
func (s *JSONStore) DeleteMemory(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, "memory", id+".json")
	if err := os.Remove(path); err != nil {
		return err
	}

	s.log.Info("memory deleted", "id", id)
	return nil
}

// SaveSkill stores a skill
func (s *JSONStore) SaveSkill(skill *Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, "skills", skill.ID+".json")
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.log.Debug("skill saved", "id", skill.ID, "name", skill.Name)
	return nil
}

// LoadSkills loads all skills
func (s *JSONStore) LoadSkills() ([]*Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.baseDir, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			continue
		}

		skills = append(skills, &skill)
	}

	s.log.Debug("skills loaded", "count", len(skills))
	return skills, nil
}

// DeleteSkill removes a skill
func (s *JSONStore) DeleteSkill(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, "skills", id+".json")
	if err := os.Remove(path); err != nil {
		return err
	}

	s.log.Info("skill deleted", "id", id)
	return nil
}
