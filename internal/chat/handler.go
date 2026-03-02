package chat

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
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
			http.Error(w, "failed to create conversation", http.StatusInternalServerError)
			return
		}
	} else {
		conv, err = h.store.Get(req.ConversationID)
		if err != nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return
		}
	}

	// Append user message
	userMsg := llm.ChatMessage{Role: "user", Content: req.Content}
	conv, err = h.store.AppendMessage(conv.ID, userMsg)
	if err != nil {
		http.Error(w, "failed to save message", http.StatusInternalServerError)
		return
	}

	// Start streaming from engine
	engine := h.manager.Engine()
	tokenCh, err := engine.ChatStream(r.Context(), conv.Messages)
	if err != nil {
		http.Error(w, "engine error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
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
	assistantMsg := llm.ChatMessage{Role: "assistant", Content: fullResponse}
	h.store.AppendMessage(conv.ID, assistantMsg)
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		req.Title = "New Conversation"
	}
	conv, err := h.store.Create(req.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, conv)
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conv, err := h.store.Get(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

func (h *Handler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
