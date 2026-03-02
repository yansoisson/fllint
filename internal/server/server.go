package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/chat"
	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/image"
	"github.com/fllint/fllint/internal/llm"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	router     chi.Router
	cfg        *config.Config
	llmManager *llm.Manager
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

	s := &Server{
		router:     chi.NewRouter(),
		cfg:        cfg,
		llmManager: llmManager,
	}

	for _, mw := range CommonMiddleware() {
		s.router.Use(mw)
	}

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		chatHandler := chat.NewHandler(chatStore, llmManager)
		r.Mount("/", chatHandler.Routes())

		r.Get("/models", s.listModels)
		r.Put("/models/active", s.setActiveModel)

		r.Post("/image/upload", imgHandler.Upload)
		r.Handle("/uploads/*", imgHandler.Serve())

		r.Get("/config", s.getConfig)
		r.Put("/config", s.updateConfig)
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
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.llmManager.SetActive(req.ModelID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Config handlers ---

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.cfg)
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var c config.Config
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := config.Save(&c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, c)
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
