package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fllint/fllint/internal/config"
)

// ExternalEngine implements the Engine interface for external OpenAI-compatible
// model servers (e.g. Ollama). It does not manage a child process — the server
// is expected to be running independently.
type ExternalEngine struct {
	baseURL    string
	apiKey     string
	modelName  string
	providerID string
	dataDir    string
	httpClient *http.Client
}

// NewExternalEngine creates an engine that talks to an external server.
func NewExternalEngine(baseURL, apiKey, modelName, providerID, dataDir string) *ExternalEngine {
	return &ExternalEngine{
		baseURL:    baseURL,
		apiKey:     apiKey,
		modelName:  modelName,
		providerID: providerID,
		dataDir:    dataDir,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// ChatStream implements Engine. It POSTs to the server's OpenAI-compatible
// /v1/chat/completions endpoint and streams tokens back.
func (e *ExternalEngine) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error) {
	// Build messages — images are converted to base64 data URIs in the OpenAI
	// multimodal content format. If the external model doesn't support images,
	// the server will return an appropriate error.
	oaiMsgs := make([]oaiMessage, 0, len(messages))
	for _, m := range messages {
		msg, err := buildOAIMessageWithImages(e.dataDir, m)
		if err != nil {
			return nil, err
		}
		oaiMsgs = append(oaiMsgs, msg)
	}

	req := oaiRequest{
		Model:    e.modelName,
		Messages: oaiMsgs,
		Stream:   true,
	}

	// Optionally forward Fllint's inference params
	cfg := config.Get()
	if cfg != nil && cfg.ForwardParamsToExternal {
		req.Temperature = cfg.Temperature
		req.TopP = cfg.TopP
		if cfg.MaxTokens > 0 {
			req.MaxTokens = cfg.MaxTokens
		}
		if cfg.Seed >= 0 {
			req.Seed = cfg.Seed
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	url := e.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf(
			"Could not connect to the model server at %s. Make sure it is running.",
			e.baseURL,
		)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, fmt.Errorf("Authentication failed. Check your API key for this provider.")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		errMsg := strings.TrimSpace(string(respBody))
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("Model server error: %s", errMsg)
	}

	ch := make(chan Token)
	go parseOpenAISSE(ctx, resp.Body, ch)
	return ch, nil
}

// ModelName implements Engine.
func (e *ExternalEngine) ModelName() string {
	return e.modelName
}

// IsReady implements Engine. External engines are always ready.
func (e *ExternalEngine) IsReady() bool {
	return true
}
