package server

import (
	"encoding/json"
	"net/http"

	"github.com/fllint/fllint/internal/ocr"
	"github.com/go-chi/chi/v5"
)

func (s *Server) startOCR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PDFURL     string          `json:"pdf_url"`
		TotalPages int             `json:"total_pages"`
		OCRPages   []ocr.PageImage `json:"ocr_pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if req.TotalPages == 0 {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Total page count is required.")
		return
	}
	if len(req.OCRPages) == 0 {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "No OCR pages provided.")
		return
	}

	jobID, err := s.ocrService.StartOCR(req.PDFURL, req.TotalPages, req.OCRPages)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "ocr_error", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"job_id": jobID})
}

func (s *Server) ocrStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job := s.ocrService.GetJob(id)
	if job == nil {
		respondErrorJSON(w, http.StatusNotFound, "not_found", "OCR job not found.")
		return
	}

	job.Mtx.Lock()
	resp := map[string]any{
		"id":           job.ID,
		"status":       job.Status,
		"total_pages":  job.TotalPages,
		"done_pages":   job.DonePages,
		"failed_pages": job.FailedPages,
		"error":        job.Error,
		"result_text":  job.ResultText,
		"result_url":   job.ResultURL,
	}
	job.Mtx.Unlock()

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) cancelOCR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "bad_request", "Invalid request body.")
		return
	}
	if err := s.ocrService.CancelJob(req.JobID); err != nil {
		respondErrorJSON(w, http.StatusNotFound, "ocr_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
