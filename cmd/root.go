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
	"github.com/fllint/fllint/internal/server"
	"github.com/fllint/fllint/internal/updater"
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
	serverBinaryPath := filepath.Join(appPaths.BinDir, "llama-server")

	// Initialize LLM manager with real model discovery
	llmManager := llm.NewManager(serverBinaryPath, cfg.ModelsDir, cfg.DataDir)

	// Auto-load the smallest available model in the background
	go func() {
		if !llmManager.HasBinary() {
			return
		}
		models := llmManager.ListModels()
		if len(models) == 0 {
			return
		}
		// models[0] is the smallest (sorted by size in scanModels)
		target := models[0]
		log.Printf("Auto-loading %q...", target.Name)
		if err := llmManager.SetActive(target.ID); err != nil {
			log.Printf("Auto-load failed: %v", err)
		}
	}()

	// Initialize download manager for in-app model downloads
	downloadMgr := download.NewManager(cfg.ModelsDir, llmManager)

	// Create HTTP server
	srv, err := server.New(cfg, frontendFS, llmManager, downloadMgr)
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

		// Auto-check for updates after a short delay (don't slow down startup)
		if updater.HelperExists() {
			go func() {
				time.Sleep(10 * time.Second)
				log.Println("Sparkle: auto-checking for updates")
				if err := updater.CheckForUpdate(); err != nil {
					log.Printf("Sparkle: %v", err)
				}
			}()
		} else {
			log.Println("Sparkle: helper not found, auto-update disabled")
		}
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
