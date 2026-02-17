package storage

import "errors"

var (
	// ErrNotFound indicates the requested item was not found
	ErrNotFound = errors.New("not found")
)

// Storage defines the interface for data persistence
type Storage interface {
	// Conversation management
	SaveConversation(conv *Conversation) error
	LoadConversation(id string) (*Conversation, error)
	ListConversations() ([]string, error)
	DeleteConversation(id string) error

	// Memory management
	SaveMemory(item *MemoryItem) error
	LoadMemories() ([]*MemoryItem, error)
	DeleteMemory(id string) error

	// Skill management
	SaveSkill(skill *Skill) error
	LoadSkills() ([]*Skill, error)
	DeleteSkill(id string) error
}
