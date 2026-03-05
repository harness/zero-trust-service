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

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[verify] failed to read request body: %v", err)
		m.VerifyRequestsTotal.Inc(metrics.LabelStatusError, "")
		m.VerifyRequestDuration.Observe(time.Since(start).Seconds(), metrics.LabelStatusError)
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.VerifyRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[verify] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		m.VerifyRequestsTotal.Inc(metrics.LabelStatusError, "")
		m.VerifyRequestDuration.Observe(time.Since(start).Seconds(), metrics.LabelStatusError)
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	tracker := verifier.NewTracker()
	ctx := verifier.WithTracker(r.Context(), tracker)
	ctx, _ = verifier.WithPipelineHolder(ctx)

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
		m.VerifyRequestsTotal.Inc(metrics.LabelStatusError, accountID)
		m.VerifyRequestDuration.Observe(duration.Seconds(), metrics.LabelStatusError)

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
	m.VerifyRequestsTotal.Inc(status, accountID)
	m.VerifyRequestDuration.Observe(duration.Seconds(), status)

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

func recordMissingMetadata(req types.VerifyRequest, m *metrics.Metrics) {
	if req.TaskPackage == nil {
		m.MissingMetadataTotal.Inc(metrics.LabelFieldZTSMetadata)
		return
	}
	if req.TaskPackage.ZTSMetadata == nil {
		m.MissingMetadataTotal.Inc(metrics.LabelFieldZTSMetadata)
		return
	}
	if req.ResolveAccountID() == "" {
		m.MissingMetadataTotal.Inc(metrics.LabelFieldAccountID)
	}
	if req.ResolveTaskType() == "" {
		m.MissingMetadataTotal.Inc(metrics.LabelFieldTaskType)
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
