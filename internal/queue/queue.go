package queue

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"

	"github.com/fllint/fllint/internal/llm"
)

// Priority levels for queue items. Lower number = higher priority.
const (
	PriorityChat      = 1 // Main LLM inference (user chat)
	PriorityEmbedding = 2 // Embedding generation (future)
	PriorityOCR       = 3 // OCR processing (future)
	PrioritySummary   = 4 // Title/summary generation
)

// Item represents a single inference job in the queue.
type Item struct {
	ID       string
	ModelID  string
	Messages []llm.ChatMessage
	Priority int            // lower = higher priority (default: PriorityChat)
	TokenCh  chan llm.Token // tokens stream here from the worker
	DoneCh   chan struct{}  // closed when processing is done
	ErrCh    chan error     // send error if engine fails
	Ctx      context.Context
	Cancel   context.CancelFunc
}

// Queue manages a FIFO queue of inference jobs, processing one at a time.
type Queue struct {
	mu      sync.Mutex
	items   []*Item
	active  *Item
	manager *llm.Manager
	notify  chan struct{}
	stopCh  chan struct{}
	stopped bool
}

// NewQueue creates a new Queue and starts the worker goroutine.
func NewQueue(manager *llm.Manager) *Queue {
	q := &Queue{
		manager: manager,
		notify:  make(chan struct{}, 1),
		stopCh:  make(chan struct{}),
	}
	go q.worker()
	return q
}

// Enqueue adds an inference job to the queue with default (chat) priority
// and returns the item and its 0-indexed position.
func (q *Queue) Enqueue(ctx context.Context, modelID string, messages []llm.ChatMessage) (*Item, int) {
	return q.EnqueueWithPriority(ctx, modelID, messages, PriorityChat)
}

// EnqueueWithPriority adds an inference job to the queue with the given priority
// and returns the item and its 0-indexed position (0 means it will be processed
// next, after any active job).
func (q *Queue) EnqueueWithPriority(ctx context.Context, modelID string, messages []llm.ChatMessage, priority int) (*Item, int) {
	itemCtx, cancel := context.WithCancel(ctx)

	item := &Item{
		ID:       uuid.New().String(),
		ModelID:  modelID,
		Messages: messages,
		Priority: priority,
		TokenCh:  make(chan llm.Token, 100),
		DoneCh:   make(chan struct{}),
		ErrCh:    make(chan error, 1),
		Ctx:      itemCtx,
		Cancel:   cancel,
	}

	q.mu.Lock()
	q.items = append(q.items, item)
	position := len(q.items) - 1
	if q.active != nil {
		// If something is actively processing, the position as seen by the
		// user is the queue index + 1 (since position 0 = processing).
		position = len(q.items)
	}
	q.mu.Unlock()

	// Signal the worker that a new item is available.
	select {
	case q.notify <- struct{}{}:
	default:
	}

	return item, position
}

// Cancel cancels a queued or active item by its ID.
func (q *Queue) Cancel(itemID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if it's the active item.
	if q.active != nil && q.active.ID == itemID {
		q.active.Cancel()
		return
	}

	// Remove from the waiting queue.
	for i, item := range q.items {
		if item.ID == itemID {
			item.Cancel()
			close(item.DoneCh)
			q.items = append(q.items[:i], q.items[i+1:]...)
			return
		}
	}
}

// Position returns the current position of an item.
// Returns 0 if it is actively being processed, -1 if not found.
func (q *Queue) Position(itemID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active != nil && q.active.ID == itemID {
		return 0
	}

	for i, item := range q.items {
		if item.ID == itemID {
			// Position 1..N means waiting (1 = next up).
			return i + 1
		}
	}

	return -1
}

// Stop stops the worker goroutine and cancels all queued and active items.
func (q *Queue) Stop() {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	close(q.stopCh)

	// Cancel all pending items.
	for _, item := range q.items {
		item.Cancel()
		close(item.DoneCh)
	}
	q.items = nil

	// Cancel the active item.
	if q.active != nil {
		q.active.Cancel()
	}
	q.mu.Unlock()
}

// worker is the main loop that processes items one at a time.
func (q *Queue) worker() {
	for {
		// Wait for work or stop signal.
		select {
		case <-q.stopCh:
			return
		case <-q.notify:
		}

		// Process all available items.
		for {
			item := q.dequeue()
			if item == nil {
				break
			}

			// Check stop signal between items.
			select {
			case <-q.stopCh:
				return
			default:
			}

			q.processItem(item)
		}
	}
}

// dequeue removes and returns the highest-priority item from the queue,
// setting it as the active item. Within the same priority level, items are
// processed in FIFO order. Returns nil if the queue is empty.
func (q *Queue) dequeue() *Item {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	// Find the item with the lowest priority number (highest priority).
	// Among items with equal priority, the earliest (lowest index) wins (FIFO).
	bestIdx := 0
	for i := 1; i < len(q.items); i++ {
		if q.items[i].Priority < q.items[bestIdx].Priority {
			bestIdx = i
		}
	}

	item := q.items[bestIdx]
	q.items = append(q.items[:bestIdx], q.items[bestIdx+1:]...)
	q.active = item
	return item
}

// processItem runs a single inference job: gets the engine, streams tokens,
// and signals completion.
func (q *Queue) processItem(item *Item) {
	defer func() {
		// Close TokenCh so the SSE handler can drain it and detect completion.
		close(item.TokenCh)

		q.mu.Lock()
		if q.active == item {
			q.active = nil
		}
		q.mu.Unlock()

		// Close DoneCh to signal the SSE handler that this item is finished.
		// Use a recover in case it was already closed (e.g., by Cancel/Stop).
		defer func() { recover() }()
		close(item.DoneCh)
	}()

	// Check if the context was already cancelled (e.g., removed from queue).
	if item.Ctx.Err() != nil {
		return
	}

	// Get the engine for the requested model.
	engine := q.manager.GetEngine(item.ModelID)
	if engine == nil {
		// Fall back to the default engine if no model-specific engine is found.
		engine = q.manager.Engine()
	}
	if engine == nil {
		select {
		case item.ErrCh <- &QueueError{Message: "No model is loaded. Please load a model first."}:
		default:
		}
		return
	}

	// Start streaming from the engine.
	tokenCh, err := engine.ChatStream(item.Ctx, item.Messages)
	if err != nil {
		select {
		case item.ErrCh <- err:
		default:
		}
		return
	}

	// Pipe tokens from the engine channel to the item's TokenCh.
	for token := range tokenCh {
		select {
		case item.TokenCh <- token:
		case <-item.Ctx.Done():
			return
		case <-q.stopCh:
			return
		}
	}

	log.Printf("Queue: inference complete for item %s", item.ID)
}

// QueueError represents an error originating from the queue system.
type QueueError struct {
	Message string
}

func (e *QueueError) Error() string {
	return e.Message
}
