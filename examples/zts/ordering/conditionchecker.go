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
	"errors"
	"maps"
	"strings"
	"sync"

	"github.com/harness/zero-trust-service/verifier/pipeline"
)

// ErrUnresolvedCondition is returned by Eval when a `when` guard cannot be decided
// (a runtime-input guard, or an expression with no override and no evaluator). It
// maps the checker's internal "not known" to the pipeline.ConditionFunc error
// contract, so the SDK fails closed with a ConditionUnknownViolation.
var ErrUnresolvedCondition = errors.New("ordering: when-guard could not be evaluated")

// ConditionChecker is a reference pipeline.ConditionFunc for `when` guards;
// replace in a real integration. It splits a When into a status gate (assumed
// Success on the happy path unless MarkStatus records a real status) and an
// expression gate resolved parser-free by Overrides, else handed to an optional
// Evaluator. A guard that cannot be evaluated fails closed (the step is denied):
// this checker never allows a step on a guard whose result it doesn't know.
type ConditionChecker struct {
	// Overrides forces a raw `when` expression to run/skip by exact string match,
	// bypassing evaluation. See Config.ConditionOverrides.
	Overrides map[string]bool
	// Evaluator optionally evaluates an expression not covered by Overrides; nil
	// means such an expression is unevaluable and the guard fails closed.
	Evaluator ConditionEvaluator
	// Vars are forwarded to the Evaluator verbatim.
	Vars map[string]string

	mu       sync.Mutex
	statuses map[string]map[string]string // planExecID -> owner FQN -> status
}

// ConditionEvaluator evaluates a raw `when` JEXL expression under planExecID.
// known is false when the expression is not evaluated; an unknown guard fails
// closed.
type ConditionEvaluator interface {
	EvalExpr(planExecID, expr string, vars map[string]string) (result, known bool)
}

// ConditionEvaluatorFunc adapts a func to ConditionEvaluator.
type ConditionEvaluatorFunc func(planExecID, expr string, vars map[string]string) (bool, bool)

// EvalExpr implements ConditionEvaluator.
func (f ConditionEvaluatorFunc) EvalExpr(planExecID, expr string, vars map[string]string) (bool, bool) {
	return f(planExecID, expr, vars)
}

// NewConditionChecker returns a checker delegating expression evaluation to
// eval (forwarding vars); a nil eval means no expression can be evaluated.
func NewConditionChecker(eval ConditionEvaluator, vars map[string]string) *ConditionChecker {
	return &ConditionChecker{Evaluator: eval, Vars: vars, statuses: make(map[string]map[string]string)}
}

// MergeOverrides overlays o onto the checker's Overrides, with o taking
// precedence. A no-op for an empty map. Lets expression overrides be supplied
// from config as well as at construction (see Config.ConditionOverrides).
func (c *ConditionChecker) MergeOverrides(o map[string]bool) {
	if len(o) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Overrides == nil {
		c.Overrides = make(map[string]bool, len(o))
	}
	maps.Copy(c.Overrides, o)
}

// MarkStatus records an owner's observed status ("Success" | "Failure") under
// planExecID, overriding statusGate's happy-path assumption for that owner.
// Optional: absent a record, the gate assumes Success.
func (c *ConditionChecker) MarkStatus(planExecID, ownerFQN, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	set := c.statuses[planExecID]
	if set == nil {
		set = make(map[string]string)
		c.statuses[planExecID] = set
	}
	set[ownerFQN] = status
}

// Eval reports whether the when-guard w holds under planExecID. A guard is the AND
// of its status gate and expression gate: known false if either gate is known
// false, known true if both are known true, otherwise unresolvable. An unresolvable
// guard returns ErrUnresolvedCondition so the SDK fails closed (a
// ConditionUnknownViolation): the checker never lets a step run on a guard it can't
// decide. Its signature matches pipeline.ConditionFunc.
func (c *ConditionChecker) Eval(planExecID string, w pipeline.When) (hold bool, err error) {
	// Whole `when` is a runtime input; cannot be reasoned about statically -> unknown.
	if w.RuntimeInput {
		return false, ErrUnresolvedCondition
	}
	statusOK, statusKnown := c.statusGate(planExecID, w)
	exprOK, exprKnown := c.exprGate(planExecID, w)
	if (statusKnown && !statusOK) || (exprKnown && !exprOK) {
		return false, nil // one gate is known false: the guard is known false
	}
	if statusKnown && exprKnown {
		return statusOK && exprOK, nil
	}
	return false, ErrUnresolvedCondition // unresolvable
}

// statusGate evaluates w.Status against the owner's status. "" and "All" always
// hold. For "Success"/"Failure" the happy-path default is to assume the owner
// (stage/pipeline) succeeded, so a Success guard holds and a Failure guard does
// not — the status gate is therefore always known, and only an unresolved
// expression can leave a guard unresolvable. A real integration may record the
// actual status via MarkStatus, which then takes precedence over this default.
func (c *ConditionChecker) statusGate(planExecID string, w pipeline.When) (ok, known bool) {
	switch strings.ToLower(strings.TrimSpace(string(w.Status))) {
	case "", "all":
		return true, true
	}
	observed := "Success" // happy-path assumption unless a real status is recorded
	c.mu.Lock()
	if s, seen := c.statuses[planExecID][w.OwnerFQN]; seen {
		observed = s
	}
	c.mu.Unlock()
	return strings.EqualFold(observed, string(w.Status)), true
}

// exprGate evaluates w.Expression. An empty expression holds. An exact Overrides
// match wins with no evaluation (keys are compared verbatim after trimming outer
// whitespace, so quotes are matched literally). Otherwise the Evaluator decides;
// a nil evaluator leaves the expression unknown.
func (c *ConditionChecker) exprGate(planExecID string, w pipeline.When) (ok, known bool) {
	expr := strings.TrimSpace(w.Expression)
	if expr == "" {
		return true, true
	}
	if res, found := c.Overrides[expr]; found {
		return res, true
	}
	if c.Evaluator == nil {
		return false, false
	}
	return c.Evaluator.EvalExpr(planExecID, expr, c.Vars)
}
