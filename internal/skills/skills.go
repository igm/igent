package skills

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/igm/igent/internal/logger"
	"github.com/igm/igent/internal/storage"
)

// Registry manages available skills
type Registry struct {
	store  *storage.JSONStore
	skills map[string]*storage.Skill
	mu     sync.RWMutex
	log    *slog.Logger
}

// NewRegistry creates a new skill registry
func NewRegistry(store *storage.JSONStore) (*Registry, error) {
	log := logger.L().With("component", "skills")

	r := &Registry{
		store:  store,
		skills: make(map[string]*storage.Skill),
		log:    log,
	}

	// Load existing skills
	skills, err := store.LoadSkills()
	if err != nil {
		return nil, err
	}

	for _, skill := range skills {
		if skill.Enabled {
			r.skills[skill.ID] = skill
			log.Debug("skill loaded", "id", skill.ID, "name", skill.Name)
		}
	}

	if len(r.skills) > 0 {
		log.Info("skills loaded from storage", "count", len(r.skills))
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
	r.log.Info("skill registered", "id", skill.ID, "name", skill.Name, "enabled", skill.Enabled)
	return nil
}

// Unregister removes a skill
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.store.DeleteSkill(id); err != nil {
		return err
	}

	skillName := ""
	if skill, ok := r.skills[id]; ok {
		skillName = skill.Name
	}
	delete(r.skills, id)
	r.log.Info("skill unregistered", "id", id, "name", skillName)
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
			r.log.Debug("skill matched by name", "id", skill.ID, "name", skill.Name)
			continue
		}

		// Check trigger patterns
		for key := range skill.Parameters {
			if pattern, ok := skill.Parameters["trigger_"+key]; ok {
				if matched, _ := regexp.MatchString(pattern, input); matched {
					matches = append(matches, skill)
					r.log.Debug("skill matched by pattern", "id", skill.ID, "pattern_key", key)
					break
				}
			}
		}
	}

	if len(matches) > 0 {
		r.log.Debug("skills matched", "count", len(matches))
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
	var skillNames []string
	for _, skill := range matches {
		enhancements = append(enhancements, skill.Prompt)
		skillNames = append(skillNames, skill.Name)
	}

	r.log.Info("prompt enhanced with skills", "skills", strings.Join(skillNames, ", "))

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
		r.log.Debug("skills already exist, skipping defaults initialization")
		return nil
	}

	r.log.Info("initializing default skills")
	for _, skill := range DefaultSkills() {
		if err := r.Register(skill); err != nil {
			return fmt.Errorf("registering skill %s: %w", skill.ID, err)
		}
	}

	return nil
}
