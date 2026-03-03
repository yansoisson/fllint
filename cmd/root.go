package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/launcher"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/server"
)

// Run is the main entry point for the application.
// It must be called from the main goroutine because systray requires the main OS thread on macOS.
func Run(frontendFS fs.FS) {
	// Determine data directory
	dataDir := os.Getenv("FLLINT_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Load config
	cfg, err := config.Load(dataDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override from env
	if modelsDir := os.Getenv("FLLINT_MODELS_DIR"); modelsDir != "" {
		cfg.ModelsDir = modelsDir
	}
	if port := os.Getenv("FLLINT_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Port)
	}

	// Ensure directories exist
	os.MkdirAll(cfg.DataDir, 0755)
	os.MkdirAll(cfg.ModelsDir, 0755)

	// Discover llama-server binary
	binDir := os.Getenv("FLLINT_BIN_DIR")
	if binDir == "" {
		binDir = "./bin"
	}
	os.MkdirAll(binDir, 0755)
	serverBinaryPath := filepath.Join(binDir, "llama-server")

	// Initialize LLM manager with real model discovery
	llmManager := llm.NewManager(serverBinaryPath, cfg.ModelsDir, cfg.DataDir)

	// Create HTTP server
	srv, err := server.New(cfg, frontendFS, llmManager)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	httpServer := &http.Server{
		Addr:    srv.Addr(),
		Handler: srv,
	}

	// Start HTTP server in background
	go func() {
		log.Printf("Fllint server starting on http://localhost%s", srv.Addr())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Open browser after a brief delay for server to start
	url := fmt.Sprintf("http://localhost%s", srv.Addr())
	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := launcher.OpenBrowser(url); err != nil {
			log.Printf("Could not open browser: %v (open %s manually)", err, url)
		}
	}()

	// Shared shutdown logic — safe to call from multiple goroutines
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Println("Shutting down...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdownCtx)
			llmManager.Stop()
		})
	}

	// Handle graceful shutdown in background
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		shutdown()
		launcher.QuitTray()
	}()

	// Run system tray on the main goroutine (required by macOS AppKit).
	// This blocks until the tray exits (via Quit menu item or QuitTray() call).
	launcher.RunTray(
		func() { launcher.OpenBrowser(url) },
		func() { shutdown() },
	)

	log.Println("Fllint stopped.")
}
