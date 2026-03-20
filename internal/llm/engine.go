package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// NoReasoningKey is the context key for the no-reasoning flag.
const NoReasoningKey contextKey = "no_reasoning"

// DocumentAttachment represents an uploaded document with its extracted text.
type DocumentAttachment struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Text     string `json:"text"`
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role             string               `json:"role"` // "user", "assistant", "system", "tool"
	Content          string               `json:"content"`
	Reasoning        string               `json:"reasoning,omitempty"`
	ThinkingDuration *int                 `json:"thinking_duration,omitempty"`
	Images           []string             `json:"images,omitempty"`    // URLs like "/api/uploads/uuid.png"
	Documents        []DocumentAttachment `json:"documents,omitempty"` // Attached documents with extracted text
	ToolCalls        []ToolCall           `json:"tool_calls,omitempty"` // Tool calls made by assistant
	ToolCallID       string               `json:"tool_call_id,omitempty"` // For tool result messages
}

// ToolCall represents a function call requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Token represents a single streamed token from the LLM.
// Usage fields are set once on the final chunk when stream_options.include_usage is true.
type Token struct {
	Content          string     `json:"content,omitempty"`
	Reasoning        string     `json:"reasoning,omitempty"`
	PromptTokens     int        `json:"prompt_tokens,omitempty"`
	CompletionTokens int        `json:"completion_tokens,omitempty"`
	FinishReason     string     `json:"finish_reason,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolStatus       string     `json:"tool_status,omitempty"` // "searching", "fetching" — for frontend indicator
}

// ToolsKey is the context key for passing tool definitions to engines.
type toolsKeyType string
const ToolsKey toolsKeyType = "tools"

// Engine defines the interface for LLM inference backends.
type Engine interface {
	// ChatStream sends messages to the model and returns a channel of tokens.
	// The channel is closed when generation is complete.
	ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error)

	// ModelName returns the display name of the currently loaded model.
	ModelName() string

	// IsReady reports whether the engine is ready to accept requests.
	IsReady() bool

	// ContextSize returns the model's context window size (n_ctx).
	// Returns 0 if unknown.
	ContextSize() int
}

// StubEngine is a placeholder that returns canned responses for development.
type StubEngine struct{}

func NewStubEngine() *StubEngine {
	return &StubEngine{}
}

func (s *StubEngine) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error) {
	ch := make(chan Token)
	go func() {
		defer close(ch)
		response := "Hello! I'm Fllint, your local AI assistant. " +
			"This is a stubbed response — the real LLM engine isn't connected yet. " +
			"Once a model is loaded, you'll see actual inference results streamed here token by token. " +
			"For now, everything is working end-to-end: your message was received, " +
			"processed through the API, and this response is being streamed back via SSE."

		for _, word := range splitIntoTokens(response) {
			select {
			case <-ctx.Done():
				return
			case ch <- Token{Content: word}:
				time.Sleep(30 * time.Millisecond)
			}
		}
	}()
	return ch, nil
}

func (s *StubEngine) ModelName() string {
	return "stub-model"
}

func (s *StubEngine) IsReady() bool {
	return true
}

func (s *StubEngine) ContextSize() int {
	return 4096
}

// imageToDataURI reads an uploaded image file from disk and returns a
// base64-encoded data URI suitable for the OpenAI vision API.
// Shared between LlamaCppEngine and ExternalEngine.
func imageToDataURI(dataDir string, imgURL string) (string, error) {
	filename := strings.TrimPrefix(imgURL, "/api/uploads/")
	if filename == imgURL || filename == "" {
		return "", fmt.Errorf("invalid image URL format")
	}

	filePath := filepath.Join(dataDir, "uploads", filename)

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

// buildContentWithDocuments prepends document text to the user's message content.
// Documents are wrapped in XML tags so the LLM can distinguish them from the user's message.
func buildContentWithDocuments(content string, docs []DocumentAttachment) string {
	if len(docs) == 0 {
		return content
	}
	var sb strings.Builder
	for _, doc := range docs {
		sb.WriteString(fmt.Sprintf("<document filename=\"%s\">\n", doc.Filename))
		sb.WriteString(doc.Text)
		sb.WriteString("\n</document>\n\n")
	}
	if content != "" {
		sb.WriteString(content)
	}
	return sb.String()
}

// buildOAIMessageWithImages converts a ChatMessage to an OAI message,
// handling images by converting them to base64 data URIs.
// Shared between LlamaCppEngine and ExternalEngine.
func buildOAIMessageWithImages(dataDir string, m ChatMessage) (oaiMessage, error) {
	msg := oaiMessage{Role: m.Role}

	// Handle tool result messages
	if m.Role == "tool" {
		msg.Content = m.Content
		msg.ToolCallID = m.ToolCallID
		return msg, nil
	}

	// Handle assistant messages with tool calls
	if m.Role == "assistant" && len(m.ToolCalls) > 0 {
		msg.Content = m.Content
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, oaiToolCallChunk{
				ID: tc.ID,
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		return msg, nil
	}

	// Inject document text into content
	effectiveContent := buildContentWithDocuments(m.Content, m.Documents)

	if len(m.Images) == 0 {
		msg.Content = effectiveContent
		return msg, nil
	}

	var parts []oaiContentPart
	if effectiveContent != "" {
		parts = append(parts, oaiContentPart{
			Type: "text",
			Text: effectiveContent,
		})
	}

	for _, imgURL := range m.Images {
		dataURI, err := imageToDataURI(dataDir, imgURL)
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

// splitIntoTokens breaks text into word-level tokens preserving spaces.
func splitIntoTokens(text string) []string {
	var tokens []string
	current := ""
	for _, ch := range text {
		if ch == ' ' {
			if current != "" {
				tokens = append(tokens, current+" ")
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}
