// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auditreader

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	auditfile "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit/file"
	"github.com/go-chi/chi/v5"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

type Handler struct {
	reader *auditfile.Reader
}

func NewHandler(reader *auditfile.Reader) *Handler {
	return &Handler{reader: reader}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/audits", h.handleList)
	r.Get("/audits/{id}/payload", h.handleGetPayload)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	fromStr := q.Get("from")
	if fromStr == "" {
		http.Error(w, `"from" query parameter is required (epoch millis)`, http.StatusBadRequest)
		return
	}

	fromMs, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid \"from\" value: %v", err), http.StatusBadRequest)
		return
	}

	toMs := time.Now().UnixMilli()
	if toStr := q.Get("to"); toStr != "" {
		toMs, err = strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid \"to\" value: %v", err), http.StatusBadRequest)
			return
		}
	}

	if toMs <= fromMs {
		http.Error(w, `"to" must be after "from"`, http.StatusBadRequest)
		return
	}

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

	kind := q.Get("kind")
	if kind == "" {
		kind = audit.EventVerify
	}

	req := auditfile.ListRequest{
		Kind:      kind,
		FromTime:  time.UnixMilli(fromMs).UTC(),
		ToTime:    time.UnixMilli(toMs).UTC(),
		AccountID: q.Get("account_id"),
		TaskType:  q.Get("task_type"),
		TaskID:    q.Get("task_id"),
		Limit:     limit,
		Offset:    offset,
	}

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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[auditreader] failed to encode response: %v", err)
	}
}

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
	if _, err := w.Write(payload); err != nil {
		log.Printf("[auditreader] failed to write payload: %v", err)
	}
}
