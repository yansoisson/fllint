package document

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const maxExtractedChars = 100_000

// allowedExtensions maps file extensions to their extraction category.
var allowedExtensions = map[string]string{
	// Plain text / code
	".txt": "text", ".md": "text", ".csv": "text", ".json": "text",
	".xml": "text", ".yaml": "text", ".yml": "text", ".toml": "text", ".log": "text",
	".go": "text", ".js": "text", ".ts": "text", ".jsx": "text", ".tsx": "text",
	".py": "text", ".rs": "text", ".c": "text", ".cpp": "text", ".h": "text", ".hpp": "text",
	".java": "text", ".rb": "text", ".php": "text", ".swift": "text", ".kt": "text",
	".sh": "text", ".bash": "text", ".zsh": "text",
	".css": "text", ".scss": "text", ".html": "text", ".sql": "text",
	".r": "text", ".lua": "text", ".pl": "text", ".ex": "text", ".exs": "text",
	".hs": "text", ".ml": "text", ".scala": "text", ".dart": "text",
	".conf": "text", ".cfg": "text", ".ini": "text",
	".svelte": "text", ".vue": "text",
	// PDF
	".pdf": "pdf",
	// Word
	".docx": "docx",
}

// IsSupported returns true if the file extension is in the allowlist.
func IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := allowedExtensions[ext]
	return ok
}

// ExtractText reads the file and returns its text content.
func ExtractText(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	category, ok := allowedExtensions[ext]
	if !ok {
		return "", fmt.Errorf("Unsupported file type. Supported: .txt, .md, .pdf, .docx, and common code files.")
	}

	var text string
	var err error

	switch category {
	case "text":
		text, err = extractPlainText(filePath)
	case "pdf":
		text, err = extractPDF(filePath)
	case "docx":
		text, err = extractDOCX(filePath)
	default:
		return "", fmt.Errorf("Unsupported file type.")
	}

	if err != nil {
		return "", err
	}

	text = strings.TrimSpace(text)
	if text == "" && category != "pdf" {
		return "", fmt.Errorf("No text could be extracted from this file.")
	}

	// Truncate if too long
	if len([]rune(text)) > maxExtractedChars {
		runes := []rune(text)
		text = string(runes[:maxExtractedChars]) + "\n\n[Document truncated — exceeded maximum length]"
	}

	return text, nil
}

func extractPlainText(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Could not read file: %w", err)
	}

	// Reject binary files: check if content is mostly valid UTF-8
	if !utf8.Valid(data) {
		// Allow some invalid bytes (common in text files with mixed encodings)
		// but reject clearly binary content
		validCount := 0
		totalCount := len(data)
		for i := 0; i < totalCount; {
			r, size := utf8.DecodeRune(data[i:])
			if r != utf8.RuneError || size > 1 {
				validCount += size
			}
			i += size
		}
		if totalCount > 0 && float64(validCount)/float64(totalCount) < 0.9 {
			return "", fmt.Errorf("This file appears to be binary, not text.")
		}
	}

	return string(data), nil
}

// ExtractPDFPage extracts text from a single PDF page (1-based index).
func ExtractPDFPage(filePath string, pageNum int) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Could not open PDF file.")
	}
	defer f.Close()

	if pageNum < 1 || pageNum > r.NumPage() {
		return "", fmt.Errorf("Page %d out of range (1-%d).", pageNum, r.NumPage())
	}

	page := r.Page(pageNum)
	if page.V.IsNull() {
		return "", nil
	}
	text, err := page.GetPlainText(nil)
	if err != nil {
		return "", nil
	}
	return text, nil
}

func extractPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Could not extract text from this PDF. It may be image-based or encrypted.")
	}
	defer f.Close()

	var sb strings.Builder
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		if i < numPages {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// DOCX parsing — extract text from word/document.xml inside the ZIP archive.

type docxBody struct {
	Paragraphs []docxParagraph `xml:"body>p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text []docxText `xml:"t"`
}

type docxText struct {
	Value string `xml:",chardata"`
}

func extractDOCX(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("Could not read this document. The file may be corrupt.")
	}
	defer r.Close()

	// Find word/document.xml
	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("Could not read this document. The file may be corrupt.")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("Could not read this document. The file may be corrupt.")
	}
	defer rc.Close()

	var body docxBody
	if err := xml.NewDecoder(rc).Decode(&body); err != nil {
		return "", fmt.Errorf("Could not read this document. The file may be corrupt.")
	}

	var sb strings.Builder
	for i, para := range body.Paragraphs {
		var line strings.Builder
		for _, run := range para.Runs {
			for _, t := range run.Text {
				line.WriteString(t.Value)
			}
		}
		if line.Len() > 0 {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(line.String())
		}
	}

	return sb.String(), nil
}
