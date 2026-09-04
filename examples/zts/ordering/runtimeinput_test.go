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

package ordering

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// caaReq builds a verify request for the tc29 follower step in the given execution.
func caaReq(exec string) (context.Context, types.VerifyRequest) {
	caa := "pipeline.stages.caa.spec.execution.steps.ShellScript_1"
	ctx, h := verifier.WithPipelineHolder(context.Background())
	h.Set(&resolver.ResolvedPipeline{ResolvedYAML: tc29ResolvedYAML})
	req := types.VerifyRequest{TaskPackage: &types.TaskPackage{
		ZTSMetadata: &types.ZTSMetadata{
			StepFQN:          caa,
			ExecutionDetails: &types.ExecutionDetails{PipelineExecutionID: exec},
		},
	}}
	return ctx, req
}

// TestOrderingVerifier_RuntimeInputFailsClosed: tc29's "cd" ancestor fans out on a
// runtime <+input>, so its instance set is never knowable and its fan-in can't be
// verified complete. This reference verifier supplies no runtime-input policy, so
// the SDK's fail-closed default stands: the follower is denied (permanently — there
// is nothing to wait for).
func TestOrderingVerifier_RuntimeInputFailsClosed(t *testing.T) {
	v, err := New(Config{}) // default: DenyOnMissing=true
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, req := caaReq("xpUXM15FTZazuE-ehlbCvA")

	if err := v.Handle(ctx, req); err == nil {
		t.Fatal("caa should be denied on a runtime <+input> ancestor, got allow")
	}
}

// TestOrderingVerifier_RuntimeInput_ObserveOnly: with DenyOnMissing=false the
// verifier only observes — the runtime-input denial is logged but not enforced, so
// Handle returns nil (no block) rather than failing open.
func TestOrderingVerifier_RuntimeInput_ObserveOnly(t *testing.T) {
	no := false
	v, err := New(Config{DenyOnMissing: &no})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, req := caaReq("exec-observe")
	if err := v.Handle(ctx, req); err != nil {
		t.Fatalf("observe-only should not block, got: %v", err)
	}
}
