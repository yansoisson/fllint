package summary

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/queue"
)

const (
	maxInputLen = 500
	maxTitleLen = 60
)

// TitleUpdater is the interface needed to update conversation titles.
type TitleUpdater interface {
	UpdateTitle(id string, title string) error
}

// Service generates conversation titles using a configured summary model.
type Service struct {
	store   TitleUpdater
	manager *llm.Manager
	queue   *queue.Queue
}

// NewService creates a new summary service.
func NewService(store TitleUpdater, manager *llm.Manager, q *queue.Queue) *Service {
	return &Service{
		store:   store,
		manager: manager,
		queue:   q,
	}
}

// GenerateTitle asynchronously generates a title for a conversation.
// It uses userContent if non-empty, otherwise falls back to assistantResponse.
// This method is meant to be called in a goroutine.
func (s *Service) GenerateTitle(convID, userContent, assistantResponse string) {
	text := strings.TrimSpace(userContent)
	if text == "" {
		text = strings.TrimSpace(assistantResponse)
		if len(text) > maxInputLen {
			text = text[:maxInputLen]
		}
	}
	if text == "" {
		return // nothing to summarize, keep "New chat"
	}

	// Resolve summary model ID
	cfg := config.Get()
	modelID := cfg.SummaryModelID
	if modelID == "" {
		modelID = s.manager.AutoDetectHelperModel("Summary")
	}

	// No summary model available — fall back to truncation
	if modelID == "" {
		title := text
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		if err := s.store.UpdateTitle(convID, title); err != nil {
			log.Printf("Summary: failed to update title for conv %s: %v", convID, err)
		}
		return
	}

	// Read the summary prompt from file (auto-creates with default if missing)
	summaryPrompt, promptErr := prompt.ReadSummaryPrompt(cfg.DataDir)
	if promptErr != nil {
		log.Printf("Summary: could not read summary prompt: %v", promptErr)
		summaryPrompt = prompt.DefaultSummaryPrompt
	}

	// Build the prompt
	messages := []llm.ChatMessage{
		{Role: "system", Content: summaryPrompt},
		{Role: "user", Content: text},
	}

	var title string
	var err error

	if llm.IsExternalModel(modelID) {
		// External models bypass the queue — no local resource contention
		title, err = s.generateExternal(modelID, messages)
	} else {
		// Local models go through the priority queue
		title, err = s.generateLocal(modelID, messages)
	}

	if err != nil {
		log.Printf("Summary: title generation failed for conv %s: %v", convID, err)
		// Fall back to truncation
		title = text
		if len(title) > 50 {
			title = title[:50] + "..."
		}
	}

	title = cleanTitle(title)
	if title == "" {
		return
	}

	if err := s.store.UpdateTitle(convID, title); err != nil {
		log.Printf("Summary: failed to update title for conv %s: %v", convID, err)
	}
}

// generateExternal calls an external model's ChatStream directly (no queue).
func (s *Service) generateExternal(modelID string, messages []llm.ChatMessage) (string, error) {
	engine := s.manager.GetEngine(modelID)
	if engine == nil {
		return "", fmt.Errorf("summary model %q not available", modelID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tokenCh, err := engine.ChatStream(ctx, messages)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for token := range tokenCh {
		result.WriteString(token.Content)
	}
	return result.String(), nil
}

// generateLocal enqueues the request with summary priority and collects the response.
func (s *Service) generateLocal(modelID string, messages []llm.ChatMessage) (string, error) {
	// Ensure the model is loaded
	if err := s.manager.LoadModel(modelID); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	item, _ := s.queue.EnqueueWithPriority(ctx, modelID, messages, queue.PrioritySummary)

	var result strings.Builder
	for {
		select {
		case token, ok := <-item.TokenCh:
			if !ok {
				return result.String(), nil
			}
			result.WriteString(token.Content)
		case err := <-item.ErrCh:
			return "", err
		case <-item.DoneCh:
			// Drain remaining tokens
			for token := range item.TokenCh {
				result.WriteString(token.Content)
			}
			return result.String(), nil
		case <-ctx.Done():
			s.queue.Cancel(item.ID)
			return "", ctx.Err()
		}
	}
}

// cleanTitle trims whitespace, removes surrounding quotes, and limits length.
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)

	// Remove surrounding quotes (single or double)
	if len(title) >= 2 {
		first, last := title[0], title[len(title)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			title = title[1 : len(title)-1]
			title = strings.TrimSpace(title)
		}
	}

	// Remove trailing period if present
	title = strings.TrimRight(title, ".")

	if len(title) > maxTitleLen {
		title = title[:maxTitleLen] + "..."
	}

	return title
}
