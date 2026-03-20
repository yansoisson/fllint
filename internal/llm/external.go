package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
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
	roles      []string // assigned roles (e.g. "main", "summary")
	httpClient *http.Client

	ctxSize     int
	ctxSizeOnce sync.Once
}

// NewExternalEngine creates an engine that talks to an external server.
func NewExternalEngine(baseURL, apiKey, modelName, providerID, dataDir string, roles []string) *ExternalEngine {
	return &ExternalEngine{
		baseURL:    baseURL,
		apiKey:     apiKey,
		modelName:  modelName,
		providerID: providerID,
		dataDir:    dataDir,
		roles:      roles,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// HasRole checks if this engine is assigned to the given role.
// Engines with no roles default to "main" for backward compatibility.
func (e *ExternalEngine) HasRole(role string) bool {
	if len(e.roles) == 0 {
		return role == "main"
	}
	for _, r := range e.roles {
		if r == role {
			return true
		}
	}
	return false
}

// ProviderID returns the provider ID for this engine.
func (e *ExternalEngine) ProviderID() string {
	return e.providerID
}

// SetRoles updates the assigned roles.
func (e *ExternalEngine) SetRoles(roles []string) {
	e.roles = roles
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
		Model:         e.modelName,
		Messages:      oaiMsgs,
		Stream:        true,
		StreamOptions: &oaiStreamOptions{IncludeUsage: true},
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

	// Add tools if provided via context
	if toolDefs, ok := ctx.Value(ToolsKey).([]interface{}); ok && len(toolDefs) > 0 {
		req.Tools = toolDefs
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

// ContextSize returns the model's context window size, lazily fetched from the
// provider's API. Returns 0 if the provider doesn't support context size discovery.
func (e *ExternalEngine) ContextSize() int {
	e.ctxSizeOnce.Do(func() {
		e.ctxSize = e.fetchContextSize()
	})
	return e.ctxSize
}

// fetchContextSize queries the Ollama /api/show endpoint for the model's context length.
// Returns 0 on any error (graceful degradation for non-Ollama providers).
func (e *ExternalEngine) fetchContextSize() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload, _ := json.Marshal(map[string]string{"name": e.modelName})
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/show", bytes.NewReader(payload))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	// Try to extract context length from Ollama's response.
	// The format varies: model_info may have "general.context_length" or
	// the model parameters may specify num_ctx.
	var result struct {
		ModelInfo map[string]interface{} `json:"model_info"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0
	}

	// Search for any key ending in ".context_length" — Ollama uses
	// "{architecture}.context_length" (e.g. "general.context_length",
	// "llama.context_length", "kimi-k2.context_length").
	for key, v := range result.ModelInfo {
		if strings.HasSuffix(key, ".context_length") {
			if n, ok := v.(float64); ok && n > 0 {
				return int(n)
			}
		}
	}

	log.Printf("Could not determine context size for external model %s", e.modelName)
	return 0
}
