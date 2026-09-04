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

package zts

import (
	"context"
	"time"

	"github.com/harness/zero-trust-service/types"
)

// VerifyMiddleware wraps a VerifyHandler to add cross-cutting behavior
// (logging, metrics, audit, rate-limiting, etc.). Middlewares compose like
// chi: the first middleware passed to WithVerifyMiddleware is outermost
// (sees the request first, response last).
type VerifyMiddleware func(next types.VerifyHandler) types.VerifyHandler

// OutputMiddleware wraps an OutputHandler. Same composition rules as
// VerifyMiddleware.
type OutputMiddleware func(next types.OutputHandler) types.OutputHandler

type options struct {
	Port             int
	verifyHandler    types.VerifyHandler
	outputHandler    types.OutputHandler
	verifyMiddleware []VerifyMiddleware
	outputMiddleware []OutputMiddleware
}

func resolveOptions(opts ...Option) options {
	o := options{
		Port:          8080,
		verifyHandler: DefaultVerifyHandler,
		outputHandler: DefaultOutputHandler,
	}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// composedVerifyHandler returns the verify handler with all middlewares
// applied. The first middleware in the slice is outermost. responseTiming
// is always applied as the outermost wrap so that startTs/endTs reflect
// the full server-side latency observed by the client.
func (o options) composedVerifyHandler() types.VerifyHandler {
	h := o.verifyHandler
	for i := len(o.verifyMiddleware) - 1; i >= 0; i-- {
		h = o.verifyMiddleware[i](h)
	}
	return responseTiming(h)
}

// responseTiming stuffs startTs and endTs (UTC unix-millis) into the
// response Metadata. The system relies on these timestamps, so the SDK
// applies this wrap automatically — customers never list it.
func responseTiming(next types.VerifyHandler) types.VerifyHandler {
	return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		end := time.Now()
		if err != nil {
			return resp, err
		}
		if resp.Metadata == nil {
			resp.Metadata = make(map[string]interface{})
		}
		resp.Metadata["startTs"] = start.UTC().UnixMilli()
		resp.Metadata["endTs"] = end.UTC().UnixMilli()
		return resp, nil
	}
}

// composedOutputHandler returns the output handler with all middlewares
// applied. The first middleware in the slice is outermost.
func (o options) composedOutputHandler() types.OutputHandler {
	h := o.outputHandler
	for i := len(o.outputMiddleware) - 1; i >= 0; i-- {
		h = o.outputMiddleware[i](h)
	}
	return h
}

type Option func(*options)

func WithPort(port int) Option {
	if port <= 0 {
		panic("port must be greater than 0")
	}
	return func(o *options) { o.Port = port }
}

func WithVerifyHandler(handler types.VerifyHandler) Option {
	if handler == nil {
		panic("verify handler must not be nil")
	}
	return func(o *options) { o.verifyHandler = handler }
}

func WithOutputHandler(handler types.OutputHandler) Option {
	if handler == nil {
		panic("output handler must not be nil")
	}
	return func(o *options) { o.outputHandler = handler }
}

// WithVerifyMiddleware appends middlewares to the verify handler chain.
// Middlewares run outermost-first: the first middleware sees the request
// before all others, and the response last.
func WithVerifyMiddleware(mws ...VerifyMiddleware) Option {
	for _, mw := range mws {
		if mw == nil {
			panic("verify middleware must not be nil")
		}
	}
	return func(o *options) { o.verifyMiddleware = append(o.verifyMiddleware, mws...) }
}

// WithOutputMiddleware appends middlewares to the output handler chain.
// Middlewares run outermost-first.
func WithOutputMiddleware(mws ...OutputMiddleware) Option {
	for _, mw := range mws {
		if mw == nil {
			panic("output middleware must not be nil")
		}
	}
	return func(o *options) { o.outputMiddleware = append(o.outputMiddleware, mws...) }
}

