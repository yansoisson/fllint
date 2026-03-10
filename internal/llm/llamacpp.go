package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EngineState represents the lifecycle state of the llama-server process.
type EngineState int

const (
	EngineStateIdle     EngineState = iota // No process running
	EngineStateStarting                    // Process launched, waiting for health
	EngineStateReady                       // Health check passed, accepting requests
	EngineStateError                       // Process failed or crashed
	EngineStateStopping                    // Graceful shutdown in progress
)

func (s EngineState) String() string {
	switch s {
	case EngineStateIdle:
		return "idle"
	case EngineStateStarting:
		return "starting"
	case EngineStateReady:
		return "ready"
	case EngineStateError:
		return "error"
	case EngineStateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// InferenceParams holds per-request inference parameters read from config.
type InferenceParams struct {
	Temperature   float64
	TopP          float64
	TopK          int
	RepeatPenalty float64
	MaxTokens     int
	Seed          int
}

// LlamaCppEngine manages a llama-server child process and communicates
// with it via the OpenAI-compatible HTTP API.
type LlamaCppEngine struct {
	mu sync.RWMutex

	// Configuration (immutable after construction)
	serverBinaryPath string
	modelPath        string
	modelName        string
	mmprojPath       string // optional multimodal projector for vision
	port             int
	dataDir          string
	ctxSize          int
	nGPULayers       int
	flashAttn        string // "auto", "on", "off"
	mmap             bool

	// Inference parameters (guarded by mu, can be updated live)
	inferenceParams InferenceParams

	// Runtime state (guarded by mu)
	state        EngineState
	loadProgress float64 // 0.0–1.0, updated during model loading from /health
	stateErr error
	cmd      *exec.Cmd
	cancel   context.CancelFunc

	// processDone is closed when the supervise goroutine exits (process fully reaped).
	// Stop() waits on this instead of calling cmd.Wait() a second time.
	processDone chan struct{}

	httpClient *http.Client
}

// LlamaCppConfig holds parameters for creating a LlamaCppEngine.
type LlamaCppConfig struct {
	ServerBinaryPath string
	ModelPath        string
	ModelName        string
	MmprojPath       string // optional path to mmproj file for vision models
	CtxSize          int
	NGPULayers       int
	FlashAttn        string // "auto", "on", "off"
	Mmap             bool   // enable memory-mapped I/O for SSD-backed loading
	DataDir          string // For resolving image URLs to disk paths
	InferenceParams  InferenceParams
}

// NewLlamaCppEngine creates a new engine but does not start the server process.
// Call Start() to launch the llama-server.
func NewLlamaCppEngine(cfg LlamaCppConfig) (*LlamaCppEngine, error) {
	if _, err := os.Stat(cfg.ServerBinaryPath); err != nil {
		return nil, fmt.Errorf(
			"llama-server binary not found at %q. "+
				"Download it from https://github.com/ggml-org/llama.cpp/releases "+
				"and place it in the bin/ folder next to the Fllint app",
			cfg.ServerBinaryPath,
		)
	}
	if _, err := os.Stat(cfg.ModelPath); err != nil {
		return nil, fmt.Errorf(
			"Model file not found at %q. "+
				"Download a .gguf model file and place it in the models/ folder",
			cfg.ModelPath,
		)
	}

	if cfg.CtxSize == 0 {
		cfg.CtxSize = 4096
	}
	if cfg.NGPULayers == 0 {
		cfg.NGPULayers = 999
	}
	if cfg.FlashAttn == "" {
		cfg.FlashAttn = "auto"
	}

	port, err := findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("could not find a free port for llama-server: %w", err)
	}

	return &LlamaCppEngine{
		serverBinaryPath: cfg.ServerBinaryPath,
		modelPath:        cfg.ModelPath,
		modelName:        cfg.ModelName,
		mmprojPath:       cfg.MmprojPath,
		port:             port,
		dataDir:          cfg.DataDir,
		ctxSize:          cfg.CtxSize,
		nGPULayers:       cfg.NGPULayers,
		flashAttn:        cfg.FlashAttn,
		mmap:             cfg.Mmap,
		inferenceParams:  cfg.InferenceParams,
		state:            EngineStateIdle,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// Start launches the llama-server process and begins health polling.
func (e *LlamaCppEngine) Start() error {
	e.mu.Lock()
	if e.state == EngineStateStarting || e.state == EngineStateReady {
		e.mu.Unlock()
		return nil
	}
	e.state = EngineStateStarting
	e.stateErr = nil
	e.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	args := []string{
		"--model", e.modelPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", e.port),
		"--ctx-size", fmt.Sprintf("%d", e.ctxSize),
		"--n-gpu-layers", fmt.Sprintf("%d", e.nGPULayers),
	}
	if e.flashAttn != "" && e.flashAttn != "off" {
		args = append(args, "--flash-attn", e.flashAttn)
	}
	if e.mmap {
		args = append(args, "--mmap")
	}
	if e.mmprojPath != "" {
		args = append(args, "--mmproj", e.mmprojPath)
	}

	// Use exec.Command (NOT CommandContext) — we handle process lifecycle
	// manually in supervise/Stop to avoid Go's internal double-Wait issues.
	cmd := exec.Command(e.serverBinaryPath, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	// Set library path so llama-server can find its shared libraries in bin/
	binDir := filepath.Dir(e.serverBinaryPath)
	cmd.Env = append(os.Environ(),
		"DYLD_LIBRARY_PATH="+binDir,
		"LD_LIBRARY_PATH="+binDir,
	)

	if err := cmd.Start(); err != nil {
		cancel()
		e.mu.Lock()
		e.state = EngineStateError
		if os.IsPermission(err) {
			e.stateErr = fmt.Errorf(
				"Failed to start llama-server: permission denied. "+
					"Run: chmod +x %s",
				e.serverBinaryPath,
			)
		} else {
			e.stateErr = fmt.Errorf(
				"Failed to start llama-server: %v. "+
					"Check that the binary at %q is valid for your platform",
				err, e.serverBinaryPath,
			)
		}
		result := e.stateErr
		e.mu.Unlock()
		return result
	}

	processDone := make(chan struct{})

	e.mu.Lock()
	e.cmd = cmd
	e.cancel = cancel
	e.processDone = processDone
	e.mu.Unlock()

	go e.supervise(ctx, cmd, processDone)

	return nil
}

// supervise polls health until ready, then monitors for crashes.
// It owns the single cmd.Wait() call and closes processDone when the
// process has been fully reaped.
func (e *LlamaCppEngine) supervise(ctx context.Context, cmd *exec.Cmd, processDone chan struct{}) {
	defer close(processDone)

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", e.port)

	const (
		healthPollInterval = 500 * time.Millisecond
		healthTimeout      = 5 * time.Minute
	)

	deadline := time.Now().Add(healthTimeout)
	healthy := false
	healthClient := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			// Stop() was called during startup — kill and reap the process.
			log.Printf("[supervise] context cancelled during health poll, killing process")
			cmd.Process.Kill()
			cmd.Wait()
			return
		default:
		}

		resp, err := healthClient.Get(healthURL)
		if err == nil {
			var body struct {
				Status   string  `json:"status"`
				Progress float64 `json:"progress"`
			}
			json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if body.Progress > 0 {
				e.mu.Lock()
				e.loadProgress = body.Progress
				e.mu.Unlock()
			}
			if body.Status == "ok" {
				e.mu.Lock()
				e.loadProgress = 1.0
				e.mu.Unlock()
				healthy = true
				break
			}
		}
		time.Sleep(healthPollInterval)
	}

	if !healthy {
		e.mu.Lock()
		e.state = EngineStateError
		e.stateErr = fmt.Errorf(
			"The model didn't finish loading in time. " +
				"It may need more memory than available. Try a smaller model",
		)
		e.mu.Unlock()
		cmd.Process.Kill()
		cmd.Wait()
		return
	}

	e.mu.Lock()
	e.state = EngineStateReady
	e.stateErr = nil
	e.mu.Unlock()
	log.Printf("llama-server ready on port %d with model %s", e.port, e.modelName)

	// Monitor for process exit (crash detection).
	// This goroutine owns the only cmd.Wait() call.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		// Stop() was called — kill the process and wait for it to exit.
		log.Printf("[supervise] context cancelled, killing process")
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-waitCh
		}
		return
	case err := <-waitCh:
		e.mu.Lock()
		if e.state == EngineStateStopping {
			e.state = EngineStateIdle
		} else {
			e.state = EngineStateError
			e.stateErr = fmt.Errorf(
				"llama-server stopped unexpectedly (%v). "+
					"This may indicate insufficient memory or a problem with the model file. "+
					"Try selecting a different model",
				err,
			)
			log.Printf("llama-server crashed: %v", err)
		}
		e.mu.Unlock()
	}
}

// Stop gracefully shuts down the llama-server process.
// It cancels the context (which tells supervise to kill & reap the process)
// and waits for the supervise goroutine to finish via processDone.
func (e *LlamaCppEngine) Stop() {
	e.mu.Lock()
	if e.state == EngineStateIdle || e.state == EngineStateStopping {
		e.mu.Unlock()
		return
	}
	e.state = EngineStateStopping
	cancel := e.cancel
	processDone := e.processDone
	e.mu.Unlock()

	// Signal the supervise goroutine to kill the process.
	if cancel != nil {
		cancel()
	}

	// Wait for supervise to finish (it handles kill + cmd.Wait).
	if processDone != nil {
		<-processDone
	}

	e.mu.Lock()
	e.state = EngineStateIdle
	e.stateErr = nil
	e.cmd = nil
	e.cancel = nil
	e.processDone = nil
	e.mu.Unlock()
}

// killProcess is no longer needed — supervise owns all process lifecycle.
// Kept as a no-op for safety.

// ChatStream implements the Engine interface. It sends messages to the
// llama-server /v1/chat/completions endpoint and streams tokens back.
func (e *LlamaCppEngine) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error) {
	e.mu.RLock()
	state := e.state
	stateErr := e.stateErr
	port := e.port
	e.mu.RUnlock()

	switch state {
	case EngineStateIdle:
		return nil, fmt.Errorf("No model is loaded. Select a model to get started.")
	case EngineStateStarting:
		return nil, fmt.Errorf(
			"The model is still loading. This can take a minute for larger models. Please wait and try again.",
		)
	case EngineStateError:
		if stateErr != nil {
			return nil, stateErr
		}
		return nil, fmt.Errorf("The model engine encountered an error. Try selecting a model again.")
	case EngineStateStopping:
		return nil, fmt.Errorf("The model is shutting down. Please wait.")
	case EngineStateReady:
		// proceed
	}

	// Build OpenAI-compatible request
	e.mu.RLock()
	params := e.inferenceParams
	e.mu.RUnlock()

	oaiMsgs := make([]oaiMessage, 0, len(messages))
	for _, m := range messages {
		msg, err := e.buildOAIMessage(m)
		if err != nil {
			return nil, err
		}
		oaiMsgs = append(oaiMsgs, msg)
	}

	req := oaiRequest{
		Model:         e.modelName,
		Messages:      oaiMsgs,
		Stream:        true,
		Temperature:   params.Temperature,
		TopP:          params.TopP,
		TopK:          params.TopK,
		RepeatPenalty: params.RepeatPenalty,
	}
	if params.MaxTokens > 0 {
		req.MaxTokens = params.MaxTokens
	}
	if params.Seed >= 0 {
		req.Seed = params.Seed
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf(
			"Failed to connect to the model server. It may have stopped. " +
				"Try selecting the model again.",
		)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf(
			"Model server returned an error (%d): %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)),
		)
	}

	ch := make(chan Token)
	go e.parseSSEStream(ctx, resp.Body, ch)
	return ch, nil
}

// parseSSEStream reads the OpenAI SSE format and sends tokens to the channel.
func (e *LlamaCppEngine) parseSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- Token) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		content := chunk.Choices[0].Delta.Content
		reasoning := chunk.Choices[0].Delta.ReasoningContent
		if content == "" && reasoning == "" {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case ch <- Token{Content: content, Reasoning: reasoning}:
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		log.Printf("SSE stream read error: %v", err)
	}
}

func (e *LlamaCppEngine) ModelName() string {
	return e.modelName
}

func (e *LlamaCppEngine) IsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state == EngineStateReady
}

// State returns the current engine state and any associated error.
func (e *LlamaCppEngine) State() (EngineState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state, e.stateErr
}

// LoadProgress returns the model loading progress (0.0–1.0).
func (e *LlamaCppEngine) LoadProgress() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loadProgress
}

// --- OpenAI-compatible request types ---

type oaiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []oaiContentPart
}

type oaiRequest struct {
	Model         string       `json:"model"`
	Messages      []oaiMessage `json:"messages"`
	Stream        bool         `json:"stream"`
	Temperature   float64      `json:"temperature,omitempty"`
	TopP          float64      `json:"top_p,omitempty"`
	TopK          int          `json:"top_k,omitempty"`
	RepeatPenalty float64      `json:"repeat_penalty,omitempty"`
	MaxTokens     int          `json:"max_tokens,omitempty"`
	Seed          int          `json:"seed,omitempty"`
}

type oaiContentPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *oaiImageURL `json:"image_url,omitempty"`
}

type oaiImageURL struct {
	URL string `json:"url"`
}

// SetInferenceParams updates the inference parameters used for new requests.
// This does not require an engine restart.
func (e *LlamaCppEngine) SetInferenceParams(p InferenceParams) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inferenceParams = p
}

// HasVision reports whether this engine was started with a multimodal projector.
func (e *LlamaCppEngine) HasVision() bool {
	return e.mmprojPath != ""
}

// buildOAIMessage converts a ChatMessage to the OpenAI format. Text-only
// messages use a plain string for Content; messages with images use a
// content array with text and image_url parts.
func (e *LlamaCppEngine) buildOAIMessage(m ChatMessage) (oaiMessage, error) {
	msg := oaiMessage{Role: m.Role}

	if len(m.Images) == 0 {
		msg.Content = m.Content
		return msg, nil
	}

	if !e.HasVision() {
		return oaiMessage{}, fmt.Errorf(
			"This model doesn't support image input. " +
				"To use images, place a mmproj .gguf file in the models/ folder " +
				"(download one that matches your model from HuggingFace)",
		)
	}

	// Multimodal: build content array
	var parts []oaiContentPart

	if m.Content != "" {
		parts = append(parts, oaiContentPart{
			Type: "text",
			Text: m.Content,
		})
	}

	for _, imgURL := range m.Images {
		dataURI, err := e.imageToDataURI(imgURL)
		if err != nil {
			return oaiMessage{}, fmt.Errorf("failed to process image %s: %w", imgURL, err)
		}
		parts = append(parts, oaiContentPart{
			Type:     "image_url",
			ImageURL: &oaiImageURL{URL: dataURI},
		})
	}

	msg.Content = parts
	return msg, nil
}

// imageToDataURI reads an uploaded image file from disk and returns a
// base64-encoded data URI suitable for the OpenAI vision API.
func (e *LlamaCppEngine) imageToDataURI(imgURL string) (string, error) {
	filename := strings.TrimPrefix(imgURL, "/api/uploads/")
	if filename == imgURL || filename == "" {
		return "", fmt.Errorf("invalid image URL format")
	}

	filePath := filepath.Join(e.dataDir, "uploads", filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot read image file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mime := "image/jpeg"
	switch ext {
	case ".png":
		mime = "image/png"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

// findAvailablePort asks the OS for a free TCP port on localhost.
func findAvailablePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}
