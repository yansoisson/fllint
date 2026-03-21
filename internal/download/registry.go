package download

import (
	"os"
	"path/filepath"
)

// RegistryModel describes a downloadable model from the official registry.
type RegistryModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"` // "main" or "helper"
	Tier        string `json:"tier"`     // "lite", "standard", "pro", "helper"
	URL         string `json:"url"`
	Size        int64  `json:"size"`     // expected file size in bytes
	SHA256      string `json:"sha256"`   // optional integrity hash (future)
	Filename    string `json:"filename"` // target filename, e.g. "model.gguf"
	DirName     string `json:"dir_name"` // subdirectory under modelsDir, e.g. "Standard"
	MmprojURL   string `json:"mmproj_url,omitempty"`
	MmprojSize  int64  `json:"mmproj_size,omitempty"`
	MmprojName  string `json:"mmproj_name,omitempty"`
}

// RegistryEntry is a RegistryModel enriched with download status.
type RegistryEntry struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	Category      string `json:"category"`
	Tier          string `json:"tier"`
	Size          int64  `json:"size"`
	Downloaded    bool   `json:"downloaded"`
	MmprojMissing bool   `json:"mmproj_missing,omitempty"`
	MmprojSize    int64  `json:"mmproj_size,omitempty"`
}

// Registry returns the list of official downloadable models.
// URLs are placeholders — replace with actual HuggingFace links when ready.
func Registry() []RegistryModel {
	return []RegistryModel{
		{
			ID:          "lite-qwen3.5-2b",
			DisplayName: "Lite Model",
			Category:    "main",
			Tier:        "lite",
			URL:         "https://huggingface.co/unsloth/Qwen3.5-2B-GGUF/resolve/main/Qwen3.5-2B-Q8_0.gguf?download=true",
			Size:        2_012_012_800,
			Filename:    "Qwen3.5-2B-Q8_0.gguf",
			DirName:     "Lite",
			MmprojURL:   "https://huggingface.co/unsloth/Qwen3.5-2B-GGUF/resolve/main/mmproj-BF16.gguf?download=true",
			MmprojSize:  671_372_992,
			MmprojName:  "mmproj-BF16.gguf",
		},
		{
			ID:          "standard-qwen3.5-9b",
			DisplayName: "Standard Model",
			Category:    "main",
			Tier:        "standard",
			URL:         "https://huggingface.co/unsloth/Qwen3.5-9B-GGUF/resolve/main/Qwen3.5-9B-Q8_0.gguf?download=true",
			Size:        9_527_502_048,
			Filename:    "Qwen3.5-9B-Q8_0.gguf",
			DirName:     "Standard",
			MmprojURL:   "https://huggingface.co/unsloth/Qwen3.5-9B-GGUF/resolve/main/mmproj-BF16.gguf?download=true",
			MmprojSize:  921_705_024,
			MmprojName:  "mmproj-BF16.gguf",
		},
		{
			ID:          "pro-qwen3.5-27b",
			DisplayName: "Pro Model",
			Category:    "main",
			Tier:        "pro",
			URL:         "https://huggingface.co/unsloth/Qwen3.5-27B-GGUF/resolve/main/Qwen3.5-27B-Q6_K.gguf?download=true",
			Size:        22_453_933_984,
			Filename:    "Qwen3.5-27B-Q6_K.gguf",
			DirName:     "Pro",
			MmprojURL:   "https://huggingface.co/unsloth/Qwen3.5-27B-GGUF/resolve/main/mmproj-BF16.gguf?download=true",
			MmprojSize:  931_145_984,
			MmprojName:  "mmproj-BF16.gguf",
		},
		{
			ID:          "helper-summary-qwen3.5-0.8b",
			DisplayName: "Summary Model",
			Category:    "helper",
			Tier:        "helper",
			URL:         "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_0.gguf?download=true",
			Size:        507_154_688,
			Filename:    "Qwen3.5-0.8B-Q4_0.gguf",
			DirName:     "Helper-hewrow-Nipju6-mecnop/Summary",
		},
		{
			ID:          "helper-ocr-glm-ocr",
			DisplayName: "OCR Model (GLM-OCR)",
			Category:    "helper",
			Tier:        "helper",
			URL:         "https://huggingface.co/ggml-org/GLM-OCR-GGUF/resolve/main/GLM-OCR-Q8_0.gguf?download=true",
			Size:        950_433_408,
			Filename:    "GLM-OCR-Q8_0.gguf",
			DirName:     "Helper-hewrow-Nipju6-mecnop/OCR",
			MmprojURL:   "https://huggingface.co/ggml-org/GLM-OCR-GGUF/resolve/main/mmproj-GLM-OCR-Q8_0.gguf?download=true",
			MmprojSize:  484_403_648,
			MmprojName:  "mmproj-GLM-OCR-Q8_0.gguf",
		},
	}
}

// registryByID returns a RegistryModel by its ID, or nil if not found.
func registryByID(id string) *RegistryModel {
	for _, m := range Registry() {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

// CheckDownloaded returns registry entries enriched with download status.
func CheckDownloaded(modelsDir string, models []RegistryModel) []RegistryEntry {
	entries := make([]RegistryEntry, 0, len(models))
	for _, m := range models {
		entry := RegistryEntry{
			ID:          m.ID,
			DisplayName: m.DisplayName,
			Category:    m.Category,
			Tier:        m.Tier,
			Size:        m.Size,
			MmprojSize:  m.MmprojSize,
		}
		entry.Downloaded = isDownloaded(modelsDir, m)
		if !entry.Downloaded {
			entry.MmprojMissing = isMmprojMissing(modelsDir, m)
		}
		entries = append(entries, entry)
	}
	return entries
}

// isDownloaded checks if the model file exists on disk with approximately the right size.
func isDownloaded(modelsDir string, model RegistryModel) bool {
	path := filepath.Join(modelsDir, model.DirName, model.Filename)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Accept if file exists and is within 1% of expected size (different quantization builds
	// may vary slightly). If expected size is 0, just check existence.
	if model.Size == 0 {
		if info.Size() <= 0 {
			return false
		}
	} else {
		tolerance := model.Size / 100
		diff := info.Size() - model.Size
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return false
		}
	}

	// Check that no .partial files remain (interrupted download)
	if _, err := os.Stat(path + ".partial"); err == nil {
		return false
	}

	// Check mmproj if the model requires one
	if model.MmprojName != "" {
		mmprojPath := filepath.Join(modelsDir, model.DirName, model.MmprojName)
		mmprojInfo, err := os.Stat(mmprojPath)
		if err != nil {
			return false
		}
		if model.MmprojSize > 0 {
			tolerance := model.MmprojSize / 100
			diff := mmprojInfo.Size() - model.MmprojSize
			if diff < 0 {
				diff = -diff
			}
			if diff > tolerance {
				return false
			}
		}
		// Check no .partial leftover for mmproj
		if _, err := os.Stat(mmprojPath + ".partial"); err == nil {
			return false
		}
	}

	return true
}

// isMmprojMissing checks if the main model is downloaded but mmproj is missing or incomplete.
func isMmprojMissing(modelsDir string, model RegistryModel) bool {
	if model.MmprojName == "" {
		return false
	}

	// Main model must exist
	path := filepath.Join(modelsDir, model.DirName, model.Filename)
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return false
	}

	// Check mmproj
	mmprojPath := filepath.Join(modelsDir, model.DirName, model.MmprojName)
	mmprojInfo, err := os.Stat(mmprojPath)
	if err != nil {
		return true // mmproj file doesn't exist
	}
	if model.MmprojSize > 0 {
		tolerance := model.MmprojSize / 100
		diff := mmprojInfo.Size() - model.MmprojSize
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return true // mmproj wrong size
		}
	}
	// Check for .partial
	if _, err := os.Stat(mmprojPath + ".partial"); err == nil {
		return true
	}
	return false
}
