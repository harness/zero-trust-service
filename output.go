package zts

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"github.com/google/uuid"
)

const (
	metricOutputRequestsTotal = "zts_output_requests_total"
)

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	m := s.metrics

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[output] failed to read request body: %v", err)
		m.Counter(metricOutputRequestsTotal, 1, metrics.Dim(keyStatus, statusError), metrics.Dim(keyAccountID, ""))
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.OutputRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[output] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		m.Counter(metricOutputRequestsTotal, 1, metrics.Dim(keyStatus, statusError), metrics.Dim(keyAccountID, ""))
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	accountID := ""
	taskTypeName := ""
	responseCode := ""
	if request.TaskResponse != nil {
		accountID = request.TaskResponse.AccountID
		taskTypeName = request.TaskResponse.TaskTypeName
		responseCode = request.TaskResponse.ResponseCode
	}

	taskID := request.TaskID

	log.Printf("[output] received task output account_id=%s task_id=%s task_type=%s response_code=%s",
		accountID, taskID, taskTypeName, responseCode)

	m.Counter(metricOutputRequestsTotal, 1, metrics.Dim(keyStatus, statusSuccess), metrics.Dim(keyAccountID, accountID))

	if s.auditWriter != nil {
		record := audit.OutputRecord{
			ID:           uuid.New().String(),
			Timestamp:    time.Now().UTC().UnixMilli(),
			AccountID:    accountID,
			TaskID:       taskID,
			TaskTypeName: taskTypeName,
			ResponseCode: responseCode,
		}
		go s.auditWriter.WriteEvent(audit.EventOutput, record, json.RawMessage(rawBody))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types.OutputResponse{})
}
