package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/chat"
	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/image"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/queue"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	router     chi.Router
	cfg        *config.Config
	llmManager *llm.Manager
	queue      *queue.Queue
}

// New creates a new Server with all routes configured.
func New(cfg *config.Config, frontendFS fs.FS, llmManager *llm.Manager) (*Server, error) {
	chatStore, err := chat.NewStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init chat store: %w", err)
	}

	imgHandler, err := image.NewHandler(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init image handler: %w", err)
	}

	inferenceQueue := queue.NewQueue(llmManager)

	s := &Server{
		router:     chi.NewRouter(),
		cfg:        cfg,
		llmManager: llmManager,
		queue:      inferenceQueue,
	}

	for _, mw := range CommonMiddleware() {
		s.router.Use(mw)
	}

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		chatHandler := chat.NewHandler(chatStore, llmManager, inferenceQueue)
		r.Mount("/", chatHandler.Routes())

		r.Get("/models", s.listModels)
		r.Put("/models/active", s.setActiveModel)
		r.Post("/models/load", s.loadModel)
		r.Post("/models/unload", s.unloadModel)
		r.Post("/models/refresh", s.refreshModels)
		r.Post("/models/delete", s.deleteModel)
		r.Post("/models/rename", s.renameModel)

		r.Get("/status", s.getStatus)

		r.Post("/image/upload", imgHandler.Upload)
		r.Handle("/uploads/*", imgHandler.Serve())

		r.Get("/config", s.getConfig)
		r.Put("/config", s.updateConfig)
		r.Get("/config/system-prompt-default", s.getDefaultSystemPrompt)

		r.Get("/memory", s.getMemory)

		r.Post("/open-folder", s.openFolder)
	})

	// SPA fallback (after API routes)
	if frontendFS != nil {
		s.serveSPA(frontendFS)
	}

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) Addr() string {
	return fmt.Sprintf(":%d", s.cfg.Port)
}

// StopQueue stops the inference queue worker and cancels all pending items.
func (s *Server) StopQueue() {
	if s.queue != nil {
		s.queue.Stop()
	}
}

// --- Model handlers ---

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	models := s.llmManager.ListModels()
	respondJSON(w, http.StatusOK, models)
}

func (s *Server) setActiveModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := s.llmManager.SetActive(req.ModelID); err != nil {
		var memErr *llm.MemoryError
		if errors.As(err, &memErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":           memErr.Error(),
				"code":            "insufficient_memory",
				"required_bytes":  memErr.RequiredBytes,
				"available_bytes": memErr.AvailableBytes,
				"model_name":      memErr.ModelName,
			})
			return
		}
		respondErrorJSON(w, http.StatusBadRequest, "engine_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) loadModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.ModelID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "model_id is required.")
		return
	}
	if err := s.llmManager.LoadModel(req.ModelID); err != nil {
		var memErr *llm.MemoryError
		if errors.As(err, &memErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":           memErr.Error(),
				"code":            "insufficient_memory",
				"required_bytes":  memErr.RequiredBytes,
				"available_bytes": memErr.AvailableBytes,
				"model_name":      memErr.ModelName,
			})
			return
		}
		respondErrorJSON(w, http.StatusBadRequest, "engine_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) unloadModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.ModelID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "model_id is required.")
		return
	}
	if err := s.llmManager.UnloadModel(req.ModelID); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "engine_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) refreshModels(w http.ResponseWriter, r *http.Request) {
	s.llmManager.RefreshModels()
	models := s.llmManager.ListModels()
	respondJSON(w, http.StatusOK, models)
}

// --- Status handler ---

func (s *Server) getStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.llmManager.Status())
}

// --- Config handlers ---

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, config.Get())
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var c config.Config
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	// Preserve DataDir from current config — the frontend should not override it
	current := config.Get()
	c.DataDir = current.DataDir
	c.WithDefaults()

	if err := config.Save(&c); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "config_error", "Failed to save configuration.")
		return
	}
	s.cfg = config.Get() // keep s.cfg in sync

	// Live-update inference params on ALL loaded engines (no restart needed)
	newParams := llm.InferenceParams{
		Temperature:   c.Temperature,
		TopP:          c.TopP,
		TopK:          c.TopK,
		RepeatPenalty: c.RepeatPenalty,
		MaxTokens:     c.MaxTokens,
		Seed:          c.Seed,
	}
	for _, modelID := range s.llmManager.LoadedModelIDs() {
		if engine := s.llmManager.GetEngine(modelID); engine != nil {
			if e, ok := engine.(*llm.LlamaCppEngine); ok {
				e.SetInferenceParams(newParams)
			}
		}
	}

	respondJSON(w, http.StatusOK, c)
}

func (s *Server) getDefaultSystemPrompt(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"prompt": prompt.DefaultSystemPrompt,
	})
}

func (s *Server) deleteModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := s.llmManager.DeleteModel(req.ModelID); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "model_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) renameModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := s.llmManager.RenameModel(req.ModelID, req.Name); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "model_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getMemory(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.llmManager.MemoryInfo())
}

func (s *Server) openFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Folder string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	var dir string
	switch req.Folder {
	case "models":
		dir = s.cfg.ModelsDir
	case "data":
		dir = s.cfg.DataDir
	default:
		respondErrorJSON(w, http.StatusBadRequest, "bad_request",
			"Invalid folder. Use 'models' or 'data'.")
		return
	}

	if err := openInFileManager(dir); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "open_error",
			"Failed to open folder.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func openInFileManager(dir string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "linux":
		return exec.Command("xdg-open", dir).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// --- SPA serving ---

func (s *Server) serveSPA(frontendFS fs.FS) {
	fileServer := http.FileServer(http.FS(frontendFS))

	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists in the embedded FS
		f, err := frontendFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Serve index.html as SPA fallback
		index, err := frontendFS.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		defer index.Close()

		stat, err := index.Stat()
		if err != nil {
			http.Error(w, "cannot read index.html", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
	}))
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondErrorJSON(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
