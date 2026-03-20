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

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/chat"
	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/document"
	"github.com/fllint/fllint/internal/download"
	"github.com/fllint/fllint/internal/image"
	"github.com/fllint/fllint/internal/ocr"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/provider"
	"github.com/fllint/fllint/internal/queue"
	"github.com/fllint/fllint/internal/summary"
	"github.com/fllint/fllint/internal/updater"
	"github.com/fllint/fllint/internal/version"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	router        chi.Router
	cfg           *config.Config
	llmManager    *llm.Manager
	queue         *queue.Queue
	downloadMgr   *download.Manager
	providerStore *provider.Store

	ocrService *ocr.Service

	isProduction bool
	translocated bool // macOS App Translocation — updates unavailable until restart
}

// New creates a new Server with all routes configured.
func New(cfg *config.Config, frontendFS fs.FS, llmManager *llm.Manager, downloadMgr *download.Manager, providerStore *provider.Store, translocated bool) (*Server, error) {
	chatStore, err := chat.NewStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init chat store: %w", err)
	}

	imgHandler, err := image.NewHandler(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init image handler: %w", err)
	}

	docHandler, err := document.NewHandler(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init document handler: %w", err)
	}

	inferenceQueue := queue.NewQueue(llmManager)
	summaryService := summary.NewService(chatStore, llmManager, inferenceQueue)
	ocrService := ocr.NewService(llmManager, inferenceQueue, cfg.DataDir)

	s := &Server{
		router:        chi.NewRouter(),
		cfg:           cfg,
		llmManager:    llmManager,
		queue:         inferenceQueue,
		downloadMgr:   downloadMgr,
		providerStore: providerStore,
		ocrService:    ocrService,
		isProduction:  frontendFS != nil,
		translocated:  translocated,
	}

	for _, mw := range CommonMiddleware() {
		s.router.Use(mw)
	}

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		chatHandler := chat.NewHandler(chatStore, llmManager, inferenceQueue, summaryService)
		r.Mount("/", chatHandler.Routes())

		r.Get("/models", s.listModels)
		r.Put("/models/active", s.setActiveModel)
		r.Post("/models/load", s.loadModel)
		r.Post("/models/unload", s.unloadModel)
		r.Post("/models/refresh", s.refreshModels)
		r.Post("/models/delete", s.deleteModel)
		r.Post("/models/rename", s.renameModel)
		r.Post("/models/add", s.addModel)

		r.Get("/status", s.getStatus)

		r.Post("/image/upload", imgHandler.Upload)
		r.Post("/document/upload", docHandler.Upload)
		r.Post("/document/pages", docHandler.PageCount)
		r.Handle("/uploads/*", imgHandler.Serve())

		// OCR
		r.Post("/ocr/start", s.startOCR)
		r.Get("/ocr/status/{id}", s.ocrStatus)
		r.Post("/ocr/cancel", s.cancelOCR)

		r.Get("/config", s.getConfig)
		r.Put("/config", s.updateConfig)
		r.Get("/config/system-prompt-default", s.getDefaultSystemPrompt)

		r.Get("/system-prompt", s.getSystemPrompt)
		r.Put("/system-prompt", s.updateSystemPrompt)

		r.Get("/memory", s.getMemory)

		r.Get("/downloads/registry", s.downloadRegistry)
		r.Post("/downloads/start", s.startDownload)
		r.Get("/downloads/active", s.activeDownloads)
		r.Post("/downloads/cancel", s.cancelDownload)

		r.Post("/open-folder", s.openFolder)

		// Providers
		r.Get("/providers", s.listProviders)
		r.Post("/providers", s.createProvider)
		r.Get("/providers/types", s.listProviderTypes)
		r.Put("/providers/{id}", s.updateProvider)
		r.Delete("/providers/{id}", s.deleteProvider)
		r.Post("/providers/{id}/test", s.testProvider)
		r.Post("/providers/{id}/fetch-models", s.fetchProviderModels)
		r.Post("/providers/{id}/models", s.saveProviderModels)

		// Helper models
		r.Get("/helper-models", s.listHelperModels)
		r.Put("/helper-models/config", s.updateHelperConfig)

		r.Get("/summary-prompt", s.getSummaryPrompt)
		r.Put("/summary-prompt", s.updateSummaryPrompt)
		r.Get("/summary-prompt/default", s.getDefaultSummaryPrompt)

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

func (s *Server) getSystemPrompt(w http.ResponseWriter, r *http.Request) {
	content, err := prompt.ReadFromFile(s.cfg.DataDir)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "prompt_error", "Failed to read system prompt.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"prompt":     content,
		"is_default": content == prompt.DefaultSystemPrompt,
	})
}

func (s *Server) updateSystemPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := prompt.WriteToFile(s.cfg.DataDir, req.Prompt); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "prompt_error", "Failed to save system prompt.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"prompt":     req.Prompt,
		"is_default": req.Prompt == prompt.DefaultSystemPrompt,
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

func (s *Server) addModel(w http.ResponseWriter, r *http.Request) {
	// 32 MB memory limit — larger files are written to temp files automatically
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid multipart form.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Required: main GGUF file
	ggufFile, ggufHeader, err := r.FormFile("gguf")
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "A .gguf model file is required.")
		return
	}
	defer ggufFile.Close()

	if !strings.HasSuffix(strings.ToLower(ggufHeader.Filename), ".gguf") {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Model file must be a .gguf file.")
		return
	}

	// Optional: mmproj GGUF file for vision
	mmprojFile, mmprojHeader, _ := r.FormFile("mmproj")
	if mmprojFile != nil {
		defer mmprojFile.Close()
		if !strings.HasSuffix(strings.ToLower(mmprojHeader.Filename), ".gguf") {
			respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Vision projection file must be a .gguf file.")
			return
		}
	}

	// Model name: from form field or auto-detect from filename
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = llm.ModelNameFromFilename(ggufHeader.Filename)
	}

	// Create a subdirectory for the model using sanitized name
	dirName := sanitizeDirName(name)
	if dirName == "" {
		dirName = "Custom"
	}
	destDir := filepath.Join(s.cfg.ModelsDir, dirName)

	// Ensure unique directory name
	if _, err := os.Stat(destDir); err == nil {
		base := dirName
		for i := 2; ; i++ {
			dirName = fmt.Sprintf("%s-%d", base, i)
			destDir = filepath.Join(s.cfg.ModelsDir, dirName)
			if _, err := os.Stat(destDir); os.IsNotExist(err) {
				break
			}
		}
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "model_error", "Could not create model directory.")
		return
	}

	// Copy GGUF file
	ggufDest := filepath.Join(destDir, ggufHeader.Filename)
	if err := copyUploadedFile(ggufFile, ggufDest); err != nil {
		os.RemoveAll(destDir)
		respondErrorJSON(w, http.StatusInternalServerError, "model_error", "Failed to save model file.")
		return
	}

	// Copy mmproj file if provided
	if mmprojFile != nil {
		mmprojDest := filepath.Join(destDir, mmprojHeader.Filename)
		if err := copyUploadedFile(mmprojFile, mmprojDest); err != nil {
			os.RemoveAll(destDir)
			respondErrorJSON(w, http.StatusInternalServerError, "model_error", "Failed to save vision projection file.")
			return
		}
	}

	// Write model.json with display name
	meta := struct {
		Name string `json:"name"`
	}{Name: name}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		os.WriteFile(filepath.Join(destDir, "model.json"), append(data, '\n'), 0644)
	}

	// Refresh model list
	s.llmManager.RefreshModels()

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"name":   name,
	})
}

// sanitizeDirName creates a filesystem-safe directory name from a model name.
func sanitizeDirName(name string) string {
	// Replace unsafe characters
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, name)
	safe = strings.TrimSpace(safe)
	// Collapse repeated dashes
	for strings.Contains(safe, "--") {
		safe = strings.ReplaceAll(safe, "--", "-")
	}
	return strings.Trim(safe, "-.")
}

// copyUploadedFile copies a multipart file to a destination path.
func copyUploadedFile(src io.Reader, destPath string) error {
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, src); err != nil {
		return err
	}
	// Flush to disk — required for external volumes where metadata
	// updates are deferred until sync.
	return dst.Sync()
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

// --- Provider handlers ---

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.providerStore.List()
	result := make([]provider.ProviderResponse, len(providers))
	for i := range providers {
		result[i] = providers[i].Redacted()
	}
	respondJSON(w, http.StatusOK, result)
}

func (s *Server) listProviderTypes(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, provider.RegisteredTypes())
}

func (s *Server) createProvider(w http.ResponseWriter, r *http.Request) {
	var p provider.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	created, err := s.providerStore.Create(p)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "provider_error", err.Error())
		return
	}
	s.llmManager.RefreshExternalModels()
	respondJSON(w, http.StatusCreated, created.Redacted())
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p provider.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	p.ID = id
	updated, err := s.providerStore.Update(p)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "provider_error", err.Error())
		return
	}
	s.llmManager.RefreshExternalModels()
	respondJSON(w, http.StatusOK, updated.Redacted())
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.providerStore.Delete(id); err != nil {
		respondErrorJSON(w, http.StatusNotFound, "provider_error", err.Error())
		return
	}
	s.llmManager.RefreshExternalModels()
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) testProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.providerStore.Get(id)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "provider_error", err.Error())
		return
	}

	client := provider.NewOllamaClient(p.BaseURL, p.APIKey)
	if err := client.TestConnection(r.Context()); err != nil {
		respondErrorJSON(w, http.StatusBadGateway, "connection_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) fetchProviderModels(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.providerStore.Get(id)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "provider_error", err.Error())
		return
	}

	client := provider.NewOllamaClient(p.BaseURL, p.APIKey)
	models, err := client.ListModels(r.Context())
	if err != nil {
		respondErrorJSON(w, http.StatusBadGateway, "connection_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, models)
}

func (s *Server) saveProviderModels(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Models []provider.SelectedModel `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := s.providerStore.SetModels(id, req.Models); err != nil {
		respondErrorJSON(w, http.StatusNotFound, "provider_error", err.Error())
		return
	}
	s.llmManager.RefreshExternalModels()
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Helper model handlers ---

type helperSlotResponse struct {
	Slot              string              `json:"slot"`
	AvailableModels   []helperModelOption `json:"available_models"`
	ConfiguredModelID string              `json:"configured_model_id"`
	Enabled           bool                `json:"enabled"`
}

type helperModelOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size,omitempty"`
	External bool   `json:"external"`
}

func (s *Server) listHelperModels(w http.ResponseWriter, r *http.Request) {
	helperModels := s.llmManager.ListHelperModels()
	cfg := config.Get()

	var slots []helperSlotResponse
	for _, slot := range llm.HelperSlots {
		models := helperModels[slot]
		var options []helperModelOption
		for _, m := range models {
			options = append(options, helperModelOption{
				ID:       m.ID,
				Name:     m.Name,
				Size:     m.Size,
				External: m.External,
			})
		}
		if options == nil {
			options = []helperModelOption{}
		}

		configuredID := ""
		enabled := false
		switch slot {
		case "Summary":
			configuredID = cfg.SummaryModelID
			if configuredID == "" {
				configuredID = s.llmManager.AutoDetectHelperModel("Summary")
			}
			enabled = true
		case "OCR":
			// No auto-detect for OCR — respect the user's "None (OCR disabled)" choice.
			// If the user has not configured a model, OCR stays disabled.
			configuredID = cfg.OCRModelID
			enabled = true
		}

		slots = append(slots, helperSlotResponse{
			Slot:              slot,
			AvailableModels:   options,
			ConfiguredModelID: configuredID,
			Enabled:           enabled,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (s *Server) updateHelperConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SummaryModelID *string `json:"summary_model_id"`
		OCRModelID     *string `json:"ocr_model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	cfg := config.Get()
	updated := *cfg

	if req.SummaryModelID != nil {
		updated.SummaryModelID = *req.SummaryModelID
	}
	if req.OCRModelID != nil {
		updated.OCRModelID = *req.OCRModelID
	}

	if err := config.Save(&updated); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "config_error", "Failed to save configuration.")
		return
	}
	s.cfg = config.Get()

	// If a local helper model was selected, auto-load it in the background
	if req.SummaryModelID != nil && *req.SummaryModelID != "" && !strings.HasPrefix(*req.SummaryModelID, "ext:") {
		go func() {
			if err := s.llmManager.LoadModel(*req.SummaryModelID); err != nil {
				log.Printf("Failed to load summary model %q: %v", *req.SummaryModelID, err)
			}
		}()
	}
	if req.OCRModelID != nil && *req.OCRModelID != "" && !strings.HasPrefix(*req.OCRModelID, "ext:") {
		go func() {
			if err := s.llmManager.LoadModel(*req.OCRModelID); err != nil {
				log.Printf("Failed to load OCR model %q: %v", *req.OCRModelID, err)
			}
		}()
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Summary prompt handlers ---

func (s *Server) getSummaryPrompt(w http.ResponseWriter, r *http.Request) {
	content, err := prompt.ReadSummaryPrompt(s.cfg.DataDir)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "prompt_error", "Failed to read summary prompt.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"prompt":     content,
		"is_default": content == prompt.DefaultSummaryPrompt,
	})
}

func (s *Server) updateSummaryPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := prompt.WriteSummaryPrompt(s.cfg.DataDir, req.Prompt); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "prompt_error", "Failed to save summary prompt.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"prompt":     req.Prompt,
		"is_default": req.Prompt == prompt.DefaultSummaryPrompt,
	})
}

func (s *Server) getDefaultSummaryPrompt(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"prompt": prompt.DefaultSummaryPrompt,
	})
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
	if s.translocated {
		respondErrorJSON(w, http.StatusServiceUnavailable, "update_unavailable",
			"Auto-update is unavailable on first launch. Please close and reopen the app, then try again.")
		return
	}
	if !updater.HelperExists() {
		respondErrorJSON(w, http.StatusServiceUnavailable, "update_unavailable",
			"Auto-update is not available. Check the GitHub releases page for new versions.")
		return
	}
	if err := updater.CheckForUpdate(); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
