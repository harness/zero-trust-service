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
)

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[output] failed to read request body: %v", err)
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var request types.OutputRequest
	if err := json.Unmarshal(rawBody, &request); err != nil {
		log.Printf("[output] failed to deserialize request: %v | size=%d payload=%s",
			err, len(rawBody), truncate(rawBody, 512))
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	ctx := requestctx.WithRawPayload(r.Context(), rawBody)

	response, outErr := s.outputHandler(ctx, request)
	if outErr != nil {
		http.Error(w, outErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[output] failed to encode response: %v", err)
	}
}

// DefaultOutputHandler is a no-op that returns an empty OutputResponse.
func DefaultOutputHandler(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
	return types.OutputResponse{}, nil
}
