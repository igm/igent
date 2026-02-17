package skills

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/igm/igent/internal/storage"
)

// Registry manages available skills
type Registry struct {
	store  *storage.JSONStore
	skills map[string]*storage.Skill
	mu     sync.RWMutex
}

// NewRegistry creates a new skill registry
func NewRegistry(store *storage.JSONStore) (*Registry, error) {
	r := &Registry{
		store:  store,
		skills: make(map[string]*storage.Skill),
	}

	// Load existing skills
	skills, err := store.LoadSkills()
	if err != nil {
		return nil, err
	}

	for _, skill := range skills {
		if skill.Enabled {
			r.skills[skill.ID] = skill
		}
	}

	return r, nil
}

// Get retrieves a skill by ID
func (r *Registry) Get(id string) (*storage.Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[id]
	return skill, ok
}

// List returns all enabled skills
func (r *Registry) List() []*storage.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*storage.Skill
	for _, skill := range r.skills {
		result = append(result, skill)
	}
	return result
}

// Register adds or updates a skill
func (r *Registry) Register(skill *storage.Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.store.SaveSkill(skill); err != nil {
		return err
	}

	r.skills[skill.ID] = skill
	return nil
}

// Unregister removes a skill
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.store.DeleteSkill(id); err != nil {
		return err
	}

	delete(r.skills, id)
	return nil
}

// Match finds skills that match the input
func (r *Registry) Match(input string) []*storage.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inputLower := strings.ToLower(input)
	var matches []*storage.Skill

	for _, skill := range r.skills {
		if !skill.Enabled {
			continue
		}

		// Check name match
		if strings.Contains(inputLower, strings.ToLower(skill.Name)) {
			matches = append(matches, skill)
			continue
		}

		// Check trigger patterns
		for key := range skill.Parameters {
			if pattern, ok := skill.Parameters["trigger_"+key]; ok {
				if matched, _ := regexp.MatchString(pattern, input); matched {
					matches = append(matches, skill)
					break
				}
			}
		}
	}

	return matches
}

// EnhancePrompt adds skill context to a prompt
func (r *Registry) EnhancePrompt(input string, basePrompt string) string {
	matches := r.Match(input)
	if len(matches) == 0 {
		return basePrompt
	}

	var enhancements []string
	for _, skill := range matches {
		enhancements = append(enhancements, skill.Prompt)
	}

	if basePrompt != "" {
		return basePrompt + "\n\nAdditional context from skills:\n" + strings.Join(enhancements, "\n")
	}

	return strings.Join(enhancements, "\n")
}

// DefaultSkills returns built-in skills
func DefaultSkills() []*storage.Skill {
	return []*storage.Skill{
		{
			ID:          "code",
			Name:        "Code Assistant",
			Description: "Helps with coding tasks",
			Prompt:      "When discussing code, provide clear explanations and well-structured examples. Follow best practices for the relevant language.",
			Enabled:     true,
		},
		{
			ID:          "explain",
			Name:        "Explainer",
			Description: "Provides detailed explanations",
			Prompt:      "When asked to explain something, break it down into clear steps. Use analogies when helpful.",
			Enabled:     true,
		},
		{
			ID:          "summarize",
			Name:        "Summarizer",
			Description: "Creates concise summaries",
			Prompt:      "When summarizing, capture key points and main ideas. Be concise but comprehensive.",
			Enabled:     true,
		},
	}
}

// InitializeDefaults adds default skills if none exist
func (r *Registry) InitializeDefaults() error {
	existing := r.List()
	if len(existing) > 0 {
		return nil
	}

	for _, skill := range DefaultSkills() {
		if err := r.Register(skill); err != nil {
			return fmt.Errorf("registering skill %s: %w", skill.ID, err)
		}
	}

	return nil
}
