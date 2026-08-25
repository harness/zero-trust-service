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

package audit

import (
	"encoding/json"
	"time"
)

// Event kind constants.
const (
	EventVerify = "verify"
	EventOutput = "output"
)

// AuditRecord is implemented by every audit record type.
type AuditRecord interface {
	AuditID() string
	AuditDate() string // YYYY-MM-DD in UTC
}

// Writer abstracts the persistence of audit records.
type Writer interface {
	WriteEvent(kind string, record AuditRecord, rawPayload json.RawMessage)
}

// Record is the metadata entry for a verify request.
type Record struct {
	ID                 string        `json:"id"`
	StartTime          time.Time     `json:"startTime"`
	EndTime            time.Time     `json:"endTime"`
	AccountID          string        `json:"accountId"`
	TaskID             string        `json:"taskId"`
	TaskType           string        `json:"taskType"`
	DelegateID         string        `json:"delegateId,omitempty"`
	DelegateInstanceID string        `json:"delegateInstanceId,omitempty"`
	GitOpsAgentID      string        `json:"gitOpsAgentId,omitempty"`
	Allowed            bool          `json:"allowed"`
	FailedValidator    string        `json:"failedValidator,omitempty"`
	Reason             string        `json:"reason,omitempty"`
	Error              string        `json:"error,omitempty"`
	Duration           time.Duration `json:"duration"`
	ValidatorsRun      []string      `json:"validatorsRun,omitempty"`
}

func (r Record) AuditID() string   { return r.ID }
func (r Record) AuditDate() string { return r.StartTime.UTC().Format("2006-01-02") }

// OutputRecord is the metadata entry for a task output response.
type OutputRecord struct {
	ID              string `json:"id"`
	Timestamp       int64  `json:"timestamp"`
	AccountID       string `json:"accountId"`
	TaskID          string `json:"taskId,omitempty"`
	TaskTypeName    string `json:"taskTypeName,omitempty"`
	ResponseCode    string `json:"responseCode,omitempty"`
	GitOpsAgentID   string `json:"gitOpsAgentId,omitempty"`
}

func (r OutputRecord) AuditID() string { return r.ID }
func (r OutputRecord) AuditDate() string {
	return time.UnixMilli(r.Timestamp).UTC().Format("2006-01-02")
}
