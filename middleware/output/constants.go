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

// Package output ships the first-party zts.OutputMiddleware constructors
// used by the ZTS SDK: Logging, Metrics, and Audit. Each constructor
// returns a zts.OutputMiddleware so customers can compose them with their
// own middlewares via zts.WithOutputMiddleware.
package output

const (
	statusSuccess = "success"
	statusError   = "error"

	keyStatus    = "status"
	keyAccountID = "account_id"
)
