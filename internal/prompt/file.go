package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

// SystemPromptFilename is the name of the system prompt file in the data directory.
const SystemPromptFilename = "system-prompt.md"

// SummaryPromptFilename is the name of the summary prompt file in the data directory.
const SummaryPromptFilename = "summary-prompt.md"

// FilePath returns the full path to the system prompt file.
func FilePath(dataDir string) string {
	return filepath.Join(dataDir, SystemPromptFilename)
}

// ReadFromFile reads the system prompt from the data directory.
// If the file does not exist, it creates it with the DefaultSystemPrompt.
func ReadFromFile(dataDir string) (string, error) {
	path := FilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Auto-create with default
			if wErr := WriteToFile(dataDir, DefaultSystemPrompt); wErr != nil {
				return DefaultSystemPrompt, wErr
			}
			return DefaultSystemPrompt, nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteToFile writes the system prompt to the data directory.
func WriteToFile(dataDir string, content string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(FilePath(dataDir), []byte(content), 0644)
}

// SummaryFilePath returns the full path to the summary prompt file.
func SummaryFilePath(dataDir string) string {
	return filepath.Join(dataDir, SummaryPromptFilename)
}

// ReadSummaryPrompt reads the summary prompt from the data directory.
// If the file does not exist, it creates it with the DefaultSummaryPrompt.
func ReadSummaryPrompt(dataDir string) (string, error) {
	path := SummaryFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if wErr := WriteSummaryPrompt(dataDir, DefaultSummaryPrompt); wErr != nil {
				return DefaultSummaryPrompt, wErr
			}
			return DefaultSummaryPrompt, nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteSummaryPrompt writes the summary prompt to the data directory.
func WriteSummaryPrompt(dataDir string, content string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(SummaryFilePath(dataDir), []byte(content), 0644)
}

// MigrateFromConfig writes a legacy system prompt from config.json to the file,
// but only if the file does not already exist.
func MigrateFromConfig(dataDir string, legacyPrompt string) error {
	if legacyPrompt == "" {
		return nil
	}
	path := FilePath(dataDir)
	if _, err := os.Stat(path); err == nil {
		// File already exists — don't overwrite
		return nil
	}
	return WriteToFile(dataDir, legacyPrompt)
}
