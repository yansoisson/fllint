package llm

import (
	"context"
	"time"
)

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// Token represents a single streamed token from the LLM.
type Token struct {
	Content string `json:"content"`
}

// Engine defines the interface for LLM inference backends.
type Engine interface {
	// ChatStream sends messages to the model and returns a channel of tokens.
	// The channel is closed when generation is complete.
	ChatStream(ctx context.Context, messages []ChatMessage) (<-chan Token, error)

	// ModelName returns the display name of the currently loaded model.
	ModelName() string

	// IsReady reports whether the engine is ready to accept requests.
	IsReady() bool
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
