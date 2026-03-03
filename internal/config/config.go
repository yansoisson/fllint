package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Config holds application configuration.
type Config struct {
	Port      int    `json:"port"`
	DataDir   string `json:"data_dir"`
	ModelsDir string `json:"models_dir"`
}

var (
	mu  sync.RWMutex
	cfg *Config
)

// Default returns the default configuration.
func Default() Config {
	return Config{
		Port:      8420,
		DataDir:   "./data",
		ModelsDir: "./models",
	}
}

// Load reads config from the data directory, or returns defaults if not found.
func Load(dataDir string) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	// Resolve to absolute path so saved config is unambiguous
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		log.Printf("Warning: could not resolve absolute path for %q: %v", dataDir, err)
		absDataDir = dataDir
	}

	path := filepath.Join(absDataDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			d := Default()
			d.DataDir = absDataDir
			cfg = &d
			return cfg, nil
		}
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	// Force DataDir from caller to prevent stale paths in saved config
	c.DataDir = absDataDir
	cfg = &c
	return cfg, nil
}

// Save writes the config to disk.
func Save(c *Config) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(c.DataDir, "config.json")
	cfg = c
	return os.WriteFile(path, data, 0644)
}

// Get returns the current loaded config.
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}
