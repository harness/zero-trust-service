package audit

import "time"

// Record is the lightweight metadata entry written for every verify request.
// It contains only searchable metadata — no secrets.
type Record struct {
	ID                 string   `json:"id"`
	StartTs            int64    `json:"startTs"`
	EndTs              int64    `json:"endTs"`
	AccountID          string   `json:"accountId"`
	TaskID             string   `json:"taskId"`
	TaskType           string   `json:"taskType"`
	DelegateID         string   `json:"delegateId,omitempty"`
	DelegateInstanceID string   `json:"delegateInstanceId,omitempty"`
	Allowed            bool     `json:"allowed"`
	FailedValidator    string   `json:"failedValidator,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	DurationMs         int64    `json:"durationMs"`
	ValidatorsRun      []string `json:"validatorsRun,omitempty"`
}

// ListRequest defines the filters for listing audit records.
type ListRequest struct {
	FromMs    int64  // required — epoch millis
	ToMs      int64  // required — epoch millis
	AccountID string // optional
	TaskType  string // optional
	TaskID    string // optional
	Allowed   *bool  // optional: filter by allowed true/false
	Limit     int    // default 100, max 500
	Offset    int    // pagination offset
}

// ListResponse is the paginated result of a list query.
type ListResponse struct {
	Audits []Record `json:"audits"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

// DateFromEpochMs converts epoch millis to a UTC date string (YYYY-MM-DD).
func DateFromEpochMs(epochMs int64) string {
	return time.UnixMilli(epochMs).UTC().Format("2006-01-02")
}
