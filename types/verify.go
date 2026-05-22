package types

import "context"

// VerifyRequest is the top-level request sent by the delegate service.
// It wraps a DelegateTaskPackage inside the "taskPackage" field, matching
// the Java ZtsVerificationRequest DTO.
type VerifyRequest struct {
	TaskPackage    *DelegateTaskPackage `json:"taskPackage"`
	DecodedPayload *DecodedPayload      `json:"decodedPayload,omitempty"`
}

// DecodedPayload is a view of opaque task payloads inside the task package.
// Harness keeps the original taskPackage unchanged and attaches this when a
// delegate-side decoder can safely expose task-specific data.
type DecodedPayload struct {
	Kind    string                 `json:"kind,omitempty"`
	Source  string                 `json:"source,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// VerifyResponse is returned to the delegate service.
// Fields match the Java ZtsVerificationResponse DTO:
//   - allowed:  true if the task is permitted, false if denied
//   - reason:   human-readable explanation (present when denied)
//   - metadata: optional key-value pairs for additional context
type VerifyResponse struct {
	Allowed  bool                   `json:"allowed"`
	Reason   string                 `json:"reason,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VerifyHandler is the function signature for handling verify requests.
// The context carries request-scoped data such as the validator tracker.
type VerifyHandler func(ctx context.Context, request VerifyRequest) (VerifyResponse, error)
