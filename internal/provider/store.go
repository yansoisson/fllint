package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const storeFile = "providers.json"

// Store manages provider configurations persisted to disk.
type Store struct {
	mu        sync.RWMutex
	dataDir   string
	providers []Provider
}

// NewStore creates a Store, loading existing providers from disk.
func NewStore(dataDir string) (*Store, error) {
	s := &Store{dataDir: dataDir}

	path := filepath.Join(dataDir, storeFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read providers: %w", err)
	}

	if err := json.Unmarshal(data, &s.providers); err != nil {
		log.Printf("Warning: could not parse %s, starting with empty providers: %v", storeFile, err)
		s.providers = nil
	}

	// Ensure Models slices are non-nil
	for i := range s.providers {
		if s.providers[i].Models == nil {
			s.providers[i].Models = []SelectedModel{}
		}
	}

	log.Printf("Loaded %d provider(s) from %s", len(s.providers), storeFile)
	return s, nil
}

// List returns all providers.
func (s *Store) List() []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Provider, len(s.providers))
	copy(result, s.providers)
	return result
}

// Get returns a provider by ID.
func (s *Store) Get(id string) (*Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.providers {
		if s.providers[i].ID == id {
			p := s.providers[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("Provider %q not found.", id)
}

// Create validates and adds a new provider, assigns a UUID, and persists.
func (s *Store) Create(p Provider) (*Provider, error) {
	if err := validate(p); err != nil {
		return nil, err
	}

	p.ID = uuid.New().String()
	p.BaseURL = normalizeURL(p.BaseURL)
	if p.Models == nil {
		p.Models = []SelectedModel{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers = append(s.providers, p)
	if err := s.save(); err != nil {
		// Roll back
		s.providers = s.providers[:len(s.providers)-1]
		return nil, err
	}

	result := p
	return &result, nil
}

// Update replaces a provider's configuration and persists.
// If APIKey is empty in the update, the existing key is preserved.
func (s *Store) Update(p Provider) (*Provider, error) {
	if err := validate(p); err != nil {
		return nil, err
	}

	p.BaseURL = normalizeURL(p.BaseURL)
	if p.Models == nil {
		p.Models = []SelectedModel{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.providers {
		if s.providers[i].ID == p.ID {
			// Preserve existing API key if not provided in update
			if p.APIKey == "" {
				p.APIKey = s.providers[i].APIKey
			}
			s.providers[i] = p
			if err := s.save(); err != nil {
				return nil, err
			}
			result := p
			return &result, nil
		}
	}

	return nil, fmt.Errorf("Provider %q not found.", p.ID)
}

// Delete removes a provider and persists.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.providers {
		if s.providers[i].ID == id {
			s.providers = append(s.providers[:i], s.providers[i+1:]...)
			return s.save()
		}
	}

	return fmt.Errorf("Provider %q not found.", id)
}

// SetModels updates the selected models for a provider and persists.
func (s *Store) SetModels(id string, models []SelectedModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.providers {
		if s.providers[i].ID == id {
			if models == nil {
				models = []SelectedModel{}
			}
			s.providers[i].Models = models
			return s.save()
		}
	}

	return fmt.Errorf("Provider %q not found.", id)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.providers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal providers: %w", err)
	}
	path := filepath.Join(s.dataDir, storeFile)
	return os.WriteFile(path, data, 0644)
}

func validate(p Provider) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("Provider name is required.")
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return fmt.Errorf("Provider URL is required.")
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("Provider URL must start with http:// or https://")
	}

	// Check if type requires an API key
	for _, t := range RegisteredTypes() {
		if t.Type == p.Type && t.RequiresKey && p.APIKey == "" {
			return fmt.Errorf("API key is required for %s.", t.Label)
		}
	}

	return nil
}

func normalizeURL(rawURL string) string {
	rawURL = strings.TrimRight(rawURL, "/")
	// Upgrade known cloud hosts to HTTPS (HTTP causes redirect failures with POST)
	if strings.HasPrefix(rawURL, "http://ollama.com") {
		rawURL = "https" + rawURL[4:]
	}
	return rawURL
}
