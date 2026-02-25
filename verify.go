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
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"github.com/google/uuid"
)

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	m := s.metrics

	// Read the raw body so we can store it for audit and also decode it
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[verify] failed to read request body: %v", err)
		m.VerifyRequestsTotal.WithLabelValues(metrics.LabelStatusError, "").Inc()
		m.VerifyRequestDuration.WithLabelValues(metrics.LabelStatusError).Observe(time.Since(start).Seconds())
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.VerifyRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[verify] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		m.VerifyRequestsTotal.WithLabelValues(metrics.LabelStatusError, "").Inc()
		m.VerifyRequestDuration.WithLabelValues(metrics.LabelStatusError).Observe(time.Since(start).Seconds())
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	// Create a tracker to collect validator names for audit, carried via context
	tracker := verifier.NewTracker()
	ctx := verifier.WithTracker(r.Context(), tracker)

	// track missing metadata fields
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
		m.VerifyRequestsTotal.WithLabelValues(metrics.LabelStatusError, accountID).Inc()
		m.VerifyRequestDuration.WithLabelValues(metrics.LabelStatusError).Observe(duration.Seconds())

		s.writeAudit(start, end, request, tracker, false, verifyErr.Error(), json.RawMessage(rawBody))

		http.Error(w, verifyErr.Error(), http.StatusInternalServerError)
		return
	}

	status := metrics.LabelStatusAuthorized
	if !response.Allowed {
		status = metrics.LabelStatusUnauthorized
		log.Printf("[verify] denied task_id=%s account_id=%s duration=%s reason=%s",
			taskID, accountID, duration, response.Reason)
	} else {
		log.Printf("[verify] authorized task_id=%s account_id=%s duration=%s",
			taskID, accountID, duration)
	}
	m.VerifyRequestsTotal.WithLabelValues(status, accountID).Inc()
	m.VerifyRequestDuration.WithLabelValues(status).Observe(duration.Seconds())

	s.writeAudit(start, end, request, tracker, response.Allowed, response.Reason, json.RawMessage(rawBody))

	// Attach timing metadata to the response
	if response.Metadata == nil {
		response.Metadata = make(map[string]interface{})
	}
	response.Metadata["startTs"] = start.UTC().UnixMilli()
	response.Metadata["endTs"] = end.UTC().UnixMilli()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// writeAudit creates an audit record if the audit writer is configured.
func (s *Server) writeAudit(
	start time.Time,
	end time.Time,
	request types.VerifyRequest,
	tracker *verifier.Tracker,
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
		StartTs:            start.UTC().UnixMilli(),
		EndTs:              end.UTC().UnixMilli(),
		AccountID:          request.ResolveAccountID(),
		TaskID:             taskID,
		TaskType:           taskType,
		DelegateID:         delegateID,
		DelegateInstanceID: delegateInstanceID,
		Allowed:            allowed,
		Reason:             reason,
		FailedValidator:    failedValidator,
		DurationMs:         end.Sub(start).Milliseconds(),
		ValidatorsRun:      validatorsRun,
	}

	// Write asynchronously so it doesn't block the response
	go s.auditWriter.Write(record, rawPayload)
}

func recordMissingMetadata(req types.VerifyRequest, m *metrics.Metrics) {
	if req.TaskPackage == nil {
		m.MissingMetadataTotal.WithLabelValues(metrics.LabelFieldZTSMetadata).Inc()
		return
	}
	if req.TaskPackage.ZTSMetadata == nil {
		m.MissingMetadataTotal.WithLabelValues(metrics.LabelFieldZTSMetadata).Inc()
		return
	}
	if req.ResolveAccountID() == "" {
		m.MissingMetadataTotal.WithLabelValues(metrics.LabelFieldAccountID).Inc()
	}
	if req.ResolveTaskType() == "" {
		m.MissingMetadataTotal.WithLabelValues(metrics.LabelFieldTaskType).Inc()
	}
}

func DefaultVerifyHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{Allowed: true}, nil
}

// truncate returns the first n bytes of data as a string, appending "..." if truncated.
func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}
