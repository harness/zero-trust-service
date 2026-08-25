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

// Package verify ships the first-party zts.VerifyMiddleware constructors
// used by the ZTS SDK: Logging, Metrics, Audit, and MissingMetadata. Each
// constructor returns a zts.VerifyMiddleware so customers can compose
// them with their own middlewares via zts.WithVerifyMiddleware.
package verify

const (
	statusAuthorized   = "authorized"
	statusUnauthorized = "unauthorized"
	statusError        = "error"

	keyStatus    = "status"
	keyAccountID = "account_id"
	keyField     = "field"

	fieldZTSMetadata = "zts_metadata"
	fieldAccountID   = "account_id"
	fieldTaskType    = "task_type"
)
