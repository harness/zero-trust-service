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
	"errors"
	"fmt"
	"log"
	"strings"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/pipeline"
)

// Config configures the reference execution-ordering verifier.
type Config struct {
	// DenyOnMissing denies the task when the target step's ancestor join is not
	// satisfied. Defaults to true (fail-closed). Set false to only observe/log.
	DenyOnMissing *bool `yaml:"deny_on_missing"`
	// MatrixNaming must match the engine's matrix instance-naming: "value" for
	// axis-value names (_us_free), anything else for the index default (_0_0).
	MatrixNaming string `yaml:"matrix_naming"`
	// Conditions optionally evaluates the `when` guards the SDK surfaces on a
	// step/stage. Nil (the default) enforces only ordering; when set, a guard that
	// does not hold makes the step ineligible.
	Conditions *ConditionChecker
	// ConditionOverrides forces a run/skip decision for a raw `when` expression,
	// keyed by the exact expression string (e.g. {`<+pipeline.variables.name> ==
	// "override"`: false}). Matched before any evaluator, so it needs none. A
	// non-empty map here synthesizes a checker when Conditions is nil, so `when`
	// guards can be driven purely from config. A guard that still can't be
	// evaluated fails closed (the step is denied) — there is no fail-open opt-out.
	ConditionOverrides map[string]bool `yaml:"condition_overrides"`
}

// New builds the reference execution-ordering verifier: an example of wiring
// the SDK's static ordering check (pipeline.VerifyStepOrder) into the verifier
// chain, backed by the caller-owned run-state Store. The SDK owns the graph and
// AND/OR join semantics; this verifier owns only the per-planExecutionId state
// and the deny policy. The condition callback defaults to pipeline.AlwaysTrue
// (ordering only); set cfg.Conditions to enforce when-guards too.
func New(cfg Config) (verifier.Interface, error) {
	denyOnMissing := cfg.DenyOnMissing == nil || *cfg.DenyOnMissing
	store := NewStore()

	var buildOpts []pipeline.BuildOption

	// Use a code-wired checker if supplied, else synthesize one from config-supplied
	// condition_overrides. The SDK steps over an ancestor only when its guard is
	// provably skipped (known and not held); an unresolvable guard is never stepped
	// over. Without a checker that walk-up is a no-op (AlwaysTrue skips nothing).
	checker := cfg.Conditions
	if checker == nil && len(cfg.ConditionOverrides) > 0 {
		checker = NewConditionChecker(nil, nil) // config-only: overrides, no evaluator
	}
	if checker != nil {
		checker.MergeOverrides(cfg.ConditionOverrides)
	}

	// Matrix naming comes from trusted config, not the request. "value" selects
	// axis-value names; anything else is the index default.
	if strings.EqualFold(strings.TrimSpace(cfg.MatrixNaming), "value") {
		buildOpts = append(buildOpts, pipeline.WithMatrixNaming(pipeline.MatrixNamingValue))
	}

	return verifier.From(func(ctx context.Context, request types.VerifyRequest) error {
		pkg := request.TaskPackage
		if pkg == nil || pkg.ZTSMetadata == nil || pkg.ZTSMetadata.StepFQN == "" {
			return nil // not a pipeline step task: nothing to order
		}
		zts := pkg.ZTSMetadata
		if zts.ExecutionDetails == nil || zts.ExecutionDetails.PipelineExecutionID == "" {
			return nil // no execution context to key run-state on
		}
		rp := verifier.ResolvedPipelineFrom(ctx)
		if rp == nil || rp.ResolvedYAML == "" {
			return nil // no resolved pipeline available
		}

		planExecID := zts.ExecutionDetails.PipelineExecutionID
		stepFQN := zts.StepFQN

		// The SDK's ran/cond callbacks are execution-scoped by closing over this
		// request's planExecID; the store and checker remain keyed by execution.
		//
		// Reference policy: this verifier fails open on exactly one gap — a
		// pipeline-rollback entry runs in a separate uncorrelated plan whose forward
		// gate can't be evaluated. Everything else stays fail-closed, including a
		// runtime <+input> fan-out ancestor: its instance set is never knowable from
		// the YAML, so its fan-in can never be verified complete and a follower is
		// denied (permanently — there is nothing to wait for). Malformed strategies are
		// likewise denied. This decision lives HERE, in the consumer: the SDK never
		// fails open on its own (an omitted policy fails closed), so the single policy
		// below is this verifier consciously accepting one specific gap.
		evalOpts := []pipeline.EvaluateOption{
			pipeline.WithRan(func(fqn string) (bool, error) { return store.Ran(planExecID, fqn), nil }),
			pipeline.WithPipelineRollbackPolicy(func(*pipeline.PipelineRollbackViolation) bool {
				return true // separate uncorrelated plan: forward gate not evaluable
			}),
		}
		if checker != nil {
			evalOpts = append(evalOpts, pipeline.WithCondition(
				func(w pipeline.When) (bool, error) { return checker.Eval(planExecID, w) },
			))
		}

		verdict, err := pipeline.VerifyStepOrder(
			rp.ResolvedYAML,
			stepFQN,
			buildOpts,
			evalOpts...,
		)

		if errors.Is(err, pipeline.ErrNodeNotFound) {
			// Unknown FQN: do not block a target that cannot be reasoned about.
			log.Printf("[ordering] step not found fqn=%s execution=%s; skipping", stepFQN, planExecID)
			return nil
		}
		if err != nil {
			// Unparseable / rootless pipeline (ErrInvalidPipeline): the ordering graph
			// can't be trusted. Fail closed when enforcing, else log and pass.
			log.Printf("[ordering] cannot build ordering graph fqn=%s execution=%s err=%v", stepFQN, planExecID, err)
			if denyOnMissing {
				return fmt.Errorf("execution_ordering: cannot verify step %q: %w", humanFQN(stepFQN), err)
			}
			return nil
		}

		// Surface the fail-open caveats (ignored violations) so they stay observable
		// even though they no longer block.
		for _, v := range verdict.Violations {
			if v.IsIgnored() {
				log.Printf("[ordering] fail-open fqn=%s execution=%s caveat=%q", stepFQN, planExecID, v.Reason())
			}
		}

		if !verdict.Allowed {
			// Render only the violations that actually block (not the ones a policy
			// let through); the SDK exposes no projection helper, so filter here.
			var blocking []pipeline.Violation
			for _, v := range verdict.Violations {
				if !v.IsIgnored() {
					blocking = append(blocking, v)
				}
			}
			msgs := blockingMessages(blocking)
			log.Printf("[ordering] denied fqn=%s execution=%s join=%s reasons=%q", stepFQN, planExecID, verdict.Join, msgs)
			if denyOnMissing {
				return fmt.Errorf("execution_ordering: step %q must not run: %s", humanFQN(stepFQN), strings.Join(msgs, "; "))
			}
			return nil // observe-only: do not record a denied step as run
		}

		ancestorFQNs := make([]string, len(verdict.Ancestors))
		for i, a := range verdict.Ancestors {
			ancestorFQNs[i] = a.FQN
		}
		log.Printf("[ordering] allowed fqn=%s execution=%s join=%s ancestors=%v", stepFQN, planExecID, verdict.Join, ancestorFQNs)
		// verdict.FQN is the SDK-resolved logical node; record both so instance
		// and logical gates match. Fail-open passes (rollback entry, runtime-input)
		// are allowed too, so later co-planned steps still gate on them in order.
		store.MarkRan(planExecID, stepFQN, verdict.FQN)
		return nil
	}), nil
}

// blockingMessages renders the non-ignored violations for logs and the deny
// error, humanizing FQNs. The type switch over the sealed Violation set is where
// a new violation kind would slot in.
func blockingMessages(vs []pipeline.Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		switch {
		case pipeline.IsAncestorDidNotRunViolation(v):
			m := pipeline.AsAncestorDidNotRunViolation(v)
			// For a fan-in ancestor with several outstanding instances, name the
			// logical node too; otherwise just list the outstanding steps.
			if m.AncestorFQN != "" && len(m.Ancestors) > 1 {
				out = append(out, fmt.Sprintf("waiting on %s: %d instances outstanding (%s)",
					humanFQN(m.AncestorFQN), len(m.Ancestors), humanFQNs(m.Ancestors)))
			} else {
				out = append(out, fmt.Sprintf("waiting on: %s", humanFQNs(m.Ancestors)))
			}
		case pipeline.IsConditionViolation(v):
			out = append(out, v.Reason())
		case pipeline.IsRuntimeFanoutViolation(v):
			r := pipeline.AsRuntimeFanoutViolation(v)
			// A dependency's fan-out blocks the follower; the step's own fan-out blocks
			// itself. Name which so the message points at the right node.
			node := "runtime fan-out"
			if r.Role == pipeline.FanoutAncestor {
				node = "ancestor runtime fan-out"
			}
			out = append(out, fmt.Sprintf("%s cannot be gated (%s): %s", node, r.Status, humanFQN(r.FQN)))
		default:
			out = append(out, v.Reason())
		}
	}
	return out
}

// fqnKeywords are the structural FQN segments dropped when rendering a step
// reference for humans (they carry no identity, only pipeline structure).
var fqnKeywords = map[string]bool{
	"pipeline": true, "stages": true, "spec": true, "execution": true,
	"steps": true, "rollbackSteps": true, "parallel": true,
}

// humanFQN renders a step FQN as a readable "stage / group / step" path for
// error messages, e.g. "pipeline.stages.st1.spec.execution.steps.sg1.steps.s2"
// -> "st1 / sg1 / s2". Falls back to the raw FQN if nothing is left.
func humanFQN(fqn string) string {
	var parts []string
	for _, seg := range strings.Split(fqn, ".") {
		if seg != "" && !fqnKeywords[seg] {
			parts = append(parts, seg)
		}
	}
	if len(parts) == 0 {
		return fqn
	}
	return strings.Join(parts, " / ")
}

// humanFQNs humanizes and comma-joins a list of FQNs.
func humanFQNs(fqns []string) string {
	out := make([]string, len(fqns))
	for i, f := range fqns {
		out[i] = humanFQN(f)
	}
	return strings.Join(out, ", ")
}
