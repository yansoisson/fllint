package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/chat"
	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/download"
	"github.com/fllint/fllint/internal/image"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/queue"
	"github.com/fllint/fllint/internal/version"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	router      chi.Router
	cfg         *config.Config
	llmManager  *llm.Manager
	queue       *queue.Queue
	downloadMgr *download.Manager

	isProduction     bool
	sparkleHelperMu  sync.Mutex
	sparkleHelperCmd *exec.Cmd
}

// New creates a new Server with all routes configured.
func New(cfg *config.Config, frontendFS fs.FS, llmManager *llm.Manager, downloadMgr *download.Manager) (*Server, error) {
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
		router:       chi.NewRouter(),
		cfg:          cfg,
		llmManager:   llmManager,
		queue:        inferenceQueue,
		downloadMgr:  downloadMgr,
		isProduction: frontendFS != nil,
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

		r.Get("/downloads/registry", s.downloadRegistry)
		r.Post("/downloads/start", s.startDownload)
		r.Get("/downloads/active", s.activeDownloads)
		r.Post("/downloads/cancel", s.cancelDownload)

		r.Post("/open-folder", s.openFolder)

		r.Get("/version", s.getVersion)
		r.Post("/check-update", s.checkUpdate)
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

// --- Download handlers ---

func (s *Server) downloadRegistry(w http.ResponseWriter, r *http.Request) {
	models := download.Registry()
	entries := download.CheckDownloaded(s.cfg.ModelsDir, models)
	respondJSON(w, http.StatusOK, entries)
}

func (s *Server) startDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RegistryID string `json:"registry_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.RegistryID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "registry_id is required.")
		return
	}
	dl, err := s.downloadMgr.Start(req.RegistryID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "download_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, dl)
}

func (s *Server) activeDownloads(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.downloadMgr.Status())
}

func (s *Server) cancelDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DownloadID string `json:"download_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.DownloadID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "download_id is required.")
		return
	}
	if err := s.downloadMgr.Cancel(req.DownloadID); err != nil {
		respondErrorJSON(w, http.StatusNotFound, "cancel_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Version handler ---

func (s *Server) getVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"build":   version.Build,
	})
}

// --- Update handler ---

func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "darwin" {
		respondErrorJSON(w, http.StatusBadRequest, "unsupported_platform",
			"Auto-update is only available on macOS.")
		return
	}
	if !s.isProduction {
		respondErrorJSON(w, http.StatusBadRequest, "dev_mode",
			"Auto-update is not available in development mode.")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "update_error",
			"Could not determine executable path.")
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	helperPath := filepath.Join(filepath.Dir(exe), "sparkle-helper")

	if _, err := os.Stat(helperPath); err != nil {
		respondErrorJSON(w, http.StatusNotFound, "update_unavailable",
			"Update helper not found. Updates are not available in this build.")
		return
	}

	s.sparkleHelperMu.Lock()
	defer s.sparkleHelperMu.Unlock()

	// Check if helper is already running
	if s.sparkleHelperCmd != nil && s.sparkleHelperCmd.ProcessState == nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "already_running"})
		return
	}

	cmd := exec.Command(helperPath)
	if err := cmd.Start(); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "update_error",
			"Failed to launch update checker.")
		return
	}
	s.sparkleHelperCmd = cmd

	// Reap the process in background to avoid zombies
	go func() {
		cmd.Wait()
	}()

	log.Println("Sparkle: update check launched")
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// StopSparkleHelper kills the sparkle-helper process if it is running.
func (s *Server) StopSparkleHelper() {
	s.sparkleHelperMu.Lock()
	defer s.sparkleHelperMu.Unlock()
	if s.sparkleHelperCmd != nil && s.sparkleHelperCmd.Process != nil && s.sparkleHelperCmd.ProcessState == nil {
		s.sparkleHelperCmd.Process.Kill()
		s.sparkleHelperCmd = nil
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
