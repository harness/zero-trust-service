package file

import "time"

// Config holds file-based audit writer settings.
type Config struct {
	Dir        string
	MaxAgeDays int
}

// ListRequest defines the filters for listing audit records.
type ListRequest struct {
	Kind      string // "verify" (default) or "output"
	FromTime  time.Time
	ToTime    time.Time
	AccountID string
	TaskType  string
	TaskID    string
	Allowed   *bool
	Limit     int
	Offset    int
}

// ListResponse is the paginated result of a list query.
type ListResponse struct {
	Kind   string `json:"kind"`
	Audits any    `json:"audits"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
