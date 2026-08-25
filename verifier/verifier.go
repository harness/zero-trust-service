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

package verifier

import (
	"context"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

type Interface interface {
	Handle(ctx context.Context, request types.VerifyRequest) error
}

type middlewareFunc func(ctx context.Context, request types.VerifyRequest) error

func (f middlewareFunc) Handle(ctx context.Context, request types.VerifyRequest) error {
	return f(ctx, request)
}

func From(f func(ctx context.Context, request types.VerifyRequest) error) Interface {
	return middlewareFunc(f)
}

// Wrap applies middlewares to a verifier in outermost-first order:
// the first middleware in the slice sees the request before the rest.
//
//	v := verifier.Wrap(handler, mw1, mw2, mw3)
//	// request flow: mw1 → mw2 → mw3 → handler
func Wrap(v Interface, mws ...func(next Interface) Interface) Interface {
	for i := len(mws) - 1; i >= 0; i-- {
		v = mws[i](v)
	}
	return v
}
