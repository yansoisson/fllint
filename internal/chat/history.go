package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fllint/fllint/internal/llm"
	"github.com/google/uuid"
)

// Store manages conversation persistence as JSON files.
type Store struct {
	mu  sync.RWMutex
	dir string
}

func NewStore(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "conversations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create conversations dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Create(title string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		Messages:  []llm.ChatMessage{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.save(conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *Store) Get(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.load(id)
}

func (s *Store) List() ([]Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var convs []Conversation
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		c, err := s.load(id)
		if err != nil {
			continue
		}
		convs = append(convs, *c)
	}

	sort.Slice(convs, func(i, j int) bool {
		return convs[i].UpdatedAt.After(convs[j].UpdatedAt)
	})
	return convs, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// DeleteAll removes all conversations from disk.
func (s *Store) DeleteAll() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil {
			return count, fmt.Errorf("failed to delete %s: %w", e.Name(), err)
		}
		count++
	}
	return count, nil
}

// Count returns the number of stored conversations.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count
}

// SetModelID sets the model ID for a conversation.
func (s *Store) SetModelID(id string, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, err := s.load(id)
	if err != nil {
		return err
	}
	conv.ModelID = modelID
	conv.UpdatedAt = time.Now()
	return s.save(conv)
}

// AppendMessage adds a message to a conversation and persists it.
func (s *Store) AppendMessage(id string, msg llm.ChatMessage) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, err := s.load(id)
	if err != nil {
		return nil, err
	}
	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()
	if err := s.save(conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *Store) load(id string) (*Conversation, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

func (s *Store) save(conv *Conversation) error {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, conv.ID+".json")
	return os.WriteFile(path, data, 0644)
}
