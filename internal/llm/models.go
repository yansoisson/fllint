package llm

import (
	"path/filepath"
	"strings"
)

// Tier represents a model capability level.
type Tier string

const (
	TierLite     Tier = "lite"
	TierStandard Tier = "standard"
	TierPro      Tier = "pro"
)

// ModelInfo describes a model available to the application.
type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Tier     Tier   `json:"tier"`
	FilePath string `json:"file_path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Active   bool   `json:"active"`
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
