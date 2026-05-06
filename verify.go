package zts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/instrumented"
	"github.com/google/uuid"
)

const (
	metricVerifyRequestsTotal   = "zts_verify_requests_total"
	metricVerifyRequestDuration = "zts_verify_request_duration_seconds"
	metricMissingMetadataTotal  = "zts_missing_metadata_total"

	statusAuthorized   = "authorized"
	statusUnauthorized = "unauthorized"
	statusSuccess      = "success"
	statusError        = "error"

	fieldZTSMetadata = "zts_metadata"
	fieldAccountID   = "account_id"
	fieldTaskType    = "task_type"

	keyStatus    = "status"
	keyAccountID = "account_id"
	keyField     = "field"
)

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	m := s.metrics

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[verify] failed to read request body: %v", err)
		m.Counter(metricVerifyRequestsTotal, 1, metrics.Dim(keyStatus, statusError), metrics.Dim(keyAccountID, ""))
		m.Histogram(metricVerifyRequestDuration, time.Since(start).Seconds(), metrics.Dim(keyStatus, statusError))
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.VerifyRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[verify] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		m.Counter(metricVerifyRequestsTotal, 1, metrics.Dim(keyStatus, statusError), metrics.Dim(keyAccountID, ""))
		m.Histogram(metricVerifyRequestDuration, time.Since(start).Seconds(), metrics.Dim(keyStatus, statusError))
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	tracker := instrumented.NewTracker()
	ctx := instrumented.WithTracker(r.Context(), tracker)

	recordMissingMetadata(request, m)

	accountID := request.ResolveAccountID()

	taskID := ""
	if request.TaskPackage != nil {
		taskID = request.TaskPackage.TaskID
	}
	log.Printf("[verify] processing request task_id=%s account_id=%s task_type=%s",
		taskID, accountID, request.ResolveTaskType())

	response, verifyErr := s.verifyHandler(ctx, request)
	end := time.Now()
	duration := end.Sub(start)

	if verifyErr != nil {
		log.Printf("[verify] internal error task_id=%s account_id=%s duration=%s error=%v",
			taskID, accountID, duration, verifyErr)
		m.Counter(metricVerifyRequestsTotal, 1, metrics.Dim(keyStatus, statusError), metrics.Dim(keyAccountID, accountID))
		m.Histogram(metricVerifyRequestDuration, duration.Seconds(), metrics.Dim(keyStatus, statusError))

		s.writeAudit(start, end, request, tracker, false, verifyErr.Error(), json.RawMessage(rawBody))

		http.Error(w, verifyErr.Error(), http.StatusInternalServerError)
		return
	}

	status := statusAuthorized
	if !response.Allowed {
		status = statusUnauthorized
		log.Printf("[verify] denied task_id=%s account_id=%s duration=%s reason=%s",
			taskID, accountID, duration, response.Reason)
	} else {
		log.Printf("[verify] authorized task_id=%s account_id=%s duration=%s",
			taskID, accountID, duration)
	}
	m.Counter(metricVerifyRequestsTotal, 1, metrics.Dim(keyStatus, status), metrics.Dim(keyAccountID, accountID))
	m.Histogram(metricVerifyRequestDuration, duration.Seconds(), metrics.Dim(keyStatus, status))

	s.writeAudit(start, end, request, tracker, response.Allowed, response.Reason, json.RawMessage(rawBody))

	if response.Metadata == nil {
		response.Metadata = make(map[string]interface{})
	}
	response.Metadata["startTs"] = start.UTC().UnixMilli()
	response.Metadata["endTs"] = end.UTC().UnixMilli()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) writeAudit(
	start time.Time,
	end time.Time,
	request types.VerifyRequest,
	tracker *instrumented.Tracker,
	allowed bool,
	reason string,
	rawPayload json.RawMessage,
) {
	if s.auditWriter == nil {
		return
	}

	taskType := request.ResolveTaskType()

	var validatorsRun []string
	failedValidator := ""
	if tracker != nil {
		validatorsRun, failedValidator = tracker.Results()
	}

	taskID := ""
	delegateID := ""
	delegateInstanceID := ""
	if request.TaskPackage != nil {
		taskID = request.TaskPackage.TaskID
		delegateID = request.TaskPackage.DelegateID
		delegateInstanceID = request.TaskPackage.DelegateInstanceID
	}

	record := audit.Record{
		ID:                 uuid.New().String(),
		StartTime:          start.UTC(),
		EndTime:            end.UTC(),
		AccountID:          request.ResolveAccountID(),
		TaskID:             taskID,
		TaskType:           taskType,
		DelegateID:         delegateID,
		DelegateInstanceID: delegateInstanceID,
		Allowed:            allowed,
		Reason:             reason,
		FailedValidator:    failedValidator,
		Duration:           end.Sub(start),
		ValidatorsRun:      validatorsRun,
	}

	go s.auditWriter.WriteEvent(audit.EventVerify, record, rawPayload)
}

func recordMissingMetadata(req types.VerifyRequest, m metrics.Emitter) {
	if req.TaskPackage == nil {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldZTSMetadata))
		return
	}
	if req.TaskPackage.ZTSMetadata == nil {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldZTSMetadata))
		return
	}
	if req.ResolveAccountID() == "" {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldAccountID))
	}
	if req.ResolveTaskType() == "" {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldTaskType))
	}
}

func DefaultVerifyHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{Allowed: true}, nil
}

func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}
