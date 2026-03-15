package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/download"
	"github.com/fllint/fllint/internal/launcher"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/paths"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/server"
)

// Run is the main entry point for the application.
// It must be called from the main goroutine because systray requires the main OS thread on macOS.
func Run(frontendFS fs.FS) {
	// Resolve all filesystem paths (bundle detection, env overrides, or CWD defaults)
	appPaths := paths.Resolve()

	// Load config from resolved data directory
	cfg, err := config.Load(appPaths.DataDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg.ModelsDir = appPaths.ModelsDir

	// Port override from env
	if port := os.Getenv("FLLINT_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Port)
	}

	// Ensure directories exist
	os.MkdirAll(cfg.DataDir, 0755)
	os.MkdirAll(cfg.ModelsDir, 0755)
	os.MkdirAll(appPaths.BinDir, 0755)

	// Migrate legacy system_prompt from config.json to system-prompt.md
	if legacy := config.LegacySystemPrompt(); legacy != "" {
		if err := prompt.MigrateFromConfig(cfg.DataDir, legacy); err != nil {
			log.Printf("Warning: failed to migrate system prompt: %v", err)
		}
		// Re-save config to strip the legacy system_prompt field
		if err := config.Save(cfg); err != nil {
			log.Printf("Warning: failed to clean config after prompt migration: %v", err)
		}
	}
	// Ensure system-prompt.md exists (creates with default if missing)
	if _, err := prompt.ReadFromFile(cfg.DataDir); err != nil {
		log.Printf("Warning: failed to initialize system prompt file: %v", err)
	}
	serverBinaryPath := filepath.Join(appPaths.BinDir, "llama-server")

	// Initialize LLM manager with real model discovery
	llmManager := llm.NewManager(serverBinaryPath, cfg.ModelsDir, cfg.DataDir)

	// Initialize download manager for in-app model downloads
	downloadMgr := download.NewManager(cfg.ModelsDir, llmManager)

	// Auto-setup: download Lite model if needed, then auto-load
	go func() {
		if !llmManager.HasBinary() {
			return
		}
		models := llmManager.ListModels()

		// If no models exist, auto-download the Lite model
		if len(models) == 0 {
			log.Println("No models found — auto-downloading Lite model...")
			info, err := downloadMgr.Start("lite-qwen3.5-2b")
			if err != nil {
				log.Printf("Auto-download failed: %v", err)
				return
			}
			// Wait for download to complete
			for {
				time.Sleep(2 * time.Second)
				statuses := downloadMgr.Status()
				var found *download.DownloadInfo
				for _, s := range statuses {
					if s.ID == info.ID {
						found = s
						break
					}
				}
				if found == nil || found.State == download.StateComplete {
					break
				}
				if found.State == download.StateError || found.State == download.StateCancelled {
					log.Printf("Auto-download stopped: %s", found.Error)
					return
				}
			}
			models = llmManager.ListModels()
			if len(models) == 0 {
				return
			}
		}

		// Auto-load the smallest available model
		target := models[0]
		log.Printf("Auto-loading %q...", target.Name)
		if err := llmManager.SetActive(target.ID); err != nil {
			log.Printf("Auto-load failed: %v", err)
		}
	}()

	// Create HTTP server
	srv, err := server.New(cfg, frontendFS, llmManager, downloadMgr, appPaths.Translocated)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Bind the port synchronously — if another Fllint is running, exit immediately
	// (before creating a systray that would become a ghost entry)
	listener, err := net.Listen("tcp", srv.Addr())
	if err != nil {
		log.Fatalf("Port %s is already in use — is another Fllint instance running?", srv.Addr())
	}

	httpServer := &http.Server{
		Handler: srv,
	}

	// Start HTTP server on the already-bound listener
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Fllint server starting on http://localhost%s", srv.Addr())
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for server to be reachable before proceeding (no blind delay)
	url := fmt.Sprintf("http://localhost%s", srv.Addr())
	if err := waitForServer(srv.Addr(), 5*time.Second); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
	log.Printf("Fllint ready at %s", url)

	// Server is confirmed up — open browser (skip in dev mode, Vite serves the frontend)
	if frontendFS != nil {
		go func() {
			if err := launcher.OpenBrowser(url); err != nil {
				log.Printf("Could not open browser: %v (open %s manually)", err, url)
			}
		}()

		// Auto-update is disabled until the repo is public and GitHub Pages
		// can serve the appcast. The Sparkle framework and helper remain in
		// the bundle so it can be re-enabled later without a new build.
		log.Println("Sparkle: auto-update disabled (appcast not yet available)")
	}

	// Shared shutdown logic — safe to call from multiple goroutines
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Println("Shutting down...")
			srv.StopQueue()
			downloadMgr.StopAll()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdownCtx)
			llmManager.Stop()
		})
	}

	// Monitor HTTP server — if it dies unexpectedly, shut everything down
	go func() {
		if err, ok := <-serverErr; ok {
			log.Printf("HTTP server died: %v", err)
			shutdown()
			launcher.QuitTray()
		}
	}()

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

// waitForServer blocks until the given address is accepting TCP connections,
// or returns an error if the timeout is exceeded.
func waitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not start within %s", timeout)
}
