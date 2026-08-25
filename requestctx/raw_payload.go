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

// Package requestctx holds typed accessors for values that flow through
// context.Context between the HTTP handler and downstream middleware /
// verify handlers. Centralizing them keeps context keys unexported and
// gives one obvious place to add new request-scoped values.
package requestctx

import "context"

type rawPayloadKey struct{}

// WithRawPayload stashes the raw request body in the context. The HTTP
// handler calls this before parsing so that downstream middleware (like
// audit) can record the exact bytes received from the client.
func WithRawPayload(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, rawPayloadKey{}, body)
}

// RawPayloadFrom retrieves the raw request body stashed by WithRawPayload,
// or nil if none was set.
func RawPayloadFrom(ctx context.Context) []byte {
	b, _ := ctx.Value(rawPayloadKey{}).([]byte)
	return b
}
