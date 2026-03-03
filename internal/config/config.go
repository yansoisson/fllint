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
	// Core
	Port      int    `json:"port"`
	DataDir   string `json:"data_dir"`
	ModelsDir string `json:"models_dir"`

	// General
	Theme   string `json:"theme"`    // "light", "dark", "system"
	ProMode bool   `json:"pro_mode"`

	// Chat Behavior
	CustomInstructions string `json:"custom_instructions"`
	SystemPrompt       string `json:"system_prompt"` // empty = use default

	// Inference Parameters
	Temperature   float64 `json:"temperature"`
	TopP          float64 `json:"top_p"`
	TopK          int     `json:"top_k"`
	RepeatPenalty float64 `json:"repeat_penalty"`
	MaxTokens     int     `json:"max_tokens"` // 0 = unlimited
	Seed          int     `json:"seed"`       // -1 = random

	// Server / Engine
	CtxSize    int    `json:"ctx_size"`
	NGPULayers int    `json:"n_gpu_layers"` // 999 = auto
	FlashAttn  string `json:"flash_attn"`   // "auto", "on", "off"
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

		Theme:   "system",
		ProMode: false,

		Temperature:   0.7,
		TopP:          0.95,
		TopK:          40,
		RepeatPenalty: 1.1,
		MaxTokens:     0,
		Seed:          -1,

		CtxSize:    4096,
		NGPULayers: 999,
		FlashAttn:  "auto",
	}
}

// WithDefaults fills zero-valued fields with their default values.
// This ensures backward compatibility when loading older config files
// that don't have the new settings fields.
func (c *Config) WithDefaults() {
	d := Default()
	if c.Theme == "" {
		c.Theme = d.Theme
	}
	if c.Temperature == 0 {
		c.Temperature = d.Temperature
	}
	if c.TopP == 0 {
		c.TopP = d.TopP
	}
	if c.TopK == 0 {
		c.TopK = d.TopK
	}
	if c.RepeatPenalty == 0 {
		c.RepeatPenalty = d.RepeatPenalty
	}
	// MaxTokens: 0 is valid (unlimited), no default needed
	// Seed: 0 is ambiguous — treat as needing default since -1 is the intended default
	if c.Seed == 0 {
		c.Seed = d.Seed
	}
	if c.CtxSize == 0 {
		c.CtxSize = d.CtxSize
	}
	if c.NGPULayers == 0 {
		c.NGPULayers = d.NGPULayers
	}
	if c.FlashAttn == "" {
		c.FlashAttn = d.FlashAttn
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
	c.WithDefaults()
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
