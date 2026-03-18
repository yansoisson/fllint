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
// If the file does not exist, it returns the embedded DefaultSystemPrompt.
func ReadFromFile(dataDir string) (string, error) {
	path := FilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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
// If the file does not exist, it returns the embedded DefaultSummaryPrompt.
func ReadSummaryPrompt(dataDir string) (string, error) {
	path := SummaryFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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

// OCRPromptFilename is the name of the OCR prompt file in the data directory.
const OCRPromptFilename = "ocr-prompt.md"

// DefaultOCRPrompt is the built-in OCR prompt.
const DefaultOCRPrompt = `Extract all text from the provided image accurately. Preserve the original formatting including paragraphs, lists, headings, and tables. Output the extracted text in Markdown format. Do not add any commentary or explanation.`

// OCRFilePath returns the full path to the OCR prompt file.
func OCRFilePath(dataDir string) string {
	return filepath.Join(dataDir, OCRPromptFilename)
}

// ReadOCRPrompt reads the OCR prompt from the data directory.
// If the file does not exist, it returns the embedded DefaultOCRPrompt.
func ReadOCRPrompt(dataDir string) (string, error) {
	path := OCRFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultOCRPrompt, nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteOCRPrompt writes the OCR prompt to the data directory.
func WriteOCRPrompt(dataDir string, content string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(OCRFilePath(dataDir), []byte(content), 0644)
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
