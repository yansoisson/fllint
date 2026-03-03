package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fllint/fllint/internal/llm"
)

// Handler holds dependencies for chat HTTP handlers.
type Handler struct {
	store   *Store
	manager *llm.Manager
}

func NewHandler(store *Store, manager *llm.Manager) *Handler {
	return &Handler{store: store, manager: manager}
}

// Routes returns a chi router with all chat-related routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/conversations", h.listConversations)
	r.Post("/conversations", h.createConversation)
	r.Get("/conversations/{id}", h.getConversation)
	r.Delete("/conversations/{id}", h.deleteConversation)

	r.Post("/chat", h.chat)

	return r
}

type chatRequest struct {
	ConversationID string   `json:"conversation_id"`
	Content        string   `json:"content"`
	Images         []string `json:"images,omitempty"`
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}

	if req.Content == "" && len(req.Images) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Message content or at least one image is required.")
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

	// Check that an engine is available before doing anything else
	engine := h.manager.Engine()
	if engine == nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "no_model",
			"No model is loaded. Select a model to get started.")
		return
	}

	// Auto-create conversation if none specified
	var conv *Conversation
	var err error
	if req.ConversationID == "" {
		title := req.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv, err = h.store.Create(title)
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

	// Append user message
	userMsg := llm.ChatMessage{Role: "user", Content: req.Content, Images: req.Images}
	conv, err = h.store.AppendMessage(conv.ID, userMsg)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "store_error",
			"Failed to save message.")
		return
	}

	// Start streaming from engine
	tokenCh, err := engine.ChatStream(r.Context(), conv.Messages)
	if err != nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "engine_error", err.Error())
		return
	}

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

	// Send conversation ID as first event
	fmt.Fprintf(w, "data: {\"conversation_id\":%q}\n\n", conv.ID)
	flusher.Flush()

	// Stream tokens
	var fullResponse string
	for token := range tokenCh {
		fullResponse += token.Content
		data, _ := json.Marshal(map[string]string{"content": token.Content})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Signal stream end
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Persist assistant response
	if fullResponse != "" {
		assistantMsg := llm.ChatMessage{Role: "assistant", Content: fullResponse}
		if _, err := h.store.AppendMessage(conv.ID, assistantMsg); err != nil {
			log.Printf("ERROR: failed to persist assistant message for conv %s: %v", conv.ID, err)
		}
	}
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
