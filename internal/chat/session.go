package chat

import (
	"time"

	"github.com/fllint/fllint/internal/llm"
)

// Conversation represents a chat session.
type Conversation struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Messages  []llm.ChatMessage `json:"messages"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
