package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Handler manages image uploads.
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

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20) // 10MB limit

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "failed to read image", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	id := uuid.New().String()
	filename := id + ext
	dst := filepath.Join(h.uploadDir, filename)

	out, err := os.Create(dst)
	if err != nil {
		http.Error(w, "failed to save image", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "failed to save image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"id":       id,
		"filename": filename,
		"url":      fmt.Sprintf("/api/uploads/%s", filename),
	})
}

// Serve returns a handler that serves uploaded files.
func (h *Handler) Serve() http.Handler {
	return http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(h.uploadDir)))
}
