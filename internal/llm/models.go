package llm

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Tier represents a model capability level.
type Tier string

const (
	TierLite     Tier = "lite"
	TierStandard Tier = "standard"
	TierPro      Tier = "pro"
	TierCustom   Tier = "custom"
	TierExternal Tier = "external"
)

// ModelInfo describes a model available to the application.
type ModelInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Tier       Tier   `json:"tier"`
	FilePath   string `json:"file_path,omitempty"`
	MmprojPath string `json:"mmproj_path,omitempty"` // path to vision projector file
	Size       int64  `json:"size,omitempty"`
	Active     bool   `json:"active"`
	Loaded     bool   `json:"loaded"`                // true if engine is currently running
	Vision     bool   `json:"vision"`                // true if mmproj file is available
	External   bool   `json:"external"`              // true for models from external providers
	ProviderID string `json:"provider_id,omitempty"` // which provider this comes from
}

// modelNameFromFilename derives a human-readable name from a GGUF filename.
// Example: "qwen2.5-1.5b-instruct-Q4_K_M.gguf" → "Qwen2 5 1 5b Instruct Q4 K M"
func modelNameFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")

	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 && w[0] >= 'a' && w[0] <= 'z' {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// ModelMeta holds user-facing metadata stored in model.json alongside the
// .gguf files. The struct is designed to be extended with new fields over time.
type ModelMeta struct {
	Name string `json:"name"`
}

const modelMetaFile = "model.json"

// loadOrCreateModelMeta reads model.json from dir. If the file does not exist
// it creates one with defaultName and returns it.
func loadOrCreateModelMeta(dir string, defaultName string) ModelMeta {
	metaPath := filepath.Join(dir, modelMetaFile)

	data, err := os.ReadFile(metaPath)
	if err == nil {
		var meta ModelMeta
		if err := json.Unmarshal(data, &meta); err == nil && meta.Name != "" {
			return meta
		}
	}

	// File missing or invalid — create with defaults
	meta := ModelMeta{Name: defaultName}
	out, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		if err := os.WriteFile(metaPath, append(out, '\n'), 0644); err != nil {
			log.Printf("Could not write %s: %v", metaPath, err)
		}
	}
	return meta
}

// tierFromDirName returns a tier based on the subdirectory name.
// Only directories named exactly "Lite", "Standard", or "Pro" (case-insensitive)
// get tier labels. Returns empty string for all other names.
func tierFromDirName(dirName string) Tier {
	switch strings.ToLower(dirName) {
	case "lite":
		return TierLite
	case "standard":
		return TierStandard
	case "pro":
		return TierPro
	default:
		return ""
	}
}

// tierFromSize classifies a model by file size.
//
//	<5 GB  → Lite
//	5–15 GB → Standard
//	>15 GB  → Pro
func tierFromSize(sizeBytes int64) Tier {
	const gb = 1024 * 1024 * 1024
	switch {
	case sizeBytes < 5*gb:
		return TierLite
	case sizeBytes < 15*gb:
		return TierStandard
	default:
		return TierPro
	}
}
