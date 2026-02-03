package zts

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type VerifyRequest struct {
	TaskID string `json:"task_id"`
}

type VerifyStatus string

const (
	VerifyStatusAuthorized   VerifyStatus = "authorized"
	VerifyStatusUnauthorized VerifyStatus = "unauthorized"
)

type VerifyResponse struct {
	Status VerifyStatus `json:"status"`
	Error  *string      `json:"error,omitempty"`
}

type VerifyHandler func(request VerifyRequest) (VerifyResponse, error)

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var request VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("failed to deserialize request: %v", err), http.StatusBadRequest)
		return
	}

	response, err := s.verifyHandler(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func DefaultVerifyHandler(request VerifyRequest) (VerifyResponse, error) {
	return VerifyResponse{
		Status: VerifyStatusAuthorized,
	}, nil
}
