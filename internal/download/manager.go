package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/fllint/fllint/internal/llm"
)

// DownloadState represents the lifecycle state of a download.
type DownloadState string

const (
	StateQueued      DownloadState = "queued"
	StateDownloading DownloadState = "downloading"
	StateComplete    DownloadState = "complete"
	StateCancelled   DownloadState = "cancelled"
	StateError       DownloadState = "error"
)

// Download represents an in-progress or completed download.
type Download struct {
	ID          string        `json:"id"`
	RegistryID  string        `json:"registry_id"`
	DisplayName string        `json:"display_name"`
	URL         string        `json:"-"` // not exposed to frontend
	DestDir     string        `json:"-"`
	Filename    string        `json:"-"`
	TotalBytes  int64         `json:"total_bytes"`
	doneBytes   atomic.Int64  // updated atomically by writer goroutine
	State       DownloadState `json:"state"`
	Error       string        `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	FinishedAt  *time.Time    `json:"finished_at,omitempty"`

	// mmproj fields (downloaded sequentially after main file)
	mmprojURL  string
	mmprojName string

	cancel context.CancelFunc
}

// DownloadInfo is the JSON-serializable snapshot returned by the API.
type DownloadInfo struct {
	ID          string        `json:"id"`
	RegistryID  string        `json:"registry_id"`
	DisplayName string        `json:"display_name"`
	TotalBytes  int64         `json:"total_bytes"`
	DoneBytes   int64         `json:"done_bytes"`
	State       DownloadState `json:"state"`
	Error       string        `json:"error,omitempty"`
}

func (d *Download) snapshot() *DownloadInfo {
	return &DownloadInfo{
		ID:          d.ID,
		RegistryID:  d.RegistryID,
		DisplayName: d.DisplayName,
		TotalBytes:  d.TotalBytes,
		DoneBytes:   d.doneBytes.Load(),
		State:       d.State,
		Error:       d.Error,
	}
}

// Manager handles model downloads with a single-worker queue.
type Manager struct {
	mu         sync.RWMutex
	active     *Download
	queued     []*Download
	completed  []*Download // keep last few for display
	modelsDir  string
	llmManager *llm.Manager
	httpClient *http.Client
	notify     chan struct{}
	stopCh     chan struct{}
	stopped    bool
}

// URL allowlist — only these domains are accepted for downloads.
var allowedHosts = map[string]bool{
	"huggingface.co": true,
	"hf.co":          true,
}

const diskSpaceMargin = 500 * 1024 * 1024 // 500 MB safety margin

// NewManager creates a new download Manager and starts the worker goroutine.
func NewManager(modelsDir string, llmManager *llm.Manager) *Manager {
	m := &Manager{
		modelsDir:  modelsDir,
		llmManager: llmManager,
		httpClient: &http.Client{
			// No overall timeout — downloads can be multi-GB.
			// The Transport has a 30s dial/TLS timeout.
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
		notify: make(chan struct{}, 1),
		stopCh: make(chan struct{}),
	}
	go m.worker()
	return m
}

// Start queues a download for the given registry model.
func (m *Manager) Start(registryID string) (*DownloadInfo, error) {
	model := registryByID(registryID)
	if model == nil {
		return nil, fmt.Errorf("Unknown model: %s", registryID)
	}

	if err := validateURL(model.URL); err != nil {
		return nil, err
	}
	if model.MmprojURL != "" {
		if err := validateURL(model.MmprojURL); err != nil {
			return nil, err
		}
	}

	// Check if already downloaded
	if isDownloaded(m.modelsDir, *model) {
		return nil, fmt.Errorf("%s is already downloaded.", model.DisplayName)
	}

	// Check if already queued or downloading
	m.mu.RLock()
	if m.active != nil && m.active.RegistryID == registryID {
		m.mu.RUnlock()
		return nil, fmt.Errorf("%s is already downloading.", model.DisplayName)
	}
	for _, dl := range m.queued {
		if dl.RegistryID == registryID {
			m.mu.RUnlock()
			return nil, fmt.Errorf("%s is already queued.", model.DisplayName)
		}
	}
	m.mu.RUnlock()

	// Check disk space
	requiredBytes := model.Size + model.MmprojSize
	disk, err := CheckSpace(m.modelsDir)
	if err != nil {
		log.Printf("Download: could not check disk space: %v", err)
		// Continue anyway — the download will fail with a disk error if space runs out
	} else if disk.AvailableBytes < requiredBytes+diskSpaceMargin {
		needGB := float64(requiredBytes) / (1024 * 1024 * 1024)
		haveGB := float64(disk.AvailableBytes) / (1024 * 1024 * 1024)
		return nil, fmt.Errorf("Not enough disk space. The model needs %.1f GB but only %.1f GB is available.", needGB, haveGB)
	}

	dl := &Download{
		ID:          uuid.New().String(),
		RegistryID:  registryID,
		DisplayName: model.DisplayName,
		URL:         model.URL,
		DestDir:     filepath.Join(m.modelsDir, model.DirName),
		Filename:    model.Filename,
		TotalBytes:  model.Size + model.MmprojSize,
		State:       StateQueued,
		mmprojURL:   model.MmprojURL,
		mmprojName:  model.MmprojName,
	}

	m.mu.Lock()
	m.queued = append(m.queued, dl)
	m.mu.Unlock()

	// Signal the worker.
	select {
	case m.notify <- struct{}{}:
	default:
	}

	log.Printf("Download: queued %s (%s)", model.DisplayName, registryID)
	return dl.snapshot(), nil
}

// Cancel cancels a queued or active download.
func (m *Manager) Cancel(downloadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active != nil && m.active.ID == downloadID {
		m.active.cancel()
		log.Printf("Download: cancelling active download %s", downloadID)
		return nil
	}

	for i, dl := range m.queued {
		if dl.ID == downloadID {
			dl.State = StateCancelled
			m.queued = append(m.queued[:i], m.queued[i+1:]...)
			log.Printf("Download: cancelled queued download %s", downloadID)
			return nil
		}
	}

	return fmt.Errorf("Download not found: %s", downloadID)
}

// Status returns a snapshot of all active, queued, and recently completed downloads.
func (m *Manager) Status() []*DownloadInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DownloadInfo
	if m.active != nil {
		result = append(result, m.active.snapshot())
	}
	for _, dl := range m.queued {
		result = append(result, dl.snapshot())
	}
	for _, dl := range m.completed {
		result = append(result, dl.snapshot())
	}
	if result == nil {
		result = []*DownloadInfo{}
	}
	return result
}

// StopAll cancels all downloads and waits for the worker to exit.
func (m *Manager) StopAll() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)

	for _, dl := range m.queued {
		dl.State = StateCancelled
	}
	m.queued = nil

	if m.active != nil && m.active.cancel != nil {
		m.active.cancel()
	}
	m.mu.Unlock()

	log.Println("Download: stopped all downloads")
}

// worker processes downloads one at a time.
func (m *Manager) worker() {
	for {
		select {
		case <-m.stopCh:
			return
		case <-m.notify:
		}

		for {
			dl := m.dequeue()
			if dl == nil {
				break
			}

			select {
			case <-m.stopCh:
				return
			default:
			}

			m.processDownload(dl)
		}
	}
}

func (m *Manager) dequeue() *Download {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queued) == 0 {
		return nil
	}

	dl := m.queued[0]
	m.queued = m.queued[1:]
	m.active = dl
	return dl
}

func (m *Manager) processDownload(dl *Download) {
	ctx, cancel := context.WithCancel(context.Background())
	dl.cancel = cancel

	now := time.Now()
	dl.StartedAt = &now
	dl.State = StateDownloading

	log.Printf("Download: starting %s", dl.DisplayName)

	defer func() {
		cancel()
		m.mu.Lock()
		if m.active == dl {
			m.active = nil
		}
		// Keep in completed list (max 10)
		m.completed = append(m.completed, dl)
		if len(m.completed) > 10 {
			m.completed = m.completed[len(m.completed)-10:]
		}
		m.mu.Unlock()
	}()

	// Ensure destination directory exists
	if err := os.MkdirAll(dl.DestDir, 0755); err != nil {
		dl.State = StateError
		dl.Error = "Could not create models directory."
		log.Printf("Download: mkdir error: %v", err)
		return
	}

	// Download main GGUF file
	destPath := filepath.Join(dl.DestDir, dl.Filename)
	if err := m.downloadFile(ctx, dl, dl.URL, destPath); err != nil {
		if ctx.Err() != nil {
			dl.State = StateCancelled
			log.Printf("Download: cancelled %s", dl.DisplayName)
		} else {
			dl.State = StateError
			dl.Error = friendlyDownloadError(err)
			log.Printf("Download: error downloading %s: %v", dl.DisplayName, err)
		}
		return
	}

	// Download mmproj if needed
	if dl.mmprojURL != "" && dl.mmprojName != "" {
		mmprojPath := filepath.Join(dl.DestDir, dl.mmprojName)
		if err := m.downloadFile(ctx, dl, dl.mmprojURL, mmprojPath); err != nil {
			if ctx.Err() != nil {
				dl.State = StateCancelled
				log.Printf("Download: cancelled %s (mmproj)", dl.DisplayName)
			} else {
				dl.State = StateError
				dl.Error = friendlyDownloadError(err)
				log.Printf("Download: error downloading mmproj for %s: %v", dl.DisplayName, err)
			}
			return
		}
	}

	// Create model.json with display name
	m.writeModelMeta(dl.DestDir, dl.DisplayName)

	// Refresh models so the new model appears in the selector
	m.llmManager.RefreshModels()

	done := time.Now()
	dl.FinishedAt = &done
	dl.State = StateComplete
	log.Printf("Download: completed %s", dl.DisplayName)
}

// downloadFile downloads a single file with resume support.
func (m *Manager) downloadFile(ctx context.Context, dl *Download, fileURL string, destPath string) error {
	partialPath := destPath + ".partial"

	// Check for existing partial file (resume support)
	var existingSize int64
	if info, err := os.Stat(partialPath); err == nil {
		existingSize = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Fllint/1.0")

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		log.Printf("Download: resuming %s from byte %d", filepath.Base(destPath), existingSize)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	var file *os.File

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Resume successful — append to existing partial file
		file, err = os.OpenFile(partialPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open partial file: %w", err)
		}
		// DoneBytes already reflects previous progress
	case http.StatusOK:
		// Server doesn't support Range or sent full file — start from scratch
		if existingSize > 0 {
			log.Printf("Download: server returned 200 (not 206), restarting from scratch")
			// Reset progress for this file: subtract the existing partial size
			dl.doneBytes.Add(-existingSize)
		}
		file, err = os.Create(partialPath)
		if err != nil {
			return fmt.Errorf("create partial file: %w", err)
		}
		existingSize = 0
	case http.StatusRequestedRangeNotSatisfiable:
		// File is already complete? Try renaming directly.
		if existingSize > 0 {
			if err := os.Rename(partialPath, destPath); err != nil {
				return fmt.Errorf("rename: %w", err)
			}
			return nil
		}
		return fmt.Errorf("server error: %s", resp.Status)
	default:
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	defer file.Close()

	// Track progress milestones for logging
	pw := &progressWriter{
		w:    file,
		done: &dl.doneBytes,
		onMilestone: func(pct int) {
			log.Printf("Download: %s — %d%%", dl.DisplayName, pct)
		},
		totalBytes: dl.TotalBytes,
	}

	if _, err := io.Copy(pw, resp.Body); err != nil {
		file.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("write error: %w", err)
	}
	file.Close()

	// Rename partial to final
	if err := os.Rename(partialPath, destPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// writeModelMeta creates a model.json alongside the GGUF file.
func (m *Manager) writeModelMeta(dir string, displayName string) {
	meta := struct {
		Name string `json:"name"`
	}{Name: displayName}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	metaPath := filepath.Join(dir, "model.json")
	// Don't overwrite existing model.json
	if _, err := os.Stat(metaPath); err == nil {
		return
	}
	if err := os.WriteFile(metaPath, append(data, '\n'), 0644); err != nil {
		log.Printf("Download: could not write model.json: %v", err)
	}
}

// progressWriter wraps an io.Writer and tracks bytes written atomically.
type progressWriter struct {
	w             io.Writer
	done          *atomic.Int64
	totalBytes    int64
	lastMilestone int
	onMilestone   func(pct int)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if n > 0 {
		current := pw.done.Add(int64(n))
		// Log milestones at 25%, 50%, 75%
		if pw.totalBytes > 0 && pw.onMilestone != nil {
			pct := int(current * 100 / pw.totalBytes)
			milestone := (pct / 25) * 25
			if milestone > pw.lastMilestone && milestone > 0 && milestone < 100 {
				pw.lastMilestone = milestone
				pw.onMilestone(milestone)
			}
		}
	}
	return n, err
}

func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("Invalid download URL.")
	}
	if !allowedHosts[u.Hostname()] {
		return fmt.Errorf("Downloads are only allowed from huggingface.co.")
	}
	return nil
}

func friendlyDownloadError(err error) string {
	if os.IsPermission(err) {
		return "Permission denied. Check that Fllint can write to the models folder."
	}
	errStr := err.Error()
	// Check for common errors
	if contains(errStr, "no space left") || contains(errStr, "disk full") {
		return "Disk is full. Free up some space and try again."
	}
	if contains(errStr, "connection refused") || contains(errStr, "no such host") {
		return "Could not connect. Check your internet connection and try again."
	}
	if contains(errStr, "timeout") || contains(errStr, "deadline exceeded") {
		return "Download timed out. Check your internet connection and try again."
	}
	return "Download failed. Please try again."
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
