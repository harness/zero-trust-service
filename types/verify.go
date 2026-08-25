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

package types

import "context"

// VerifyRequest is the top-level request sent by the delegate service or
// the GitOps agent. Both use the same TaskPackage structure — GitOps agents
// populate the subset of fields they have (TaskID, AccountID, GitOpsAgentID,
// TaskDetails.TaskType).
type VerifyRequest struct {
	TaskPackage *TaskPackage `json:"taskPackage"`
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
