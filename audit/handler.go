package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// Handler exposes HTTP endpoints for querying local audit records.
type Handler struct {
	reader *Reader
}

// NewHandler creates a new audit HTTP handler.
func NewHandler(reader *Reader) *Handler {
	return &Handler{reader: reader}
}

// RegisterRoutes registers the audit API routes on the given chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/audits", h.handleList)
	r.Get("/audits/{id}/payload", h.handleGetPayload)
}

// handleList handles GET /api/audits?from=<epochMs>&to=<epochMs>&account_id=...&...
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse required "from" and "to" parameters (epoch millis)
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if fromStr == "" || toStr == "" {
		http.Error(w, `"from" and "to" query parameters are required (epoch millis)`, http.StatusBadRequest)
		return
	}

	fromMs, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid \"from\" value: %v (expected epoch millis)", err), http.StatusBadRequest)
		return
	}
	toMs, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid \"to\" value: %v (expected epoch millis)", err), http.StatusBadRequest)
		return
	}

	if toMs <= fromMs {
		http.Error(w, `"to" must be after "from"`, http.StatusBadRequest)
		return
	}

	// Parse optional filters
	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if raw := q.Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	req := ListRequest{
		FromMs:    fromMs,
		ToMs:      toMs,
		AccountID: q.Get("account_id"),
		TaskType:  q.Get("task_type"),
		TaskID:    q.Get("task_id"),
		Limit:     limit,
		Offset:    offset,
	}

	// Parse optional "allowed" filter (true/false)
	if raw := q.Get("allowed"); raw != "" {
		switch raw {
		case "true":
			v := true
			req.Allowed = &v
		case "false":
			v := false
			req.Allowed = &v
		}
	}

	resp, err := h.reader.List(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list audits: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGetPayload handles GET /api/audits/{id}/payload
func (h *Handler) handleGetPayload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "audit id is required", http.StatusBadRequest)
		return
	}

	payload, err := h.reader.GetPayload(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("payload not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}
