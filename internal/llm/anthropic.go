package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fllint/fllint/internal/config"
)

const anthropicAPIVersion = "2023-06-01"

// AnthropicEngine implements the Engine interface for Anthropic's Messages API.
// Unlike ExternalEngine (OpenAI-compatible), Anthropic uses a different request
// format, auth header, and SSE event structure.
type AnthropicEngine struct {
	baseURL    string
	apiKey     string
	modelName  string
	providerID string
	dataDir    string
	roles      []string
	httpClient *http.Client
}

// NewAnthropicEngine creates an engine for the Anthropic Messages API.
func NewAnthropicEngine(baseURL, apiKey, modelName, providerID, dataDir string, roles []string) *AnthropicEngine {
	return &AnthropicEngine{
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
func (e *AnthropicEngine) HasRole(role string) bool {
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
func (e *AnthropicEngine) ProviderID() string {
	return e.providerID
}

// SetRoles updates the assigned roles.
func (e *AnthropicEngine) SetRoles(roles []string) {
	e.roles = roles
}

// -- Anthropic request types --

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
	// Optional params
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	TopK        *int     `json:"top_k,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"` // "user" or "assistant"
	Content interface{} `json:"content"`
}

type anthropicContentBlock struct {
	Type   string `json:"type"` // "text" or "image"
	Text   string `json:"text,omitempty"`
	Source *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png", etc.
	Data      string `json:"data"`
}

// ChatStream implements Engine.
func (e *AnthropicEngine) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error) {
	// Separate system prompt from messages
	var system string
	var anthropicMsgs []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}

		// Build content blocks (text + optional images)
		var blocks []anthropicContentBlock

		// Add images as content blocks
		for _, imgURL := range m.Images {
			data, mediaType, err := resolveImageToBase64(e.dataDir, imgURL)
			if err != nil {
				continue
			}
			blocks = append(blocks, anthropicContentBlock{
				Type: "image",
				Source: &anthropicImageSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      data,
				},
			})
		}

		// Add text content
		content := m.Content
		// Prepend document text if present
		for _, doc := range m.Documents {
			if doc.Text != "" {
				content = fmt.Sprintf("<document filename=\"%s\">\n%s\n</document>\n\n%s", doc.Filename, doc.Text, content)
			}
		}

		if len(blocks) > 0 {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: content})
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    m.Role,
				Content: blocks,
			})
		} else {
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    m.Role,
				Content: content,
			})
		}
	}

	maxTokens := 4096
	cfg := config.Get()
	if cfg != nil && cfg.ForwardParamsToExternal && cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}

	req := anthropicRequest{
		Model:     e.modelName,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  anthropicMsgs,
		Stream:    true,
	}

	// Forward optional params
	if cfg != nil && cfg.ForwardParamsToExternal {
		t := cfg.Temperature
		req.Temperature = &t
		p := cfg.TopP
		req.TopP = &p
		if cfg.TopK > 0 {
			k := cfg.TopK
			req.TopK = &k
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	url := e.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", e.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Could not connect to Anthropic at %s.", e.baseURL)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, fmt.Errorf("Authentication failed. Check your Anthropic API key.")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		errMsg := strings.TrimSpace(string(respBody))
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("Anthropic error: %s", errMsg)
	}

	ch := make(chan Token)
	go parseAnthropicSSE(ctx, resp.Body, ch)
	return ch, nil
}

// ModelName implements Engine.
func (e *AnthropicEngine) ModelName() string {
	return e.modelName
}

// IsReady implements Engine.
func (e *AnthropicEngine) IsReady() bool {
	return true
}

// ContextSize implements Engine.
func (e *AnthropicEngine) ContextSize() int {
	return 0 // Anthropic doesn't expose this via API
}

// -- SSE parsing for Anthropic's event format --

func parseAnthropicSSE(ctx context.Context, body io.ReadCloser, ch chan<- Token) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		// Track event type
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		switch currentEvent {
		case "content_block_delta":
			var event struct {
				Delta struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					Thinking string `json:"thinking"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			tok := Token{}
			switch event.Delta.Type {
			case "text_delta":
				tok.Content = event.Delta.Text
			case "thinking_delta":
				tok.Reasoning = event.Delta.Thinking
			}
			if tok.Content != "" || tok.Reasoning != "" {
				select {
				case ch <- tok:
				case <-ctx.Done():
					return
				}
			}

		case "message_delta":
			var event struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			tok := Token{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				FinishReason:      "stop",
			}
			select {
			case ch <- tok:
			case <-ctx.Done():
				return
			}

		case "message_stop":
			return

		case "error":
			var event struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil && event.Error.Message != "" {
				select {
				case ch <- Token{Content: "\n\n[Error: " + event.Error.Message + "]"}:
				case <-ctx.Done():
				}
			}
			return
		}
	}
}

// resolveImageToBase64 reads an uploaded image and returns raw base64 data and media type.
func resolveImageToBase64(dataDir string, imgURL string) (string, string, error) {
	filename := strings.TrimPrefix(imgURL, "/api/uploads/")
	if filename == imgURL || filename == "" {
		return "", "", fmt.Errorf("invalid image URL format")
	}

	filePath := filepath.Join(dataDir, "uploads", filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("cannot read image file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mediaType := "image/jpeg"
	switch ext {
	case ".png":
		mediaType = "image/png"
	case ".gif":
		mediaType = "image/gif"
	case ".webp":
		mediaType = "image/webp"
	}

	return base64.StdEncoding.EncodeToString(data), mediaType, nil
}
