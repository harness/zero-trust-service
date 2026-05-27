package zts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/requestctx"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/instrumented"
)

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[verify] failed to read request body: %v", err)
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.VerifyRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[verify] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	tracker := instrumented.NewTracker()
	ctx := instrumented.WithTracker(r.Context(), tracker)
	ctx = requestctx.WithRawPayload(ctx, rawBody)

	response, verifyErr := s.verifyHandler(ctx, request)
	if verifyErr != nil {
		http.Error(w, verifyErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DefaultVerifyHandler allows every request through. It exists so the
// server can boot without any handler configured.
func DefaultVerifyHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{Allowed: true}, nil
}

func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}
