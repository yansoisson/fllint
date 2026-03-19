package ocr

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/document"
	"github.com/fllint/fllint/internal/llm"
	"github.com/fllint/fllint/internal/prompt"
	"github.com/fllint/fllint/internal/queue"
)

const maxRetries = 3

// PageImage maps a 1-based page number to its uploaded image URL.
type PageImage struct {
	PageNum  int    `json:"page_num"`
	ImageURL string `json:"image_url"`
}

// Job represents an OCR processing job.
type Job struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"` // "processing", "complete", "error", "cancelled"
	TotalPages  int      `json:"total_pages"`
	DonePages   int      `json:"done_pages"`
	FailedPages []int    `json:"failed_pages,omitempty"`
	Error       string   `json:"error,omitempty"`
	ResultText  string   `json:"result_text,omitempty"`
	ResultURL   string   `json:"result_url,omitempty"`

	Mtx    sync.Mutex `json:"-"`
	cancel context.CancelFunc
}

// Service manages OCR processing jobs.
type Service struct {
	manager *llm.Manager
	queue   *queue.Queue
	dataDir string

	mu   sync.Mutex
	jobs map[string]*Job
}

// NewService creates a new OCR service.
func NewService(manager *llm.Manager, q *queue.Queue, dataDir string) *Service {
	return &Service{
		manager: manager,
		queue:   q,
		dataDir: dataDir,
		jobs:    make(map[string]*Job),
	}
}

// StartOCR begins OCR processing.
// pdfURL is the uploaded PDF (for text extraction of non-OCR pages).
// totalPages is the total number of pages in the PDF.
// ocrPages maps page numbers to their rendered image URLs.
func (s *Service) StartOCR(pdfURL string, totalPages int, ocrPages []PageImage) (string, error) {
	if totalPages == 0 {
		return "", fmt.Errorf("No pages to process.")
	}

	// Resolve OCR model — no auto-detect, respect user's "None" choice
	cfg := config.Get()
	modelID := cfg.OCRModelID
	if modelID == "" {
		return "", fmt.Errorf("No OCR model configured. Please configure one in Settings.")
	}

	// Verify model is available
	engine := s.manager.GetEngine(modelID)
	if engine == nil && !llm.IsExternalModel(modelID) {
		if err := s.manager.LoadModel(modelID); err != nil {
			return "", fmt.Errorf("Failed to load OCR model: %v", err)
		}
	}

	jobID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	job := &Job{
		ID:         jobID,
		Status:     "processing",
		TotalPages: totalPages,
		DonePages:  0,
		cancel:     cancel,
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	go s.processJob(ctx, job, modelID, pdfURL, totalPages, ocrPages)

	return jobID, nil
}

// GetJob returns the current status of an OCR job.
func (s *Service) GetJob(jobID string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[jobID]
}

// CancelJob cancels a running OCR job.
func (s *Service) CancelJob(jobID string) error {
	s.mu.Lock()
	job := s.jobs[jobID]
	s.mu.Unlock()

	if job == nil {
		return fmt.Errorf("Job not found.")
	}

	job.Mtx.Lock()
	defer job.Mtx.Unlock()

	if job.Status == "processing" {
		job.cancel()
		job.Status = "cancelled"
	}
	return nil
}

func (s *Service) processJob(ctx context.Context, job *Job, modelID string, pdfURL string, totalPages int, ocrPages []PageImage) {
	// Read OCR prompt
	cfg := config.Get()
	ocrPrompt, err := prompt.ReadOCRPrompt(cfg.DataDir)
	if err != nil {
		log.Printf("OCR: could not read OCR prompt: %v", err)
		ocrPrompt = prompt.DefaultOCRPrompt
	}

	// Build a map of page number → image URL for quick lookup
	ocrPageMap := make(map[int]string)
	for _, p := range ocrPages {
		ocrPageMap[p.PageNum] = p.ImageURL
	}

	// Resolve PDF file path for text extraction of non-OCR pages
	pdfFilename := strings.TrimPrefix(pdfURL, "/api/uploads/")
	pdfPath := filepath.Join(s.dataDir, "uploads", pdfFilename)

	// Process all pages
	pageTexts := make([]string, totalPages)
	var failedPages []int

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		imgURL, isOCRPage := ocrPageMap[pageNum]
		if isOCRPage {
			// OCR this page with retries
			pageText, ocrErr := s.processPageWithRetries(ctx, modelID, ocrPrompt, imgURL, pageNum)
			if ocrErr != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("OCR: permanently failed page %d after %d retries: %v", pageNum, maxRetries, ocrErr)
				failedPages = append(failedPages, pageNum)
				// Fall back to text extraction for this page
				fallbackText, _ := document.ExtractPDFPage(pdfPath, pageNum)
				if strings.TrimSpace(fallbackText) != "" {
					pageTexts[pageNum-1] = fallbackText + "\n\n> *Note: OCR failed for this page. Text was extracted using standard text extraction.*"
				} else {
					pageTexts[pageNum-1] = fmt.Sprintf("*OCR failed for page %d after %d attempts. No text could be extracted.*", pageNum, maxRetries)
				}
			} else {
				pageTexts[pageNum-1] = pageText
			}
		} else {
			// Text extraction for non-OCR pages
			text, _ := document.ExtractPDFPage(pdfPath, pageNum)
			pageTexts[pageNum-1] = text
		}

		job.Mtx.Lock()
		job.DonePages = pageNum
		job.Mtx.Unlock()
	}

	// Assemble markdown with correct page numbers
	var sb strings.Builder
	for i, page := range pageTexts {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("## Page %d\n\n", i+1))
		trimmed := strings.TrimSpace(page)
		if trimmed == "" {
			sb.WriteString("*[Empty page]*")
		} else {
			sb.WriteString(trimmed)
		}
	}
	resultText := sb.String()

	// Save as .md file
	mdFilename := uuid.New().String() + ".md"
	mdPath := filepath.Join(s.dataDir, "uploads", mdFilename)
	if err := os.MkdirAll(filepath.Dir(mdPath), 0755); err != nil {
		log.Printf("OCR: failed to create uploads dir: %v", err)
	}
	if err := os.WriteFile(mdPath, []byte(resultText), 0644); err != nil {
		log.Printf("OCR: failed to save result: %v", err)
	}

	job.Mtx.Lock()
	job.Status = "complete"
	job.ResultText = resultText
	job.ResultURL = fmt.Sprintf("/api/uploads/%s", mdFilename)
	job.FailedPages = failedPages
	if len(failedPages) > 0 {
		job.Error = fmt.Sprintf("OCR failed for page(s) %s. Text extraction was used as fallback.", formatPageList(failedPages))
	}
	job.Mtx.Unlock()

	// Clean up temporary page images
	for _, p := range ocrPages {
		filename := strings.TrimPrefix(p.ImageURL, "/api/uploads/")
		imgPath := filepath.Join(s.dataDir, "uploads", filename)
		os.Remove(imgPath)
	}
}

// processPageWithRetries attempts OCR on a single page up to maxRetries times.
func (s *Service) processPageWithRetries(ctx context.Context, modelID, ocrPrompt, imgURL string, pageNum int) (string, error) {
	messages := []llm.ChatMessage{
		{Role: "system", Content: ocrPrompt},
		{Role: "user", Content: "Extract the text from this page.", Images: []string{imgURL}},
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		var pageText string
		var processErr error

		if llm.IsExternalModel(modelID) {
			pageText, processErr = s.processExternal(ctx, modelID, messages)
		} else {
			pageText, processErr = s.processLocal(ctx, modelID, messages)
		}

		if processErr == nil && strings.TrimSpace(pageText) != "" {
			return pageText, nil
		}

		lastErr = processErr
		if lastErr == nil {
			lastErr = fmt.Errorf("empty response from OCR model")
		}

		if attempt < maxRetries {
			log.Printf("OCR: page %d attempt %d/%d failed: %v — retrying in 3s", pageNum, attempt, maxRetries, lastErr)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}

	return "", lastErr
}

// processExternal calls an external model's ChatStream directly (no queue).
func (s *Service) processExternal(ctx context.Context, modelID string, messages []llm.ChatMessage) (string, error) {
	engine := s.manager.GetEngine(modelID)
	if engine == nil {
		return "", fmt.Errorf("OCR model %q not available", modelID)
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
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

// processLocal enqueues the request with OCR priority and collects the response.
func (s *Service) processLocal(ctx context.Context, modelID string, messages []llm.ChatMessage) (string, error) {
	if err := s.manager.LoadModel(modelID); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	item, _ := s.queue.EnqueueWithPriority(ctx, modelID, messages, queue.PriorityOCR)

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

// formatPageList formats a list of page numbers for display, e.g. "1, 3, 5".
func formatPageList(pages []int) string {
	strs := make([]string, len(pages))
	for i, p := range pages {
		strs[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(strs, ", ")
}
