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
