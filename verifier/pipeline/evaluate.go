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

package pipeline

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNodeNotFound is returned by EvaluateStep and VerifyStepOrder when the target
// FQN is not a node in the pipeline. It is the single signal for a missing step:
// a nil error means the returned verdict is fully populated, so callers fail closed
// with errors.Is. (The returned verdict carries only the FQN in this case.)
var ErrNodeNotFound = errors.New("pipeline: step not found")

// Ordering checks layer on the immutable DAG from BuildGraph. The two questions
// it can't answer statically — has an ancestor already run, does a when-guard
// hold — are delegated to caller-supplied functions owning runtime state. The
// caller closes over its own execution scope, so neither function is passed an
// execution id.

// StepRanFunc reports whether the step with the given FQN has completed in the
// execution being evaluated. The caller owns execution scope and closes over it,
// keeping run-state correct across concurrent executions. A non-nil error means
// the caller could not determine the run-state (a lookup failed, state was
// unavailable): the SDK cannot tell whether the step ran, so it fails closed with
// a RunStateUnknownViolation rather than assume either way.
type StepRanFunc func(fqn string) (ran bool, err error)

// ConditionFunc reports whether a when-guard currently holds. Guard truth is only
// determinable at execution time. A non-nil error means the guard could not be
// evaluated (an unresolved expression, a resolver gap): the SDK cannot tell
// whether the guarded node was required, so it fails closed with a
// ConditionUnknownViolation — distinct from a guard that evaluated and simply did
// not hold (holds=false, nil error), which is a ConditionViolation. The ancestor
// walk-up never steps over a guard it could not evaluate, so an unresolvable
// ancestor guard keeps the ancestor required.
type ConditionFunc func(w When) (holds bool, err error)

// AlwaysTrue is the default ConditionFunc: every guard holds. Use it when only
// ordering matters.
func AlwaysTrue(When) (bool, error) { return true, nil }

// EvaluateOption supplies the runtime inputs and tuning for a single ordering
// evaluation (see EvaluateStep). Distinct from BuildOption: build-time options
// shape the graph, evaluate-time options gate a step against it.
type EvaluateOption func(*evalOptions)

type evalOptions struct {
	ran  StepRanFunc
	cond ConditionFunc

	// Per-violation fail-open policies. The SDK never fails open on its own: a
	// violation blocks unless the caller supplied a policy for that type AND it
	// returns true for this occurrence. A nil policy (the default) fails closed.
	// Each policy receives the fully-populated violation so the caller can decide
	// from its structured detail (e.g. RuntimeFanoutViolation.Status,
	// ConditionUnknownViolation.Err).
	conditionPolicy        func(*ConditionViolation) bool
	conditionUnknownPolicy func(*ConditionUnknownViolation) bool
	ancestorPolicy         func(*AncestorDidNotRunViolation) bool
	runtimeFanoutPolicy    func(*RuntimeFanoutViolation) bool
	runStateUnknownPolicy  func(*RunStateUnknownViolation) bool
	pipelineRollbackPolicy func(*PipelineRollbackViolation) bool
}

// WithRan supplies the run-state callback — whether a given ancestor FQN has
// completed in the execution being evaluated. Omitted (or nil) means "nothing
// has run", so every ancestor gate is unsatisfied.
func WithRan(fn StepRanFunc) EvaluateOption {
	return func(o *evalOptions) { o.ran = fn }
}

// WithCondition supplies the when-guard callback. Omitted (or nil) defaults to
// AlwaysTrue, so only ordering is enforced.
func WithCondition(fn ConditionFunc) EvaluateOption {
	return func(o *evalOptions) { o.cond = fn }
}

// Per-violation fail-open policies. Each takes a decision function that receives
// the fully-populated violation and returns true to fail open (accept it) or
// false to fail closed (block). The SDK supplies no default policy: an omitted
// (or nil) policy fails closed, so the caller must consciously opt a specific
// occurrence into fail-open using the violation's own detail. The decision is
// per-occurrence, not per-type: the same policy may fail open on one violation and
// closed on the next.

// WithConditionPolicy sets the fail-open policy for a ConditionViolation (the
// evaluated step is defined not to run: a known-false `when` guard, or a strategy
// that excludes every matrix combination). It is invoked once per such fact, so
// the callback can inspect the guard (GuardFQN, Level, Kind, Expression/Status).
// Absent policy fails closed.
func WithConditionPolicy(fn func(*ConditionViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.conditionPolicy = fn }
}

// WithConditionUnknownPolicy sets the fail-open policy for a
// ConditionUnknownViolation (a `when` guard, on the step or an ancestor, that the
// ConditionFunc could not evaluate). Kept separate from WithConditionPolicy: an
// unmet guard is a pipeline fact, an unevaluable one is a resolver gap, so
// accepting the second must not accept the first. The callback can inspect Err
// (what the ConditionFunc returned). Absent policy fails closed.
func WithConditionUnknownPolicy(fn func(*ConditionUnknownViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.conditionUnknownPolicy = fn }
}

// WithAncestorPolicy sets the fail-open policy for an AncestorDidNotRunViolation
// (an ancestor requirement definitively not satisfied). It is invoked once per
// requirement — one per ancestor under an AND join, one for the group under OR —
// so the callback can decide per requirement. Absent policy fails closed.
func WithAncestorPolicy(fn func(*AncestorDidNotRunViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.ancestorPolicy = fn }
}

// WithRuntimeFanoutPolicy sets the fail-open policy for a RuntimeFanoutViolation
// (a node — an ancestor or the evaluated step — that fans out at runtime whose
// instance FQNs can't be enumerated, so its fan-in can't be verified complete).
// It is invoked once per offending node, so the callback can inspect that node's
// FQN and Status (runtime_input | custom_names). Absent policy fails closed.
func WithRuntimeFanoutPolicy(fn func(*RuntimeFanoutViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.runtimeFanoutPolicy = fn }
}

// WithRunStateUnknownPolicy sets the fail-open policy for a RunStateUnknownViolation
// (the StepRanFunc could not determine whether a node ran). The callback can
// inspect Err (what the StepRanFunc returned). Absent policy fails closed.
func WithRunStateUnknownPolicy(fn func(*RunStateUnknownViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.runStateUnknownPolicy = fn }
}

// WithPipelineRollbackPolicy sets the fail-open policy for a PipelineRollbackViolation
// (a PipelineRollback entry running as a separate uncorrelated execution whose
// forward gate can't be evaluated). Absent policy fails closed.
func WithPipelineRollbackPolicy(fn func(*PipelineRollbackViolation) bool) EvaluateOption {
	return func(o *evalOptions) { o.pipelineRollbackPolicy = fn }
}

// Violation is one reason a step may not run now. The set is sealed — only this
// package implements the unexported marker — so callers can exhaustively
// type-switch on the concrete *T types (or use the Is*/As* helpers). A violation
// that is not ignored blocks the step; IsIgnored reports whether a per-violation
// policy opted this occurrence into fail-open.
type Violation interface {
	// IsIgnored reports whether a per-violation fail-open policy (a With*Policy
	// option) accepted this occurrence. An ignored violation does not block the
	// step; the default, with no policy, is false (fail closed).
	IsIgnored() bool
	// Reason is a human-readable explanation, for logs and error messages.
	Reason() string
	// Err is the underlying error when the violation arose because something could
	// not be evaluated — a ConditionFunc or StepRanFunc returned an error. It is nil
	// for violations that are a plain pipeline fact (an unmet guard, an ancestor that
	// definitively did not run, a non-enumerable fan-out, a pipeline-rollback entry).
	Err() error
	isViolation()
}

// ConditionKind classifies the guard a ConditionViolation or ConditionUnknownViolation
// concerns, so a caller can react without re-parsing the raw form.
type ConditionKind string

const (
	// ConditionExpression: a JEXL `when` expression (Expression holds the raw form;
	// a runtime-input `when` also reports as an expression).
	ConditionExpression ConditionKind = "expression"
	// ConditionStatus: a scope-status `when` gate (Status is Success/Failure/All).
	ConditionStatus ConditionKind = "status"
	// ConditionStrategyExcluded: the node's matrix strategy excludes every
	// combination, so the node is defined not to run. Only on ConditionViolation.
	ConditionStrategyExcluded ConditionKind = "strategy_excluded"
)

// ConditionViolation reports that the evaluated step is defined NOT to run: one of
// its `when` guards evaluated and did not hold, or its matrix strategy excludes
// every combination (Kind == ConditionStrategyExcluded). It is never raised for an
// ancestor — a skipped ancestor is stepped over, not a violation of the target.
// One violation is emitted per such fact — never grouped — so a caller can accept
// or block each independently via WithConditionPolicy. The guard is carried
// structurally (GuardFQN/Level/Kind and Expression or Status) so its source and
// form are available without string-matching. A guard that could NOT be evaluated
// is a ConditionUnknownViolation instead, a distinct fail-closed case.
type ConditionViolation struct {
	GuardFQN   string        // FQN of the unit that declared the guard (or the step, for strategy_excluded)
	Level      WhenLevel     // stage | stepGroup | step (WhenLevelUnset for strategy_excluded: no guard level)
	Kind       ConditionKind // expression | status | strategy_excluded
	Expression string        // the raw JEXL, when Kind == ConditionExpression
	Status     WhenStatus    // the scope status, when Kind == ConditionStatus

	description string
	ignored     bool
}

func (c *ConditionViolation) IsIgnored() bool { return c.ignored }
func (c *ConditionViolation) Reason() string  { return c.description }
func (c *ConditionViolation) Err() error      { return nil }
func (*ConditionViolation) isViolation()       {}

// ConditionUnknownViolation reports that a `when` guard — on the evaluated step OR
// on an ancestor the walk-up needed to classify — could not be evaluated (the
// ConditionFunc returned an error: an unresolved expression, a resolver gap). The
// SDK cannot tell whether the guarded node was required, so it fails closed. It is
// kept separate from ConditionViolation because an unmet guard is a pipeline fact
// while an unevaluable one is a resolver gap: accepting the second must not accept
// the first (WithConditionUnknownPolicy vs WithConditionPolicy). FQN is the node
// whose guard failed to evaluate; the guard itself is carried in GuardFQN/Level/
// Kind/Expression/Status and the raw error in Err.
type ConditionUnknownViolation struct {
	FQN        string        // the node whose guard could not be evaluated (the step or an ancestor)
	GuardFQN   string        // FQN of the unit that declared the guard
	Level      WhenLevel     // stage | stepGroup | step
	Kind       ConditionKind // expression | status
	Expression string        // the raw JEXL, when Kind == ConditionExpression
	Status     WhenStatus    // the scope status, when Kind == ConditionStatus

	description string
	err         error
	ignored     bool
}

func (c *ConditionUnknownViolation) IsIgnored() bool { return c.ignored }
func (c *ConditionUnknownViolation) Reason() string  { return c.description }
func (c *ConditionUnknownViolation) Err() error      { return c.err }
func (*ConditionUnknownViolation) isViolation()       {}

// AncestorDidNotRunViolation reports an ancestor requirement the run-state proves
// unsatisfied — the ancestor(s) definitively did not run. One violation is emitted
// per requirement so a caller can accept or block each independently: under an AND
// join, one per ancestor, its Ancestors holding that ancestor's still-missing FQNs
// (every un-run instance for a statically-enumerable fan-out); under an OR join, a
// single violation for the whole group, its Ancestors holding the candidate FQNs
// none of which has run. Join records which. A non-enumerable fan-out is NOT here —
// its completion is unverifiable, so it is a RuntimeFanoutViolation; a run-state
// lookup that failed is a RunStateUnknownViolation.
//
// The requirement is carried structurally so a policy can reason without parsing
// FQNs: AncestorFQN names the logical ancestor node (empty for an OR group, which
// spans several candidates), and Ancestors holds the still-missing FQNs.
type AncestorDidNotRunViolation struct {
	Join        JoinType
	AncestorFQN string   // logical ancestor node; empty for an OR group
	Ancestors   []string // still-missing FQNs (AND: instances of AncestorFQN; OR: group candidates)

	description string
	ignored     bool
}

func (m *AncestorDidNotRunViolation) IsIgnored() bool { return m.ignored }
func (m *AncestorDidNotRunViolation) Reason() string  { return m.description }
func (m *AncestorDidNotRunViolation) Err() error      { return nil }
func (*AncestorDidNotRunViolation) isViolation()       {}

// FanoutRole says whether a RuntimeFanoutViolation is about the evaluated step's
// own fan-out or a dependency's, so a policy can decide structurally rather than by
// comparing FQNs: a caller may let its own runtime-parallelism step run (FanoutSelf)
// while still blocking when an ancestor it depends on fans out unverifiably
// (FanoutAncestor).
type FanoutRole string

const (
	// FanoutSelf: the evaluated step itself fans out non-enumerably.
	FanoutSelf FanoutRole = "self"
	// FanoutAncestor: an ancestor the evaluated step gates on fans out non-enumerably.
	FanoutAncestor FanoutRole = "ancestor"
)

// RuntimeFanoutViolation reports a node — an ancestor OR the evaluated step itself —
// that fans out at runtime whose concrete instance FQNs are not statically
// enumerable, so its fan-in can never be verified complete: a run of the logical
// FQN proves only that SOME instance finished, not all. Status is why (a runtime
// <+input> cardinality or engine-assigned instance names) and Role which node it is
// (self | ancestor). It is fail-closed by default whenever such a node gates the
// step (WithRuntimeFanoutPolicy relaxes it); one is emitted per offending node.
// Structurally malformed strategies never reach here — they fail BuildGraph — so
// ignoring this never accepts a tampered graph.
type RuntimeFanoutViolation struct {
	FQN    string
	Status FanoutStatus // runtime_input | custom_names
	Role   FanoutRole   // self | ancestor

	description string
	ignored     bool
}

func (r *RuntimeFanoutViolation) IsIgnored() bool { return r.ignored }
func (r *RuntimeFanoutViolation) Reason() string  { return r.description }
func (r *RuntimeFanoutViolation) Err() error      { return nil }
func (*RuntimeFanoutViolation) isViolation()       {}

// RunStateUnknownViolation reports that the StepRanFunc could not determine whether
// a node ran (it returned an error: a lookup failed, state was unavailable). The
// SDK cannot tell whether the ancestor requirement was met, so it fails closed
// rather than assume either way. FQN is the node whose run-state was unknowable and
// Err carries the raw error (WithRunStateUnknownPolicy relaxes it).
type RunStateUnknownViolation struct {
	FQN string

	description string
	err         error
	ignored     bool
}

func (s *RunStateUnknownViolation) IsIgnored() bool { return s.ignored }
func (s *RunStateUnknownViolation) Reason() string  { return s.description }
func (s *RunStateUnknownViolation) Err() error      { return s.err }
func (*RunStateUnknownViolation) isViolation()       {}

// PipelineRollbackViolation reports that the step is the rollback entry of a stage
// with a PipelineRollback failure action, which runs in a separate uncorrelated
// plan; the forward gate can't be evaluated.
type PipelineRollbackViolation struct {
	description string
	ignored     bool
}

func (s *PipelineRollbackViolation) IsIgnored() bool { return s.ignored }
func (s *PipelineRollbackViolation) Reason() string  { return s.description }
func (s *PipelineRollbackViolation) Err() error      { return nil }
func (*PipelineRollbackViolation) isViolation()       {}

// Typed accessors for the violation kinds. Is* reports the kind; As* returns the
// concrete value (nil when the kind does not match) so callers need no type
// assertion.
func IsConditionViolation(v Violation) bool { _, ok := v.(*ConditionViolation); return ok }
func AsConditionViolation(v Violation) *ConditionViolation {
	c, _ := v.(*ConditionViolation)
	return c
}
func IsConditionUnknownViolation(v Violation) bool {
	_, ok := v.(*ConditionUnknownViolation)
	return ok
}
func AsConditionUnknownViolation(v Violation) *ConditionUnknownViolation {
	c, _ := v.(*ConditionUnknownViolation)
	return c
}
func IsAncestorDidNotRunViolation(v Violation) bool {
	_, ok := v.(*AncestorDidNotRunViolation)
	return ok
}
func AsAncestorDidNotRunViolation(v Violation) *AncestorDidNotRunViolation {
	m, _ := v.(*AncestorDidNotRunViolation)
	return m
}
func IsRuntimeFanoutViolation(v Violation) bool {
	_, ok := v.(*RuntimeFanoutViolation)
	return ok
}
func AsRuntimeFanoutViolation(v Violation) *RuntimeFanoutViolation {
	r, _ := v.(*RuntimeFanoutViolation)
	return r
}
func IsRunStateUnknownViolation(v Violation) bool {
	_, ok := v.(*RunStateUnknownViolation)
	return ok
}
func AsRunStateUnknownViolation(v Violation) *RunStateUnknownViolation {
	s, _ := v.(*RunStateUnknownViolation)
	return s
}
func IsPipelineRollbackViolation(v Violation) bool {
	_, ok := v.(*PipelineRollbackViolation)
	return ok
}
func AsPipelineRollbackViolation(v Violation) *PipelineRollbackViolation {
	s, _ := v.(*PipelineRollbackViolation)
	return s
}

// AncestorRef is one ancestor the evaluated step is gated on. It is a named type
// (rather than a bare FQN string) so more context can be added later without
// breaking callers; today it carries only the FQN — an instance FQN for a
// fanned-out ancestor, the logical FQN otherwise.
type AncestorRef struct {
	FQN string
}

// ConditionRef is one `when` guard considered on the evaluated step. It carries
// the full guard (When — so its source is available: OwnerFQN says which node
// declared it, Level whether it was a step- or stage-level guard) alongside Desc,
// the rendered form used in logs and messages.
type ConditionRef struct {
	When When
	Desc string
}

// Verdict is the outcome of checking whether a step may run now.
type Verdict struct {
	FQN     string   // logical FQN that was evaluated
	Allowed bool     // may this step run now? (no un-ignored violations)
	Join    JoinType // how its ancestors were gated (AND/OR)
	// Ancestors is the complete effective ancestor set: a fanned-out ancestor
	// contributes its concrete instance FQNs, a non-fanning one its logical FQN.
	Ancestors []AncestorRef
	// Conditions are the `when` guards considered on this step, with their source.
	Conditions []ConditionRef
	// Violations are the reasons the step may not run, most-specific first. Empty
	// when the step is allowed and unguarded. A violation carries whether it was
	// ignored (a With*Policy accepted it); Allowed is false iff any is not ignored.
	// There are no projection helpers over this slice: iterate it and switch on the
	// concrete kind (Is*/As*), which composes cleanly with a caller's own filtering.
	Violations []Violation
}

// EvaluateStep resolves fqn and decides whether it may run now: its when-guards
// must hold (WithCondition) and its ancestor join must be satisfied (WithRan) —
// JoinAND requires every ancestor, JoinOR at least one. An entry step with no
// ancestors is allowed once its guards hold. Reasons it may not run are collected
// as typed Violations; each is fail-closed unless a per-violation With*Policy
// option accepts it. Without WithRan nothing has run; without WithCondition guards
// default to AlwaysTrue. For fan-in, a statically-known ancestor requires every
// instance under AND; a runtime-input fan-out becomes a RuntimeFanoutViolation.
func (g *PipelineGraph) EvaluateStep(fqn string, opts ...EvaluateOption) (Verdict, error) {
	var eo evalOptions
	for _, opt := range opts {
		opt(&eo)
	}
	ran := eo.ran
	if ran == nil {
		ran = func(string) (bool, error) { return false, nil }
	}
	cond := eo.cond
	if cond == nil {
		cond = AlwaysTrue
	}

	res := g.Resolve(fqn)
	v := Verdict{FQN: res.FQN, Join: res.Join}
	if !res.Found {
		return Verdict{FQN: fqn}, ErrNodeNotFound
	}

	// The evaluated step's OWN facts are always locally knowable, so they are
	// evaluated even for a separate-plan rollback entry (only that entry's forward
	// gate is unknowable — handled below). Each is an independent gate that generates
	// its own violations and composes with the rest.

	// Its matrix strategy may exclude every combination, so the step is defined not
	// to run (the engine skips it gracefully). Surface it as a ConditionViolation of
	// kind strategy_excluded on the step itself, fail-closed under WithConditionPolicy.
	// Level is unset: strategy_excluded is a strategy fact, not a `when` guard.
	if res.Excluded {
		cv := &ConditionViolation{
			GuardFQN:    res.FQN,
			Level:       WhenLevelUnset,
			Kind:        ConditionStrategyExcluded,
			description: fmt.Sprintf("matrix strategy on %s excludes every combination; the step is defined not to run", res.FQN),
		}
		cv.ignored = applyPolicy(eo.conditionPolicy, cv)
		v.Violations = append(v.Violations, cv)
	}

	// Its OWN fan-out may not be statically enumerable (a runtime <+input> count or
	// engine-named instances). Surface it as its own RuntimeFanoutViolation
	// (FQN == the step's), fail-closed under WithRuntimeFanoutPolicy.
	if res.Status != FanoutEnumerable && res.Status != "" {
		rv := newRuntimeFanoutViolation(res.FQN, res.Status, FanoutSelf)
		rv.ignored = applyPolicy(eo.runtimeFanoutPolicy, rv)
		v.Violations = append(v.Violations, rv)
	}

	// Its own when-guards: evaluated, and a guard that does not hold (or cannot be
	// evaluated) becomes its own violation without short-circuiting the rest.
	report, condViolations := eo.conditionViolations(res.FQN, res.Conditions, cond)
	v.Conditions = report
	v.Violations = append(v.Violations, condViolations...)

	// The forward/ancestor gate. A PipelineRollback entry runs in a separate
	// uncorrelated plan whose forward run isn't visible, so ONLY this gate is
	// unknowable — surface exactly that (fail-closed unless a policy accepts it)
	// instead of gating on invisible state, but keep the step's own facts above. For
	// every other step, resolve the effective ancestors and gate the join.
	if res.SeparatePlanRollback {
		rb := &PipelineRollbackViolation{
			description: "pipeline-rollback entry runs in a separate plan; forward gate not evaluated",
		}
		rb.ignored = applyPolicy(eo.pipelineRollbackPolicy, rb)
		v.Violations = append(v.Violations, rb)
	} else {
		// Step over any ancestor that never runs — skipped by a `when` guard, or a
		// matrix strategy that excludes every combination — gating on that unit's live
		// predecessors instead, transitively, for BOTH joins. An ancestor whose guard
		// could not be evaluated is surfaced as a ConditionUnknownViolation (fail-closed,
		// relaxable via WithConditionUnknownPolicy) rather than silently required.
		// Coverage-pruning of redundant upstream nodes is valid only under AND ordering.
		ancestors := res.Ancestors
		if node := g.lookup(fqn); node != nil {
			var unknowns []*ConditionUnknownViolation
			ancestors, unknowns = liveAncestorsOf(g, node, fqn, cond, res.Join == JoinAND)
			for _, cu := range unknowns {
				cu.ignored = applyPolicy(eo.conditionUnknownPolicy, cu)
				v.Violations = append(v.Violations, cu)
			}
		}

		// Build the complete effective ancestor list: a fanned-out ancestor contributes
		// its concrete instance FQNs, a non-fanning (or non-enumerable) one its logical
		// FQN. An excluded ancestor never runs and is stepped over, so it is omitted.
		for _, a := range ancestors {
			if a.Excluded {
				continue
			}
			if len(a.Instances) > 0 {
				for _, inst := range a.Instances {
					v.Ancestors = append(v.Ancestors, AncestorRef{FQN: inst})
				}
			} else {
				v.Ancestors = append(v.Ancestors, AncestorRef{FQN: a.FQN})
			}
		}
		v.Violations = append(v.Violations, eo.ancestorViolations(res.Join, ancestors, ran)...)
	}

	// Allowed unless some violation is not ignored.
	v.Allowed = true
	for _, vi := range v.Violations {
		if !vi.IsIgnored() {
			v.Allowed = false
			break
		}
	}
	return v, nil
}

// conditionViolations records every when-guard considered (report, for the Verdict)
// and raises one violation per guard that does not run the step — never grouped, so
// a caller can accept or block each independently. A guard that evaluated and did
// not hold is a ConditionViolation; one the ConditionFunc could not evaluate (a
// non-nil error) fails closed as a ConditionUnknownViolation, so a policy can relax
// the unevaluable case while still blocking a known-false one. fqn is the evaluated
// step, carried on ConditionUnknownViolation so a caller knows whose guard it was.
func (eo evalOptions) conditionViolations(fqn string, conds []When, cond ConditionFunc) (report []ConditionRef, violations []Violation) {
	for _, w := range conds {
		report = append(report, ConditionRef{When: w, Desc: conditionDesc(w)})
		hold, err := cond(w)
		if err != nil {
			cu := newConditionUnknownViolation(fqn, w, err)
			cu.ignored = applyPolicy(eo.conditionUnknownPolicy, cu)
			violations = append(violations, cu)
			continue
		}
		if hold {
			continue
		}
		cv := newConditionViolation(w)
		cv.ignored = applyPolicy(eo.conditionPolicy, cv)
		violations = append(violations, cv)
	}
	return report, violations
}

// ancestorViolations gates the effective ancestor set for the join and raises the
// ancestor violations. An excluded ancestor never runs and is stepped over. A
// non-enumerable fan-out (Expands with no static instances) has no knowable instance
// set, so its fan-in can never be verified complete: even if the logical FQN has
// run, that only proves *some* instance finished, not all. It is therefore ALWAYS a
// RuntimeFanoutViolation (fail-closed by default) whenever it gates the step — never
// silently satisfied by a best-effort logical match. A run-state lookup that failed
// is a RunStateUnknownViolation, also fail-closed. An ancestor the run-state proves
// did not run is an AncestorDidNotRunViolation: one per ancestor under AND (its
// still-missing instance FQNs), one for the group under OR (the candidate FQNs).
func (eo evalOptions) ancestorViolations(join JoinType, ancestors []Ancestor, ran StepRanFunc) []Violation {
	var violations []Violation

	switch join {
	case JoinOR:
		// The OR fires once ANY live candidate has run. A non-enumerable fan-out is
		// never trusted to satisfy it (its completion is unverifiable) and an
		// unknowable run-state can't confirm it either; both only matter (and only
		// then block) when nothing else satisfied the join, so defer them.
		satisfied, live := false, 0
		var candidates []string
		var fanouts []*RuntimeFanoutViolation
		var unknowns []*RunStateUnknownViolation
		for _, a := range ancestors {
			if a.Excluded {
				continue
			}
			live++
			if a.Expands && len(a.Instances) == 0 {
				fanouts = append(fanouts, newRuntimeFanoutViolation(a.FQN, a.Status, FanoutAncestor))
				continue
			}
			for _, f := range effectiveFQNs([]Ancestor{a}) {
				ok, err := ran(f)
				if err != nil {
					// Run-state unknown: reported as its own violation, and NOT counted
					// among the "none of these ran" candidates — we can't claim it didn't.
					unknowns = append(unknowns, newRunStateUnknownViolation(f, err))
					continue
				}
				candidates = append(candidates, f)
				if ok {
					satisfied = true
				}
			}
		}
		if live == 0 {
			satisfied = true // an entry step: the OR is vacuously satisfied
		}
		if !satisfied {
			for _, rv := range fanouts {
				rv.ignored = applyPolicy(eo.runtimeFanoutPolicy, rv)
				violations = append(violations, rv)
			}
			for _, rsv := range unknowns {
				rsv.ignored = applyPolicy(eo.runStateUnknownPolicy, rsv)
				violations = append(violations, rsv)
			}
			if len(candidates) > 0 {
				// An OR group spans several candidates, so there is no single logical
				// ancestor node; all candidates are outstanding (none ran).
				av := newAncestorDidNotRunViolation(join, "", candidates)
				av.ignored = applyPolicy(eo.ancestorPolicy, av)
				violations = append(violations, av)
			}
		}
	default: // JoinAND
		for _, a := range ancestors {
			if a.Excluded {
				continue
			}
			if a.Expands && len(a.Instances) == 0 {
				rv := newRuntimeFanoutViolation(a.FQN, a.Status, FanoutAncestor)
				rv.ignored = applyPolicy(eo.runtimeFanoutPolicy, rv)
				violations = append(violations, rv)
				continue
			}
			effective := effectiveFQNs([]Ancestor{a})
			var missing []string
			for _, f := range effective {
				ok, err := ran(f)
				if err != nil {
					rsv := newRunStateUnknownViolation(f, err)
					rsv.ignored = applyPolicy(eo.runStateUnknownPolicy, rsv)
					violations = append(violations, rsv)
					continue
				}
				if !ok {
					missing = append(missing, f)
				}
			}
			if len(missing) > 0 {
				av := newAncestorDidNotRunViolation(join, a.FQN, missing)
				av.ignored = applyPolicy(eo.ancestorPolicy, av)
				violations = append(violations, av)
			}
		}
	}
	return violations
}

// conditionKind classifies a guard for a ConditionViolation/ConditionUnknownViolation:
// a scope-status gate reports ConditionStatus, everything else (a JEXL expression or a
// runtime-input `when`) reports ConditionExpression.
func conditionKind(w When) ConditionKind {
	if w.Status != WhenStatusUnset {
		return ConditionStatus
	}
	return ConditionExpression
}

// newConditionViolation builds the violation for a guard that evaluated and did not
// hold, carrying the guard structurally so a caller need not re-parse it.
func newConditionViolation(w When) *ConditionViolation {
	return &ConditionViolation{
		GuardFQN:    w.OwnerFQN,
		Level:       w.Level,
		Kind:        conditionKind(w),
		Expression:  w.Expression,
		Status:      w.Status,
		description: `"when" condition did not hold: ` + conditionDesc(w),
	}
}

// newConditionUnknownViolation builds the violation for a guard the ConditionFunc
// could not evaluate; fqn is the node whose guard it was and err the raw failure.
func newConditionUnknownViolation(fqn string, w When, err error) *ConditionUnknownViolation {
	return &ConditionUnknownViolation{
		FQN:         fqn,
		GuardFQN:    w.OwnerFQN,
		Level:       w.Level,
		Kind:        conditionKind(w),
		Expression:  w.Expression,
		Status:      w.Status,
		err:         err,
		description: fmt.Sprintf("%q on %s could not be evaluated: %v", conditionDesc(w), fqn, err),
	}
}

// newRuntimeFanoutViolation builds the violation for a node whose runtime fan-out
// cannot be statically enumerated, describing why from its FanoutStatus and which
// node it is (self | ancestor) from its FanoutRole.
func newRuntimeFanoutViolation(fqn string, status FanoutStatus, role FanoutRole) *RuntimeFanoutViolation {
	why := "fans out a runtime <+input> count of instances"
	if status == FanoutCustomNames {
		why = "fans out into engine-named instances"
	}
	return &RuntimeFanoutViolation{
		FQN:         fqn,
		Status:      status,
		Role:        role,
		description: fmt.Sprintf("%s %s; fan-in cannot be verified complete", fqn, why),
	}
}

// newRunStateUnknownViolation builds the violation for a node whose run-state the
// StepRanFunc could not determine.
func newRunStateUnknownViolation(fqn string, err error) *RunStateUnknownViolation {
	return &RunStateUnknownViolation{
		FQN:         fqn,
		err:         err,
		description: fmt.Sprintf("run-state of %s could not be determined: %v", fqn, err),
	}
}

// newAncestorDidNotRunViolation builds the violation for an ancestor requirement the
// run-state proves unsatisfied, wording the reason for the join. ancestorFQN is the
// logical ancestor node (empty for an OR group); missing holds the still-outstanding
// FQNs (a fan-in ancestor's un-run instances, else the OR group's candidates).
func newAncestorDidNotRunViolation(join JoinType, ancestorFQN string, missing []string) *AncestorDidNotRunViolation {
	desc := "preceding steps have not finished: " + strings.Join(missing, ", ")
	if join == JoinOR {
		desc = "no preceding step has run (need any of): " + strings.Join(missing, ", ")
	}
	return &AncestorDidNotRunViolation{
		Join:        join,
		AncestorFQN: ancestorFQN,
		Ancestors:   missing,
		description: desc,
	}
}

// conditionDesc renders a when-guard for logs and error messages (it also fills
// ConditionRef.Desc). Expression guards use the raw JEXL (matching the override-key
// convention); status/runtime-input guards get a readable form.
func conditionDesc(w When) string {
	switch {
	case w.RuntimeInput:
		return fmt.Sprintf("<+input> when-guard on %s", w.OwnerFQN)
	case w.Expression != "":
		return w.Expression
	case w.Status != WhenStatusUnset:
		return fmt.Sprintf("%s %s-status gate", w.Status, w.Level)
	default:
		return fmt.Sprintf("when-guard on %s", w.OwnerFQN)
	}
}

// applyPolicy runs a per-violation fail-open policy: a nil policy fails closed
// (returns false), otherwise the caller's decision for this occurrence stands.
// This is the single point where a violation may become ignored — the SDK never
// fails open on its own.
func applyPolicy[T Violation](fn func(T) bool, v T) bool {
	if fn == nil {
		return false
	}
	return fn(v)
}

// effectiveFQNs flattens ancestors to the FQNs actually gated on: instance FQNs
// for a fanned-out ancestor, the logical FQN otherwise.
func effectiveFQNs(as []Ancestor) []string {
	var out []string
	for _, a := range as {
		if len(a.Instances) > 0 {
			out = append(out, a.Instances...)
		} else {
			out = append(out, a.FQN)
		}
	}
	return out
}

// guardSkip classifies whether node would be skipped at runtime because a `when`
// guard governing it (its own or an enclosing container's) does not hold under cond.
// skipped is true when a guard evaluated and did not hold — the node provably never
// runs, so liveAncestorsOf walks past it (a provably-false guard overrides any
// unevaluable one). When no guard proved it skipped but at least one could NOT be
// evaluated, unknown carries a ConditionUnknownViolation for the first such guard:
// the SDK can't tell whether the node runs, so the caller fails closed on that fact
// rather than either skipping or silently requiring it. No-op under AlwaysTrue.
func guardSkip(node *graphNode, cond ConditionFunc) (skipped bool, unknown *ConditionUnknownViolation) {
	var firstUnknown *ConditionUnknownViolation
	for c := node; c != nil; c = c.parent {
		if c.when == nil {
			continue
		}
		hold, err := cond(*c.when)
		if err != nil {
			if firstUnknown == nil {
				firstUnknown = newConditionUnknownViolation(node.fqn, *c.when, err)
			}
			continue
		}
		if !hold {
			return true, nil
		}
	}
	return false, firstUnknown
}

// liveAncestorsOf returns target's effective direct predecessors, substituting any
// skipped predecessor (guardSkip) or all-excluded matrix with its own live
// predecessors, transitively (s1->s2->s3 with s2 skipped gates s3 on s1) — for both
// joins. A predecessor whose guard could not be evaluated is not gated on directly;
// instead its ConditionUnknownViolation is returned so the caller can fail closed on
// the unevaluable fact (relaxable via WithConditionUnknownPolicy) without blanket-
// requiring the ancestor. Instances are computed against the ORIGINAL target so
// shared-strategy fan-in pinning stays correct. prune drops redundant upstream nodes
// a live sibling already implies — valid only under mandatory AND ordering. Kept
// standalone (taking g) to keep graph construction separate from evaluation.
func liveAncestorsOf(g *PipelineGraph, target *graphNode, rawTarget string, cond ConditionFunc, prune bool) ([]Ancestor, []*ConditionUnknownViolation) {
	var out []Ancestor
	var unknowns []*ConditionUnknownViolation
	var nodes []*graphNode // retained node per out entry, for redundancy pruning
	seen := make(map[string]bool)
	var visit func(node *graphNode)
	visit = func(node *graphNode) {
		var conds []When // climb needs a sink; the gathered guards are unused here
		for _, p := range g.climb(node, &conds) {
			if seen[p.fqn] {
				continue
			}
			seen[p.fqn] = true
			// A matrix that excludes every combination never runs: step over it,
			// gating on ITS live predecessors instead (no guard to evaluate).
			if strategyExcluded(p) {
				visit(p)
				continue
			}
			// A predecessor provably skipped by a `when` guard is likewise stepped
			// over. One whose guard could not be evaluated is surfaced as an unknown
			// and NOT gated on directly — the caller fails closed on that fact.
			skipped, unknown := guardSkip(p, cond)
			if unknown != nil {
				unknowns = append(unknowns, unknown)
				continue
			}
			if skipped {
				visit(p)
				continue
			}
			a := Ancestor{FQN: p.fqn, Type: p.stepType, Expands: hasStrategyInChain(p)}
			if insts, ok := g.ancestorInstances(p, target, rawTarget); ok {
				a.Instances = insts
			} else if a.Expands {
				a.Status = strategyStatus(p)
			}
			out = append(out, a)
			nodes = append(nodes, p)
		}
	}
	visit(target)

	if !prune {
		return out, unknowns
	}

	// Stepping over a skipped predecessor climbs its branch deeper than one level,
	// which can surface an upstream node that a live sibling already implies (e.g.
	// a skipped parallel branch resolves to the step before the parallel, but the
	// live sibling in that same parallel already gates on it). Drop such a node: a
	// plain sequential chain A->B->C gates C on B alone (B ran => A ran), and the
	// frontier must stay just as minimal. Only prune on mandatory AND ordering — if
	// the covering node is OR-joined its running would not imply the pruned node ran.
	if len(out) < 2 {
		return out, unknowns
	}
	pruned := out[:0:0]
	for i, a := range out {
		if coveredByAnother(g, nodes, i) {
			continue
		}
		pruned = append(pruned, a)
	}
	return pruned, unknowns
}

// coveredByAnother reports whether nodes[i] is a mandatory transitive predecessor
// of some other node in nodes — i.e. that other node having run necessarily implies
// nodes[i] ran, so gating on nodes[i] as well is redundant.
func coveredByAnother(g *PipelineGraph, nodes []*graphNode, i int) bool {
	for j, b := range nodes {
		if j == i {
			continue
		}
		if precedesVia(g, nodes[i], b) {
			return true
		}
	}
	return false
}

// precedesVia reports whether b running implies a ran: a is reachable by climbing
// backward from b through AND-joined nodes only. An OR-joined node (a Failure/All
// `when` or a rollback entry) stops the climb — its running does not imply its
// predecessors ran, so nothing beyond it can be treated as covered.
func precedesVia(g *PipelineGraph, a, b *graphNode) bool {
	visited := make(map[*graphNode]bool)
	var walk func(n *graphNode) bool
	walk = func(n *graphNode) bool {
		if orJoined(n) {
			return false // n ran does not imply n's predecessors ran
		}
		var conds []When
		for _, p := range g.climb(n, &conds) {
			if p == a {
				return true
			}
			if visited[p] {
				continue
			}
			visited[p] = true
			if walk(p) {
				return true
			}
		}
		return false
	}
	return walk(b)
}

// orJoined reports whether node gates its predecessors as an OR (fires once ANY
// ran) rather than an AND (all ran) — a Failure/All `when` or a rollback entry.
// Mirrors the Join classification in Resolve.
func orJoined(node *graphNode) bool {
	if node.when != nil && (node.when.Status == WhenStatusFailure || node.when.Status == WhenStatusAll) {
		return true
	}
	return atRollbackEntry(node)
}

// VerifyStepOrder builds the graph from resolved YAML and evaluates fqn. Supply
// run-state and guards via WithRan/WithCondition; without WithCondition guards
// default to AlwaysTrue. It returns ErrInvalidPipeline (wrapped) if the YAML
// can't be parsed and ErrNodeNotFound if fqn is not in the pipeline, so callers
// fail closed; a nil error means the returned verdict is fully populated. buildOpts must match
// the runtime engine's fan-out settings (e.g. WithMatrixNaming); evalOpts supply
// the inputs for this single evaluation.
func VerifyStepOrder(resolvedYAML, fqn string, buildOpts []BuildOption, evalOpts ...EvaluateOption) (Verdict, error) {
	g, err := BuildGraph(resolvedYAML, buildOpts...)
	if err != nil {
		return Verdict{FQN: fqn}, err
	}
	return g.EvaluateStep(fqn, evalOpts...)
}
