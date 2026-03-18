package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/queue"
)

// TitleGenerator generates conversation titles asynchronously.
type TitleGenerator interface {
	GenerateTitle(convID, userContent, assistantResponse string)
}

// Handler holds dependencies for chat HTTP handlers.
type Handler struct {
	store    *Store
	manager  *llm.Manager
	queue    *queue.Queue
	titleGen TitleGenerator
}

func NewHandler(store *Store, manager *llm.Manager, q *queue.Queue, titleGen TitleGenerator) *Handler {
	return &Handler{store: store, manager: manager, queue: q, titleGen: titleGen}
}

// Routes returns a chi router with all chat-related routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/conversations", h.listConversations)
	r.Post("/conversations", h.createConversation)
	r.Delete("/conversations", h.deleteAllConversations)
	r.Get("/conversations/{id}", h.getConversation)
	r.Delete("/conversations/{id}", h.deleteConversation)

	r.Post("/chat", h.chat)
	r.Delete("/queue/{id}", h.cancelQueueItem)

	return r
}

type documentAttachment struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Text     string `json:"text"`
}

type chatRequest struct {
	ConversationID string               `json:"conversation_id"`
	Content        string               `json:"content"`
	Images         []string             `json:"images,omitempty"`
	Documents      []documentAttachment `json:"documents,omitempty"`
	ModelID        string               `json:"model_id,omitempty"`
	NoReasoning    bool                 `json:"no_reasoning,omitempty"`
	Retry          bool                 `json:"retry,omitempty"`
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	if req.Content == "" && len(req.Images) == 0 && len(req.Documents) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Message content, image, or document is required.")
		return
	}

	// Validate image URLs
	for _, imgURL := range req.Images {
		if !strings.HasPrefix(imgURL, "/api/uploads/") {
			writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid image URL: must start with /api/uploads/")
			return
		}
		filename := strings.TrimPrefix(imgURL, "/api/uploads/")
		if strings.Contains(filename, "/") || strings.Contains(filename, "..") || filename == "" {
			writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid image URL: contains invalid characters.")
			return
		}
	}

	// Validate document attachments
	for _, doc := range req.Documents {
		if !strings.HasPrefix(doc.URL, "/api/uploads/") {
			writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid document URL: must start with /api/uploads/")
			return
		}
		filename := strings.TrimPrefix(doc.URL, "/api/uploads/")
		if strings.Contains(filename, "/") || strings.Contains(filename, "..") || filename == "" {
			writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid document URL: contains invalid characters.")
			return
		}
		if doc.Filename == "" || doc.Text == "" {
			writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Document filename and text are required.")
			return
		}
	}

	// Auto-create or load conversation
	var conv *Conversation
	var err error
	isNewConversation := req.ConversationID == ""
	if isNewConversation {
		conv, err = h.store.Create("New chat")
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "store_error",
				"Failed to create conversation.")
			return
		}
	} else {
		conv, err = h.store.Get(req.ConversationID)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, "not_found",
				"Conversation not found.")
			return
		}
	}

	// Resolve which model to use:
	// 1. Explicit model_id in request (per-tab override)
	// 2. Conversation's stored model_id
	// 3. Default model (backward compat)
	modelID := req.ModelID
	if modelID == "" {
		modelID = conv.ModelID
	}

	// Verify a model is available before queueing
	if modelID != "" {
		engine := h.manager.GetEngine(modelID)
		if engine == nil {
			if h.manager.IsSwitching() {
				writeErrorJSON(w, http.StatusServiceUnavailable, "model_switching",
					"Model is loading — please wait a moment and try again.")
			} else {
				writeErrorJSON(w, http.StatusServiceUnavailable, "model_not_loaded",
					"The selected model is not loaded. Please load it first.")
			}
			return
		}
	} else {
		engine := h.manager.Engine()
		if engine == nil {
			if h.manager.IsSwitching() {
				writeErrorJSON(w, http.StatusServiceUnavailable, "model_switching",
					"Switching models — please wait a moment and try again.")
			} else {
				writeErrorJSON(w, http.StatusServiceUnavailable, "no_model",
					"No model is loaded. Select a model to get started.")
			}
			return
		}
	}

	// Bind model to conversation if not already bound
	if conv.ModelID == "" && modelID != "" {
		conv.ModelID = modelID
		if err := h.store.SetModelID(conv.ID, modelID); err != nil {
			log.Printf("WARNING: failed to set model_id on conv %s: %v", conv.ID, err)
		}
	}

	if req.Retry {
		// On retry (e.g. answer-now), remove the last assistant message if it was
		// a partial response from the cancelled request.
		if len(conv.Messages) > 0 && conv.Messages[len(conv.Messages)-1].Role == "assistant" {
			conv, err = h.store.RemoveLastMessage(conv.ID)
			if err != nil {
				log.Printf("WARNING: failed to remove partial assistant message for conv %s: %v", conv.ID, err)
			}
		}
	} else {
		// Append user message
		var docs []llm.DocumentAttachment
		for _, d := range req.Documents {
			docs = append(docs, llm.DocumentAttachment{
				Filename: d.Filename,
				URL:      d.URL,
				Text:     d.Text,
			})
		}
		userMsg := llm.ChatMessage{Role: "user", Content: req.Content, Images: req.Images, Documents: docs}
		conv, err = h.store.AppendMessage(conv.ID, userMsg)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "store_error",
				"Failed to save message.")
			return
		}
	}

	// Build messages with system prompt prepended
	cfg := config.Get()
	systemPrompt, err := prompt.ReadFromFile(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not read system prompt file: %v", err)
		systemPrompt = prompt.DefaultSystemPrompt
	}
	systemContent := prompt.Build(systemPrompt, cfg.CustomInstructions)

	var engineMessages []llm.ChatMessage
	if systemContent != "" {
		engineMessages = append(engineMessages, llm.ChatMessage{
			Role:    "system",
			Content: systemContent,
		})
	}
	engineMessages = append(engineMessages, conv.Messages...)

	// Enqueue the inference job
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if req.NoReasoning {
		ctx = context.WithValue(ctx, llm.NoReasoningKey, true)
	}

	item, position := h.queue.Enqueue(ctx, modelID, engineMessages)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorJSON(w, http.StatusInternalServerError, "server_error",
			"Streaming not supported.")
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send conversation ID and queue item ID as first event
	firstEvent, _ := json.Marshal(map[string]interface{}{
		"conversation_id": conv.ID,
		"queue_id":        item.ID,
		"position":        position,
	})
	fmt.Fprintf(w, "data: %s\n\n", firstEvent)
	flusher.Flush()

	// marshalToken builds the SSE JSON for a token, including reasoning if present.
	marshalToken := func(token llm.Token) []byte {
		event := map[string]string{}
		if token.Content != "" {
			event["content"] = token.Content
		}
		if token.Reasoning != "" {
			event["reasoning"] = token.Reasoning
		}
		data, _ := json.Marshal(event)
		return data
	}

	// If the item is queued (not immediately processing), send position
	// updates while waiting.
	var fullResponse string
	var fullReasoning string
	var thinkingStart time.Time
	var thinkingDuration *int

	if position > 0 {
		// Send periodic position updates while waiting in the queue.
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

	waitLoop:
		for {
			select {
			case <-ctx.Done():
				// Client disconnected or request cancelled.
				h.queue.Cancel(item.ID)
				return
			case token, ok := <-item.TokenCh:
				if !ok {
					// TokenCh closed — should not happen before DoneCh
					break waitLoop
				}
				// First token arrived — we're now processing.
				if token.Reasoning != "" && thinkingStart.IsZero() {
					thinkingStart = time.Now()
				}
				if token.Content != "" && !thinkingStart.IsZero() && thinkingDuration == nil {
					d := int(time.Since(thinkingStart).Seconds())
					thinkingDuration = &d
				}
				fullResponse += token.Content
				fullReasoning += token.Reasoning
				fmt.Fprintf(w, "data: %s\n\n", marshalToken(token))
				flusher.Flush()
				break waitLoop
			case err := <-item.ErrCh:
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			case <-item.DoneCh:
				// Item was completed or cancelled while in queue.
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			case <-ticker.C:
				pos := h.queue.Position(item.ID)
				if pos <= 0 {
					// Position 0 = processing, -1 = not found (done)
					continue
				}
				posData, _ := json.Marshal(map[string]interface{}{"position": pos})
				fmt.Fprintf(w, "data: %s\n\n", posData)
				flusher.Flush()
			}
		}
	}

	// Stream remaining tokens from the queue item.
	for {
		select {
		case <-ctx.Done():
			h.queue.Cancel(item.ID)
			goto done
		case token, ok := <-item.TokenCh:
			if !ok {
				goto done
			}
			if token.Reasoning != "" && thinkingStart.IsZero() {
				thinkingStart = time.Now()
			}
			if token.Content != "" && !thinkingStart.IsZero() && thinkingDuration == nil {
				d := int(time.Since(thinkingStart).Seconds())
				thinkingDuration = &d
			}
			fullResponse += token.Content
			fullReasoning += token.Reasoning
			fmt.Fprintf(w, "data: %s\n\n", marshalToken(token))
			flusher.Flush()
		case err := <-item.ErrCh:
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			goto done
		case <-item.DoneCh:
			// Drain any remaining tokens in the channel.
			for token := range item.TokenCh {
				if token.Reasoning != "" && thinkingStart.IsZero() {
					thinkingStart = time.Now()
				}
				if token.Content != "" && !thinkingStart.IsZero() && thinkingDuration == nil {
					d := int(time.Since(thinkingStart).Seconds())
					thinkingDuration = &d
				}
				fullResponse += token.Content
				fullReasoning += token.Reasoning
				fmt.Fprintf(w, "data: %s\n\n", marshalToken(token))
				flusher.Flush()
			}
			goto done
		}
	}

done:
	// Send thinking duration if we tracked it
	if thinkingDuration != nil {
		data, _ := json.Marshal(map[string]interface{}{"thinking_duration": *thinkingDuration})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Signal stream end
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Persist assistant response (only if there's actual content — reasoning-only
	// partial responses from cancellations are discarded)
	if fullResponse != "" {
		assistantMsg := llm.ChatMessage{
			Role:             "assistant",
			Content:          fullResponse,
			Reasoning:        fullReasoning,
			ThinkingDuration: thinkingDuration,
		}
		if _, err := h.store.AppendMessage(conv.ID, assistantMsg); err != nil {
			log.Printf("ERROR: failed to persist assistant message for conv %s: %v", conv.ID, err)
		}
	}

	// Generate a title for new conversations asynchronously
	if isNewConversation && h.titleGen != nil {
		go h.titleGen.GenerateTitle(conv.ID, req.Content, fullResponse)
	}
}

func (h *Handler) cancelQueueItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Queue item ID is required.")
		return
	}
	h.queue.Cancel(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := h.store.List()
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "store_error", "Failed to load conversations.")
		return
	}
	if convs == nil {
		convs = []Conversation{}
	}
	writeJSON(w, http.StatusOK, convs)
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.Title == "" {
		req.Title = "New Conversation"
	}
	conv, err := h.store.Create(req.Title)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "store_error", "Failed to create conversation.")
		return
	}
	writeJSON(w, http.StatusCreated, conv)
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conv, err := h.store.Get(id)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Conversation not found.")
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

func (h *Handler) deleteAllConversations(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.DeleteAll(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "store_error",
			"Failed to delete conversations.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(id); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "store_error", "Failed to delete conversation.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeErrorJSON(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiError{Error: message, Code: code})
}
