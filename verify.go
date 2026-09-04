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

package zts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/harness/zero-trust-service/requestctx"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier/instrumented"
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[verify] failed to encode response: %v", err)
	}
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
