package document

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

// Handler manages document uploads and text extraction.
type Handler struct {
	uploadDir string
}

func NewHandler(dataDir string) (*Handler, error) {
	dir := filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Handler{uploadDir: dir}, nil
}

type uploadResponse struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	URL           string `json:"url"`
	OriginalName  string `json:"original_name"`
	ExtractedText string `json:"extracted_text"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			"This file is too large. Maximum size is 25 MB.")
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			"No document provided.")
		return
	}
	defer file.Close()

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !IsSupported(header.Filename) {
		writeError(w, http.StatusBadRequest, "unsupported_type",
			"Unsupported file type. Supported: .txt, .md, .pdf, .docx, and common code files.")
		return
	}

	// Save to disk with UUID name
	id := uuid.New().String()
	filename := id + ext
	dst := filepath.Join(h.uploadDir, filename)

	out, err := os.Create(dst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error",
			"Failed to save document.")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		os.Remove(dst)
		writeError(w, http.StatusInternalServerError, "server_error",
			"Failed to save document.")
		return
	}
	// Close before reading for extraction
	out.Close()

	// Extract text
	text, err := ExtractText(dst)
	if err != nil {
		os.Remove(dst)
		writeError(w, http.StatusUnprocessableEntity, "extraction_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(uploadResponse{
		ID:            id,
		Filename:      filename,
		URL:           fmt.Sprintf("/api/uploads/%s", filename),
		OriginalName:  header.Filename,
		ExtractedText: text,
	})
}

// PageCount returns the number of pages in a PDF file.
func (h *Handler) PageCount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	if !strings.HasPrefix(req.URL, "/api/uploads/") {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid document URL.")
		return
	}
	filename := strings.TrimPrefix(req.URL, "/api/uploads/")
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") || filename == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid document URL.")
		return
	}

	filePath := filepath.Join(h.uploadDir, filename)
	f, reader, err := pdf.Open(filePath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "pdf_error", "Could not open PDF file.")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"page_count": reader.NumPage()})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: message, Code: code})
}
