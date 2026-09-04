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
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrInvalidPipeline is returned by BuildGraph (and wrapped by VerifyStepOrder)
// when resolved YAML cannot be parsed or has no pipeline root. Ordering checks
// fail closed on it rather than proceeding against an empty or partial graph.
var ErrInvalidPipeline = errors.New("pipeline: invalid or unparseable YAML")

// ErrUnsupportedPipeline is returned by BuildGraph when the YAML is well-formed
// but declares a construct the SDK does not model — an unrecognized `strategy`
// shape whose fan-out it cannot reason about. Distinct from ErrInvalidPipeline
// (structurally corrupt) so a caller can tell "we don't support this" apart from
// "this is broken"; both fail the build closed rather than gate a partial graph.
var ErrUnsupportedPipeline = errors.New("pipeline: unsupported construct")

// PipelineGraph is an immutable execution-ordering DAG built from resolved
// pipeline YAML. Ordering is positional (steps/stages run sequentially,
// `parallel` blocks concurrently); it also captures `when` guards. Built once
// via BuildGraph, safe for concurrent reads, then queried with Resolve.
type PipelineGraph struct {
	root         *graphNode
	byFQN        map[string]*graphNode
	instByFQN    map[string]*graphNode // enumerated runtime instance FQN -> logical node
	duplicates   []string              // logical FQNs that appeared more than once (malformed pipeline)
	matrixNaming MatrixNaming          // how matrix instances are named (see BuildGraph options)
	// buildErr holds the first strategy-resolution error encountered while building
	// (a malformed or unsupported `strategy`). It is not stored on the graph for
	// later use — BuildGraph returns it and discards the graph — but accumulated
	// here because the builders (buildStage/buildStep/...) don't thread errors.
	buildErr error
}

// nodeKind classifies a graph node.
type nodeKind int

const (
	kindPipeline nodeKind = iota // synthetic root; children are stages
	kindStage
	kindStepGroup
	kindStep     // leaf
	kindParallel // transparent concurrency wrapper; no FQN of its own
	kindRollback // container for a stage's rollbackSteps; a failure-path sub-DAG
)

// graphNode is one unit in the ordering DAG.
type graphNode struct {
	kind     nodeKind
	id       string        // identifier ("" for parallel/pipeline)
	fqn      string        // logical FQN ("" for parallel/pipeline)
	stepType string        // the `type` field, for steps/stages (informational)
	when     *When         // conditional-execution guard on this unit, if any
	strategy bool          // this unit declares a `strategy` (matrix/repeat/parallelism)
	strat    *strategySpec // parsed fan-out of this unit's strategy (nil if none)

	parent     *graphNode
	index      int          // position within parent.children
	concurrent bool         // children run concurrently (true only for parallel)
	children   []*graphNode // ordered unless concurrent

	// rollback (stage node only) roots the rollbackSteps sub-DAG, kept out of
	// children so failure-path steps stay out of forward ordering.
	rollback *graphNode

	// pipelineRollback (stage node only) marks a PipelineRollback failure action,
	// whose rollbackSteps run in a separate plan not correlatable to the forward run.
	pipelineRollback bool
}

// WhenLevel identifies which construct a When guard belongs to.
type WhenLevel string

const (
	// WhenLevelUnset is the zero value: no when-guard level applies. Carried by a
	// ConditionViolation whose Kind is ConditionStrategyExcluded — that fact is a
	// node's matrix strategy, not a `when` guard, so it has no guard level.
	WhenLevelUnset WhenLevel = ""
	WhenStage      WhenLevel = "stage"
	WhenStepGroup  WhenLevel = "stepGroup"
	WhenStep       WhenLevel = "step"
)

// WhenStatus is the scope-status a `when` guard gates on (`stageStatus` for a
// step/stepGroup, `pipelineStatus` for a stage). An enum rather than a bare
// string so callers match a guard without spelling out literals; the zero value
// is WhenStatusUnset (no status gate, e.g. an expression-only or runtime-input
// `when`). A Failure/All gate turns the join into an OR (failure branch).
type WhenStatus string

const (
	WhenStatusUnset   WhenStatus = ""        // no status gate (expression-only or runtime-input `when`)
	WhenStatusSuccess WhenStatus = "Success" // runs on scope success (the default forward path)
	WhenStatusFailure WhenStatus = "Failure" // runs on scope failure (an OR/failure branch)
	WhenStatusAll     WhenStatus = "All"     // runs regardless of scope status (also an OR branch)
)

// When captures a `when` guard. Stage Status comes from `pipelineStatus`;
// step/stepGroup from `stageStatus`. Expression is the optional JEXL condition.
type When struct {
	Level        WhenLevel
	OwnerFQN     string     // FQN of the unit this guard belongs to
	Status       WhenStatus // Success | Failure | All (WhenStatusUnset if RuntimeInput or expression-only)
	Expression   string     // JEXL condition; "" if none
	RuntimeInput bool       // the whole `when` was a runtime expression (e.g. "<+input>"), unresolved statically
}

// Ancestor is a direct predecessor: a leaf step that must have completed
// immediately before the resolved step may start. When several ancestors are
// returned they form a join — ALL of them must have completed.
type Ancestor struct {
	FQN  string // logical FQN of the predecessor leaf step (or opaque stage)
	Type string // the step's `type` (e.g. ShellScript); "" for parallel/opaque nodes
	// Expands is true when this predecessor (or an enclosing container) declares a
	// strategy, fanning out at runtime into "<FQN>_0","<FQN>_1",…; a join over it
	// requires every instance. Match with (*PipelineGraph).MatchesAncestor.
	Expands bool
	// Instances holds the concrete runtime instance FQNs when the fan-out is
	// statically enumerable (an AND join requires all). Nil when there is no
	// strategy or the fan-out size is a runtime input (Expands may still be true) —
	// then gate on FQN and let the run-state store reconcile instances.
	Instances []string
	// Status explains why a fanning ancestor could not be enumerated. Set only when
	// Expands is true and Instances is nil (FanoutEnumerable otherwise).
	Status FanoutStatus
	// Excluded is true when this predecessor's matrix strategy excludes every
	// combination, so it is skipped at runtime and never gates the step (callers
	// step over it, as for a `when`-skipped predecessor).
	Excluded bool
}

// JoinType classifies how a step's ancestor set gates its execution.
type JoinType int

const (
	// JoinAND (default): the step runs only after ALL ancestors complete —
	// forward/positional ordering (sequential, parallel fan-in, strategy fan-in).
	JoinAND JoinType = iota
	// JoinOR means the step runs once ANY ancestor completes — the gate for
	// failure-path constructs (a failure-branch step, a stage's rollback entry).
	// rollbackSteps form a separate sub-DAG (kindRollback); the entry OR-gates on
	// the stage's forward leaves so rollback runs once any forward step has run.
	JoinOR
)

// String renders the join type as "AND"/"OR" for logs and diagnostics.
func (j JoinType) String() string {
	if j == JoinOR {
		return "OR"
	}
	return "AND"
}

// Resolution answers the ordering question for a single step.
type Resolution struct {
	FQN         string     // logical FQN of the resolved step
	Found       bool       // false if the FQN is not in the pipeline
	Type        string     // the resolved step's `type` (e.g. ShellScript); "" if not found
	HasStrategy bool       // the step (or an enclosing container) has a strategy
	// Status is the evaluated step's OWN fan-out status, set only when it has a
	// strategy whose runtime instances aren't statically enumerable (a benign
	// runtime <+input> or engine-named fan-out). FanoutEnumerable ("") when there
	// is no strategy or it enumerates cleanly. Malformed shapes never reach here —
	// they fail BuildGraph.
	Status      FanoutStatus
	// Excluded is true when the step's own matrix strategy excludes every
	// combination: it is skipped at runtime and so is defined not to run.
	Excluded    bool
	Join        JoinType   // how Ancestors gate this step (AND by default, OR on failure branches)
	Ancestors   []Ancestor // direct predecessors; a join (see Join: ALL for AND, ANY for OR)
	Conditions  []When     // the step's own `when` + guards of containers it is the entry of
	// SeparatePlanRollback is true when the step is the rollback ENTRY of a stage
	// with a PipelineRollback failure action. That rollback runs in a separate plan
	// whose forward run-state is not correlated here, so the gate can't be evaluated
	// and callers should fail open. Later rollback steps are ordered normally.
	SeparatePlanRollback bool
}

// The raw* types mirror only the schema slice affecting execution ordering and
// conditions (identifiers, step/stage/stepGroup/parallel wrappers, `when`,
// `strategy`); yaml.v3 drops all other unknown fields.

type rawPipelineDoc struct {
	Pipeline *rawPipeline `yaml:"pipeline"`
}

type rawPipeline struct {
	Stages []rawStageWrapper `yaml:"stages"`
}

// rawStageWrapper is one stages entry: either a stage or a parallel block.
type rawStageWrapper struct {
	Stage    *rawStage         `yaml:"stage"`
	Parallel []rawStageWrapper `yaml:"parallel"`
}

type rawStage struct {
	Identifier        string               `yaml:"identifier"`
	Type              string               `yaml:"type"`
	When              *rawWhen             `yaml:"when"`
	Strategy          *rawStrategy         `yaml:"strategy"`
	Spec              *rawStageSpec        `yaml:"spec"`
	FailureStrategies []rawFailureStrategy `yaml:"failureStrategies"`
}

// rawFailureStrategy captures only the failure action type. A PipelineRollback
// action runs rollbackSteps in a separate plan, uncorrelatable to the forward run.
type rawFailureStrategy struct {
	OnFailure struct {
		Action struct {
			Type string `yaml:"type"`
		} `yaml:"action"`
	} `yaml:"onFailure"`
}

// present is a YAML presence marker: it decodes to true whenever its key exists
// in the document, whatever the value's shape, and stays false when the key is
// absent. Used for fields we only need to detect (a fan-out we can't enumerate),
// not parse — cheaper and clearer than probing a retained *yaml.Node for nil.
type present bool

func (p *present) UnmarshalYAML(*yaml.Node) error {
	*p = true
	return nil
}

// rawStageSpec is generic across stage types (all expose execution under spec;
// pipeline/dynamic stages omit it). A Deployment stage may also fan out via
// multi-service/environment/infrastructure deployment (see multiDeploySpec).
type rawStageSpec struct {
	Execution        *rawExecution         `yaml:"execution"`
	Services         *rawMultiServices     `yaml:"services"`
	Environments     *rawMultiEnvironments `yaml:"environments"`
	EnvironmentGroup present               `yaml:"environmentGroup"` // presence marks a fan-out that is not enumerated
}

// rawMultiServices is a Deployment stage's `spec.services`; each serviceRef is
// one value in the multi-service fan-out.
type rawMultiServices struct {
	Values []rawServiceValue `yaml:"values"`
}

type rawServiceValue struct {
	ServiceRef string `yaml:"serviceRef"`
}

// rawMultiEnvironments is the `spec.environments` block; each entry pairs an
// environmentRef with its infrastructureDefinitions (or gitOpsClusters).
type rawMultiEnvironments struct {
	Values []rawEnvironmentValue `yaml:"values"`
}

type rawEnvironmentValue struct {
	EnvironmentRef            string       `yaml:"environmentRef"`
	InfrastructureDefinitions rawInfraDefs `yaml:"infrastructureDefinitions"`
	GitOpsClusters            present      `yaml:"gitOpsClusters"` // GitOps fan-out, not enumerated
}

type rawInfraDef struct {
	Identifier string `yaml:"identifier"`
}

// rawInfraDefs is an environment's infrastructureDefinitions. A literal list is
// present only when not a runtime input; any non-sequence form (`<+input>`) is a
// runtime fan-out, not statically enumerable.
type rawInfraDefs struct {
	defs         []rawInfraDef
	runtimeInput bool
}

func (r *rawInfraDefs) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		return value.Decode(&r.defs)
	}
	r.runtimeInput = true // scalar/mapping (e.g. "<+input>"): not statically enumerable
	return nil
}

type rawExecution struct {
	Steps []rawStepWrapper `yaml:"steps"`
	// RollbackSteps run on the failure path. Modeled as a separate sub-DAG
	// (kindRollback), ordered among themselves but never entangled with forward steps.
	RollbackSteps []rawStepWrapper `yaml:"rollbackSteps"`
}

// rawStepWrapper is one steps entry: a step, a stepGroup, or a parallel block.
type rawStepWrapper struct {
	Step      *rawStep         `yaml:"step"`
	StepGroup *rawStepGroup    `yaml:"stepGroup"`
	Parallel  []rawStepWrapper `yaml:"parallel"`
}

type rawStep struct {
	Identifier string       `yaml:"identifier"`
	Type       string       `yaml:"type"`
	When       *rawWhen     `yaml:"when"`
	Strategy   *rawStrategy `yaml:"strategy"`
}

type rawStepGroup struct {
	Identifier string           `yaml:"identifier"`
	When       *rawWhen         `yaml:"when"`
	Strategy   *rawStrategy     `yaml:"strategy"`
	Steps      []rawStepWrapper `yaml:"steps"`
}

// rawWhen captures a `when` guard. The schema allows either an object or a scalar
// runtime expression (e.g. "<+input>", "<+pipeline.variables.flag>"), so it needs
// a custom unmarshaler.
type rawWhen struct {
	RuntimeInput   bool
	StageStatus    string
	PipelineStatus string
	Condition      string
}

func (w *rawWhen) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		// A scalar `when` that is any Harness expression (matching harness-core's
		// NGExpressionUtils.isRuntimeOrExpressionField) is resolved at execution
		// time; mark it so it is never enforced statically.
		w.RuntimeInput = containsExpression(value.Value)
		return nil
	}
	if value.Kind != yaml.MappingNode {
		// A `when` that is neither a scalar expression nor an object is malformed.
		// Fail closed (BuildGraph surfaces this as ErrInvalidPipeline) rather than
		// silently dropping the guard, which would fail open.
		return fmt.Errorf("%w: `when` at line %d must be an object or a runtime expression", ErrInvalidPipeline, value.Line)
	}
	var obj struct {
		StageStatus    string `yaml:"stageStatus"`
		PipelineStatus string `yaml:"pipelineStatus"`
		Condition      string `yaml:"condition"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	w.StageStatus, w.PipelineStatus, w.Condition = obj.StageStatus, obj.PipelineStatus, obj.Condition
	return nil
}

// FanoutStatus classifies a fanning node's runtime instances: statically
// enumerable, or non-enumerable for a specific benign reason. It is a string so it
// prints readably in logs and messages without a lookup. It is set on an Ancestor
// only when Expands is true and Instances is nil, and lets a caller decide
// fail-open/closed per cause. Malformed or unrecognized strategies are NOT a
// status here — they fail BuildGraph (ErrInvalidPipeline / ErrUnsupportedPipeline)
// so a corrupt or unmodeled fan-out never reaches evaluation.
type FanoutStatus string

const (
	// FanoutEnumerable: the fan-out is statically enumerable (Instances populated)
	// or there is no fan-out. The zero value is "" (unset), treated as enumerable.
	FanoutEnumerable FanoutStatus = "enumerable"
	// FanoutRuntimeInput: the fan-out size is a runtime <+input>/expression
	// (matrix <+input>, runtime axis, parallelism <+input>, repeat times/items
	// input, multi-service/env/infra runtime refs). Benign; commonly failed open.
	FanoutRuntimeInput FanoutStatus = "runtime_input"
	// FanoutCustomNames: instances exist but are named at runtime (matrix/repeat
	// nodeName, sub-ranged/partitioned repeat), so their FQNs aren't derivable.
	// Benign; commonly failed open.
	FanoutCustomNames FanoutStatus = "custom_names"
)

// strategySpec is the resolved fan-out of a unit's `strategy`. When Known,
// Suffixes holds each runtime instance's suffix in engine order (e.g.
// ["_0","_1","_2"] for a 3-way repeat, ["_0_0","_0_1","_1_0","_1_1"] for a 2x2
// matrix). Known is false when the fan-out isn't derivable from the YAML
// (`<+input>`, custom `nodeName`, sub-ranged repeat); status then records which
// benign cause it was — callers match on the logical FQN. A malformed or
// unrecognized shape is not represented here: resolution returns an error and
// BuildGraph fails. excluded marks a matrix whose every combination is excluded,
// so the node is skipped at runtime (Known, but with no instances).
type strategySpec struct {
	Known    bool
	Suffixes []string
	status   FanoutStatus // meaningful only when Known is false and not excluded
	excluded bool         // every matrix combination excluded: node is skipped, no instances
}

// MatrixNaming selects how the engine names matrix instances, so the SDK
// enumerates the same instance FQNs.
type MatrixNaming int

const (
	// MatrixNamingIndex (engine default) suffixes each instance by the zero-based
	// positional index of its axis value, declaration order, row-major ("_0_1").
	MatrixNamingIndex MatrixNaming = iota
	// MatrixNamingValue suffixes by axis value instead: values sorted by axis
	// key, non-alphanumerics replaced by "_" ("_us_free").
	MatrixNamingValue
)

// BuildOption configures graph construction.
type BuildOption func(*buildOptions)

type buildOptions struct {
	matrixNaming MatrixNaming
}

// WithMatrixNaming selects the matrix instance-naming convention the graph
// enumerates (default MatrixNamingIndex).
func WithMatrixNaming(m MatrixNaming) BuildOption {
	return func(o *buildOptions) { o.matrixNaming = m }
}

// rawStrategy is a fully-decoded `strategy` block. The YAML is parsed into typed
// specs at unmarshal time and no yaml.Node is retained; the naming-dependent
// suffix enumeration still runs at build time (stratSpec) since it depends on
// MatrixNaming. A well-formed strategy populates exactly one of the specs; a
// scalar strategy ("strategy: <+input>") or an unmodeled shape populates none.
type rawStrategy struct {
	// runtimeInput records a scalar/expression strategy ("strategy: <+input>"): a
	// present-but-non-enumerable fan-out whose cause is a runtime input (benign),
	// NOT a malformed shape. Stored here rather than left nil so resolve can tell
	// the two apart instead of defaulting the scalar case to FanoutMalformed.
	runtimeInput bool
	matrix       *matrixStrategy
	parallelism  *parallelismStrategy
	repeat       *repeatStrategy
}

func (s *rawStrategy) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		// A scalar strategy is a runtime expression ("strategy: <+input>"): a benign
		// non-enumerable fan-out, not a malformed one. Any other non-mapping shape
		// (e.g. a sequence) stays unmodeled and resolves to FanoutMalformed below.
		if value.Kind == yaml.ScalarNode {
			s.runtimeInput = true
		}
		return nil
	}
	// yaml.v3 in this tree errors when decoding a scalar into a *yaml.Node field,
	// so key presence is detected via a value Node's non-zero Kind. The nodes are
	// consumed here and never stored on the typed specs.
	var body struct {
		Matrix      yaml.Node `yaml:"matrix"`
		Parallelism yaml.Node `yaml:"parallelism"`
		Repeat      yaml.Node `yaml:"repeat"`
	}
	if err := value.Decode(&body); err != nil {
		return nil // unmodeled shape: no fan-out (gate on the logical FQN)
	}
	if body.Matrix.Kind != 0 {
		m := parseMatrix(&body.Matrix)
		s.matrix = &m
	}
	if body.Parallelism.Kind != 0 {
		p := parseParallelism(&body.Parallelism)
		s.parallelism = &p
	}
	if body.Repeat.Kind != 0 {
		r := parseRepeat(&body.Repeat)
		s.repeat = &r
	}
	return nil
}

// stratSpec computes a raw unit's strategy fan-out under the graph's naming
// option, or nil when the unit declares no strategy. A malformed or unsupported
// strategy shape is recorded on g.buildErr (first one wins, tagged with fqn) so
// BuildGraph fails closed; the returned spec is unused in that case.
func (g *PipelineGraph) stratSpec(rs *rawStrategy, fqn string) *strategySpec {
	if rs == nil {
		return nil
	}
	spec, err := rs.resolve(g.matrixNaming)
	if err != nil && g.buildErr == nil {
		g.buildErr = fmt.Errorf("strategy on %s: %w", fqn, err)
	}
	return &spec
}

// resolve enumerates the strategy's instance suffixes under naming. It returns a
// non-enumerable spec (Known=false, with a benign status) for shapes that can't
// be enumerated statically (runtime-input strategy/axes, custom instance names,
// sub-ranged repeat, ...), an excluded spec for an all-excluded matrix, and an
// error for a malformed (ErrInvalidPipeline) or unrecognized (ErrUnsupportedPipeline)
// shape so BuildGraph can fail closed rather than gate a fan-out it can't model.
func (s *rawStrategy) resolve(naming MatrixNaming) (strategySpec, error) {
	switch {
	case s.matrix != nil:
		return matrixSuffixes(*s.matrix, naming)
	case s.parallelism != nil:
		if s.parallelism.valid {
			return strategySpec{Known: true, Suffixes: rangeSuffixes(s.parallelism.count)}, nil
		}
		return strategySpec{status: FanoutRuntimeInput}, nil // parallelism: <+input>
	case s.repeat != nil:
		return repeatSuffixes(*s.repeat), nil
	case s.runtimeInput:
		return strategySpec{status: FanoutRuntimeInput}, nil // scalar "strategy: <+input>"
	}
	return strategySpec{}, fmt.Errorf("%w: unrecognized strategy shape (no matrix/parallelism/repeat)", ErrUnsupportedPipeline)
}

// matrixStrategy is a decoded `matrix` block, retaining no yaml.Node. Axis values
// are captured as strings, with a count that survives non-scalar (object) values
// so index-mode enumeration still works. Flags mark shapes that are present but
// non-enumerable (runtime input, custom names, a runtime-input axis or malformed
// exclude).
type matrixStrategy struct {
	runtimeInput bool // `matrix: <+input>` (scalar): non-enumerable
	nodeName     bool // custom (expression-resolved) instance names: non-enumerable
	badShape     bool // an axis is not a sequence, or `exclude` is malformed
	axes         []matrixAxis
	excludes     []map[string]string
}

// matrixAxis is one matrix axis. count is always known; vals is populated (and
// allScalar true) only when every value is a scalar — index-mode with no excludes
// needs just the count and so tolerates object-valued axes.
type matrixAxis struct {
	name      string
	vals      []string
	count     int
	allScalar bool
}

// parseMatrix decodes a `matrix` node into a matrixStrategy, preserving axis
// declaration order (index naming is row-major in that order). `maxConcurrency`
// is ignored, `nodeName` and non-sequence axes mark it non-enumerable.
func parseMatrix(v *yaml.Node) matrixStrategy {
	if v == nil {
		return matrixStrategy{badShape: true}
	}
	if v.Kind != yaml.MappingNode {
		return matrixStrategy{runtimeInput: true} // scalar "<+input>"
	}
	var m matrixStrategy
	for i := 0; i+1 < len(v.Content); i += 2 {
		key, val := v.Content[i].Value, v.Content[i+1]
		switch key {
		case "maxConcurrency":
			continue // concurrency cap, not an axis
		case "nodeName":
			m.nodeName = true
			continue
		case "exclude":
			if val.Kind != yaml.SequenceNode {
				m.badShape = true
				continue
			}
			sets, ok := parseExcludeSets(val.Content)
			if !ok {
				m.badShape = true
				continue
			}
			m.excludes = sets
			continue
		}
		if val.Kind != yaml.SequenceNode {
			// A scalar axis that is a Harness expression ("axis: <+input>") is a
			// benign runtime-input fan-out, not a malformed one; any other non-sequence
			// (a literal scalar or a mapping) is structurally invalid.
			if val.Kind == yaml.ScalarNode && containsExpression(val.Value) {
				m.runtimeInput = true
			} else {
				m.badShape = true
			}
			continue
		}
		ax := matrixAxis{name: key, count: len(val.Content), allScalar: true}
		vals := make([]string, 0, len(val.Content))
		for _, n := range val.Content {
			if n.Kind != yaml.ScalarNode {
				ax.allScalar = false
				break
			}
			vals = append(vals, n.Value)
		}
		if ax.allScalar {
			ax.vals = vals
		}
		m.axes = append(m.axes, ax)
	}
	return m
}

// matrixSuffixes enumerates the matrix's instance suffixes under naming. Index
// mode with no exclusions needs only axis lengths (so object-valued axes are fine);
// every other path compares/emits values and so requires scalar axes. Kept a free
// function (not a method) so matrixStrategy stays pure data with no knowledge of
// how suffixes are built. A structurally invalid matrix (non-sequence axis, bad
// exclude, no axes) returns ErrInvalidPipeline; an all-excluded matrix returns an
// excluded spec (the node is skipped, not gated).
func matrixSuffixes(m matrixStrategy, naming MatrixNaming) (strategySpec, error) {
	switch {
	case m.runtimeInput:
		return strategySpec{status: FanoutRuntimeInput}, nil // matrix: <+input>, or a runtime-input axis
	case m.badShape:
		return strategySpec{}, fmt.Errorf("%w: matrix axis is not a sequence, or `exclude` is malformed", ErrInvalidPipeline)
	case m.nodeName:
		return strategySpec{status: FanoutCustomNames}, nil // runtime-named instances
	case len(m.axes) == 0:
		return strategySpec{}, fmt.Errorf("%w: matrix has no axes to enumerate", ErrInvalidPipeline)
	}
	if naming == MatrixNamingIndex && len(m.excludes) == 0 {
		lens := make([]int, len(m.axes))
		for i, a := range m.axes {
			lens[i] = a.count
		}
		// productSuffixes is empty when any axis is empty (a: []): a zero-instance
		// matrix that leaves no steps, treated as excluded like an all-excluded one.
		return enumerated(productSuffixes(lens)), nil
	}
	axes := make([]mAxis, len(m.axes))
	for i, a := range m.axes {
		if !a.allScalar {
			// Object-valued axis under value/exclude naming: valid, but instance
			// names aren't derivable from the values — benign, gate on the FQN.
			return strategySpec{status: FanoutCustomNames}, nil
		}
		axes[i] = mAxis{name: a.name, vals: a.vals}
	}
	if naming == MatrixNamingValue {
		return matrixValueSuffixes(axes, m.excludes), nil
	}
	out := matrixCombos(axes, m.excludes, func(vi int, _ string) string { return "_" + strconv.Itoa(vi) })
	// Empty when every combination is excluded (or an axis is empty): the node is
	// skipped at runtime, surfaced as excluded.
	return enumerated(out), nil
}

// parallelismStrategy is a decoded `parallelism` value. valid is true only for a
// literal int > 0; a runtime input ("<+input>") leaves it invalid (non-enumerable).
type parallelismStrategy struct {
	count int
	valid bool
}

func parseParallelism(v *yaml.Node) parallelismStrategy {
	if n, ok := intNode(v); ok && n > 0 {
		return parallelismStrategy{count: n, valid: true}
	}
	return parallelismStrategy{}
}

// repeatStrategy is a decoded `repeat` block, retaining no yaml.Node. times/items
// drive the instance count; the presence of any count- or name-altering key
// (start/end/unit/partitionSize/nodeName) makes the fan-out non-enumerable.
type repeatStrategy struct {
	start, end, unit, partitionSize, nodeName bool
	timesSet   bool // `times` key present (any form)
	timesOK    bool // times resolved to an int > 0
	times      int
	itemsSeq   bool // `items` is a literal sequence
	itemsCount int
}

// parseRepeat decodes a `repeat` node. An unmodeled shape (e.g. `repeat: <+input>`)
// yields a zero repeatStrategy, which enumerates to non-enumerable.
func parseRepeat(v *yaml.Node) repeatStrategy {
	var raw struct {
		Times         yaml.Node `yaml:"times"` // int or "<+input>"
		Items         yaml.Node `yaml:"items"` // sequence or "<+input>"
		Start         present   `yaml:"start"`
		End           present   `yaml:"end"`
		Unit          present   `yaml:"unit"`
		PartitionSize present   `yaml:"partitionSize"`
		NodeName      present   `yaml:"nodeName"`
	}
	if v == nil || v.Kind != yaml.MappingNode || v.Decode(&raw) != nil {
		return repeatStrategy{} // unmodeled repeat shape: non-enumerable
	}
	r := repeatStrategy{
		start: bool(raw.Start), end: bool(raw.End), unit: bool(raw.Unit),
		partitionSize: bool(raw.PartitionSize), nodeName: bool(raw.NodeName),
	}
	if raw.Times.Kind != 0 {
		r.timesSet = true
		if n, ok := intNode(&raw.Times); ok && n > 0 {
			r.times, r.timesOK = n, true
		}
	}
	if raw.Items.Kind == yaml.SequenceNode {
		r.itemsSeq = true
		r.itemsCount = len(raw.Items.Content)
	}
	return r
}

// repeatSuffixes enumerates a repeat strategy: `times: N` or a literal `items`
// list both fan out to N instances suffixed _0.._N-1 (the iteration index). A
// count- or name-altering key, or a runtime-input times/items, is non-enumerable.
// Kept a free function (not a method) so repeatStrategy stays pure data.
func repeatSuffixes(r repeatStrategy) strategySpec {
	if r.nodeName {
		return strategySpec{status: FanoutCustomNames} // runtime-named instances
	}
	if r.start || r.end || r.unit || r.partitionSize {
		return strategySpec{status: FanoutRuntimeInput} // sub-ranging / partitioning: count is runtime-derived
	}
	switch {
	case r.timesSet:
		if r.timesOK {
			return strategySpec{Known: true, Suffixes: rangeSuffixes(r.times)}
		}
		return strategySpec{status: FanoutRuntimeInput} // times: <+input> or non-positive
	case r.itemsSeq:
		// items: [] leaves no iterations: a zero-instance node, skipped like an
		// all-excluded matrix rather than blocking its followers forever.
		return enumerated(rangeSuffixes(r.itemsCount))
	default:
		return strategySpec{status: FanoutRuntimeInput} // items: <+input>, or neither times nor items
	}
}

// Multi-deployment matrix keys used when fanning a Deployment stage across
// services/environments/infrastructures. The instance suffix is these keys'
// values sorted by key (environmentRef, infraIdentifier, serviceRef).
const (
	mdKeyEnvironmentRef  = "environmentRef"
	mdKeyInfraIdentifier = "infraIdentifier"
	mdKeyServiceRef      = "serviceRef"
)

// multiDeploySpec enumerates the runtime stage instances a Deployment stage fans
// out into via multi-service/environment/infrastructure deployment (independent
// of any `strategy:` block; always named by ref value). Returns nil when there is
// no such fan-out, and a non-enumerable spec (Known=false) when it can't be
// enumerated statically (runtime/expression refs, GitOps clusters, environment
// groups, or a name collision).
func (g *PipelineGraph) multiDeploySpec(spec *rawStageSpec) *strategySpec {
	if spec == nil {
		return nil
	}
	hasSvc := spec.Services != nil && len(spec.Services.Values) > 0
	hasEnv := spec.Environments != nil && len(spec.Environments.Values) > 0
	if spec.EnvironmentGroup {
		return &strategySpec{status: FanoutRuntimeInput} // environment-group fan-out: present but not enumerated
	}
	if !hasSvc && !hasEnv {
		return nil
	}

	var serviceRefs []string
	if hasSvc {
		for _, sv := range spec.Services.Values {
			if !isLiteralRef(sv.ServiceRef) {
				return &strategySpec{status: FanoutRuntimeInput}
			}
			serviceRefs = append(serviceRefs, sv.ServiceRef)
		}
	}

	// Environment/infrastructure dimension: infra is nested per environment, so
	// each (env, infra) pair is one combination row.
	type envInfra struct {
		env, infra string
		hasInfra   bool
	}
	var envPairs []envInfra
	if hasEnv {
		for _, ev := range spec.Environments.Values {
			if !isLiteralRef(ev.EnvironmentRef) || bool(ev.GitOpsClusters) {
				return &strategySpec{status: FanoutRuntimeInput}
			}
			if ev.InfrastructureDefinitions.runtimeInput {
				return &strategySpec{status: FanoutRuntimeInput} // infra list is a runtime input: not enumerable
			}
			if len(ev.InfrastructureDefinitions.defs) == 0 {
				envPairs = append(envPairs, envInfra{env: ev.EnvironmentRef})
				continue
			}
			for _, inf := range ev.InfrastructureDefinitions.defs {
				if !isLiteralRef(inf.Identifier) {
					return &strategySpec{status: FanoutRuntimeInput}
				}
				envPairs = append(envPairs, envInfra{env: ev.EnvironmentRef, infra: inf.Identifier, hasInfra: true})
			}
		}
	}

	// Cartesian product of the configured dimensions. A dimension that isn't
	// configured contributes a single empty placeholder so the other still fans.
	svcs := serviceRefs
	if !hasSvc {
		svcs = []string{""}
	}
	pairs := envPairs
	if !hasEnv {
		pairs = []envInfra{{}}
	}
	seen := make(map[string]struct{})
	var suffixes []string
	for _, svc := range svcs {
		for _, ep := range pairs {
			m := make(map[string]string, 3)
			if hasSvc {
				m[mdKeyServiceRef] = svc
			}
			if hasEnv {
				m[mdKeyEnvironmentRef] = ep.env
				if ep.hasInfra {
					m[mdKeyInfraIdentifier] = ep.infra
				}
			}
			suf := multiDeploySuffix(m)
			if _, dup := seen[suf]; dup {
				return &strategySpec{status: FanoutRuntimeInput} // engine appends a dedup counter, not modeled
			}
			seen[suf] = struct{}{}
			suffixes = append(suffixes, suf)
		}
	}
	if len(suffixes) == 0 {
		return nil
	}
	return &strategySpec{Known: true, Suffixes: suffixes}
}

// multiDeploySuffix builds one runtime instance suffix from a combination's
// matrix values: values ordered by key, dots removed, joined by "_", non-
// alphanumerics replaced by "_", capped at the engine's length, prefixed by "_".
func multiDeploySuffix(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, strings.ReplaceAll(m[k], ".", ""))
	}
	postfix := nonAlphanumeric.ReplaceAllString(strings.Join(parts, "_"), "_")
	if len(postfix) > maxMatrixSuffixLen {
		postfix = postfix[:maxMatrixSuffixLen]
	}
	return "_" + postfix
}

// exprStart and exprEnd are the delimiters that bracket every Harness expression
// ("<+input>", "<+pipeline.variables.x>", "<+step.output>", ...). Detection
// mirrors harness-core's NGExpressionUtils.hasExpressions, which finds tokens
// delimited by ExpressionConstants.EXPR_START ("<+") and EXPR_END (">") — see
// 980-commons EngineExpressionEvaluator.findExpressions.
const (
	exprStart = "<+"
	exprEnd   = ">"
)

// containsExpression reports whether s carries any Harness expression, not just
// the literal "<+input>". This matches harness-core's
// NGExpressionUtils.isRuntimeOrExpressionField (matchesInputSetPattern ||
// hasExpressions): a value carries an expression when it has an opening "<+"
// followed by a closing ">", so it is resolved at execution time and cannot be
// reasoned about statically. A bare "<+" with no closer is not an expression.
func containsExpression(s string) bool {
	i := strings.Index(s, exprStart)
	return i >= 0 && strings.Contains(s[i+len(exprStart):], exprEnd)
}

// isLiteralRef reports whether a ref is a concrete identifier (not empty and not
// an expression), so it can be enumerated statically.
func isLiteralRef(s string) bool {
	return strings.TrimSpace(s) != "" && !containsExpression(s)
}

// combineStageSpecs merges a stage's explicit-strategy and multi-deployment
// fan-outs. One present wins; when both fan a stage out the combined naming is
// not enumerable (interleave order unmodeled), so gate on the logical FQN.
func combineStageSpecs(strategy, multiDeploy *strategySpec) *strategySpec {
	switch {
	case strategy == nil:
		return multiDeploy
	case multiDeploy == nil:
		return strategy
	default:
		return &strategySpec{status: FanoutRuntimeInput}
	}
}

// mAxis is one matrix axis: its key and its literal values (as strings).
type mAxis struct {
	name string
	vals []string
}

// matrixValueSuffixes enumerates value-named instances: axes sorted by key,
// each value a sanitized "_<value>" piece, capped at 127 chars. A collision
// after sanitization (which the engine disambiguates with a counter) is left
// non-enumerable.
func matrixValueSuffixes(axes []mAxis, exSets []map[string]string) strategySpec {
	sorted := append([]mAxis(nil), axes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	raw := matrixCombos(sorted, exSets, func(_ int, val string) string { return "_" + sanitizeMatrixValue(val) })
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = truncateMatrixSuffix(s)
		if _, dup := seen[s]; dup {
			return strategySpec{status: FanoutCustomNames} // engine appends a dedup counter, not guessed here
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	// Empty when every combination is excluded (or an axis is empty): skipped node.
	return enumerated(out)
}

// matrixCombos builds the cartesian product of axes (in the given order),
// dropping combinations that match any exclude set. piece maps an axis value's
// (index, value) to the suffix fragment it contributes.
func matrixCombos(axes []mAxis, exSets []map[string]string, piece func(valueIdx int, value string) string) []string {
	type combo struct {
		suffix string
		vals   map[string]string
	}
	combos := []combo{{suffix: "", vals: map[string]string{}}}
	for _, ax := range axes {
		next := make([]combo, 0, len(combos)*len(ax.vals))
		for _, c := range combos {
			for vi, val := range ax.vals {
				nv := make(map[string]string, len(c.vals)+1)
				maps.Copy(nv, c.vals)
				nv[ax.name] = val
				next = append(next, combo{suffix: c.suffix + piece(vi, val), vals: nv})
			}
		}
		combos = next
	}
	out := make([]string, 0, len(combos))
	for _, c := range combos {
		if !matchesAnyExclude(c.vals, exSets) {
			out = append(out, c.suffix)
		}
	}
	return out
}

// parseExcludeSets converts `exclude` entries to axis->value maps, failing on a
// non-mapping entry or non-scalar value.
func parseExcludeSets(excludes []*yaml.Node) ([]map[string]string, bool) {
	sets := make([]map[string]string, 0, len(excludes))
	for _, e := range excludes {
		if e.Kind != yaml.MappingNode {
			return nil, false
		}
		m := make(map[string]string, len(e.Content)/2)
		for i := 0; i+1 < len(e.Content); i += 2 {
			if e.Content[i+1].Kind != yaml.ScalarNode {
				return nil, false
			}
			m[e.Content[i].Value] = e.Content[i+1].Value
		}
		sets = append(sets, m)
	}
	return sets, true
}

// matchesAnyExclude reports whether a combination's axis values match any
// exclude entry (an entry matches when every axis it names equals the combo's
// value for that axis).
func matchesAnyExclude(vals map[string]string, exSets []map[string]string) bool {
	for _, ex := range exSets {
		matched := true
		for k, v := range ex {
			if vals[k] != v {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// nonAlphanumeric matches any character the engine replaces with "_" when naming
// matrix instances by value.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizeMatrixValue mirrors the engine's replaceAll("[^a-zA-Z0-9]","_") on
// matrix axis values used in value-based instance names.
func sanitizeMatrixValue(s string) string {
	return nonAlphanumeric.ReplaceAllString(s, "_")
}

// maxMatrixSuffixLen is the engine's cap on the strategy identifier postfix.
const maxMatrixSuffixLen = 127

// truncateMatrixSuffix caps a value-based suffix at the engine's limit.
func truncateMatrixSuffix(s string) string {
	if len(s) > maxMatrixSuffixLen {
		return s[:maxMatrixSuffixLen]
	}
	return s
}

// rangeSuffixes returns ["_0","_1",...,"_(n-1)"].
func rangeSuffixes(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, "_"+strconv.Itoa(i))
	}
	return out
}

// enumerated wraps a statically enumerated suffix list into a strategySpec. An
// empty list means the strategy leaves no instances to run (an empty matrix axis,
// empty repeat items, or an exclude list that removes every combination); the
// engine skips such a node gracefully without failing the pipeline (this is behind
// a Harness feature flag). It is surfaced with the same `excluded` fact an
// all-excluded matrix carries, so followers step over it and the step itself fails
// closed but is relaxable via WithConditionPolicy — the caller decides whether the
// skip is acceptable rather than the SDK hard-coding it.
func enumerated(suffixes []string) strategySpec {
	if len(suffixes) == 0 {
		return strategySpec{excluded: true}
	}
	return strategySpec{Known: true, Suffixes: suffixes}
}

// productSuffixes returns the row-major cartesian product of index ranges as
// suffixes: lens [2,3] -> ["_0_0","_0_1","_0_2","_1_0","_1_1","_1_2"]. The
// leftmost axis varies slowest, matching axis-declaration order.
func productSuffixes(lens []int) []string {
	combos := []string{""}
	for _, n := range lens {
		next := make([]string, 0, len(combos)*n)
		for _, c := range combos {
			for i := range n {
				next = append(next, c+"_"+strconv.Itoa(i))
			}
		}
		combos = next
	}
	return combos
}

// intNode extracts a non-negative integer from a scalar YAML node.
func intNode(v *yaml.Node) (int, bool) {
	if v == nil || v.Kind != yaml.ScalarNode {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v.Value))
	if err != nil {
		return 0, false
	}
	return n, true
}

// BuildGraph parses resolved pipeline YAML and builds the ordering DAG. It
// returns ErrInvalidPipeline (wrapped) when the YAML cannot be parsed or has no
// pipeline root, so callers fail closed rather than gate against an empty graph.
// Options control how the graph enumerates strategy fan-out (see WithMatrixNaming).
func BuildGraph(resolvedYAML string, opts ...BuildOption) (*PipelineGraph, error) {
	var doc rawPipelineDoc
	// A TypeError is a partial decode (an unmodeled runtime-input shape); keep the
	// doc so one field can't disable ordering. Any other error is a real failure.
	if err := yaml.Unmarshal([]byte(resolvedYAML), &doc); err != nil {
		if _, ok := err.(*yaml.TypeError); !ok {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPipeline, err)
		}
	}
	if doc.Pipeline == nil {
		return nil, fmt.Errorf("%w: no pipeline root", ErrInvalidPipeline)
	}
	var o buildOptions // zero value = MatrixNamingIndex (engine default)
	for _, opt := range opts {
		opt(&o)
	}
	g := &PipelineGraph{
		root:         &graphNode{kind: kindPipeline},
		byFQN:        make(map[string]*graphNode),
		instByFQN:    make(map[string]*graphNode),
		matrixNaming: o.matrixNaming,
	}
	g.root.children = g.buildStages(doc.Pipeline.Stages, "pipeline.stages", g.root)
	// A malformed or unsupported strategy anywhere makes the fan-out unknowable, so
	// gating any step against this graph could be wrong. Fail closed with the first
	// such error (ErrInvalidPipeline / ErrUnsupportedPipeline).
	if g.buildErr != nil {
		return nil, g.buildErr
	}
	g.indexInstances()
	// A duplicate logical FQN means two steps/stages share an identifier: the
	// pipeline is invalid and picking one over the other could gate the wrong
	// node. Fail closed rather than silently keeping one.
	if len(g.duplicates) > 0 {
		return nil, fmt.Errorf("%w: duplicate step/stage identifiers: %s", ErrInvalidPipeline, strings.Join(g.duplicates, ", "))
	}
	return g, nil
}

// indexInstances maps every statically enumerable runtime instance FQN to its
// logical node, so lookup resolves an instance FQN ("s1_us_a") under either
// naming mode. A real node wins over an instance entry; an instance FQN two nodes
// both enumerate is dropped rather than guessed.
func (g *PipelineGraph) indexInstances() {
	ambiguous := make(map[string]struct{})
	for _, n := range g.byFQN {
		insts, ok := instanceFQNs(n)
		if !ok {
			continue
		}
		for _, inst := range insts {
			if inst == n.fqn {
				continue // no strategy suffix: same as the logical FQN
			}
			if _, isReal := g.byFQN[inst]; isReal {
				continue
			}
			if _, dup := ambiguous[inst]; dup {
				continue
			}
			if _, exists := g.instByFQN[inst]; exists {
				delete(g.instByFQN, inst)
				ambiguous[inst] = struct{}{}
				continue
			}
			g.instByFQN[inst] = n
		}
	}
}

// buildStages builds the ordered stage-level children, treating `parallel` as a
// transparent concurrency wrapper (it contributes no FQN segment).
func (g *PipelineGraph) buildStages(wrappers []rawStageWrapper, prefix string, parent *graphNode) []*graphNode {
	var out []*graphNode
	for _, w := range wrappers {
		switch {
		case w.Parallel != nil:
			pn := &graphNode{kind: kindParallel, parent: parent, concurrent: true, index: len(out)}
			pn.children = g.buildStages(w.Parallel, prefix, pn)
			out = append(out, pn)
		case w.Stage != nil:
			out = append(out, g.buildStage(w.Stage, prefix, parent, len(out)))
		}
	}
	return out
}

func (g *PipelineGraph) buildStage(s *rawStage, prefix string, parent *graphNode, index int) *graphNode {
	fqn := prefix + "." + s.Identifier
	// A stage can fan out via an explicit `strategy:` block and/or (for
	// Deployment stages) multi-service/environment/infrastructure deployment.
	strat := combineStageSpecs(g.stratSpec(s.Strategy, fqn), g.multiDeploySpec(s.Spec))
	n := &graphNode{
		kind:     kindStage,
		id:       s.Identifier,
		fqn:      fqn,
		stepType: s.Type,
		when:     makeWhen(s.When, WhenStage, fqn),
		strategy: strat != nil,
		strat:    strat,
		parent:   parent,
		index:    index,
	}
	// A PipelineRollback failure action runs rollbackSteps in a separate plan,
	// so that rollback path cannot be correlated to the forward run's state.
	for _, fs := range s.FailureStrategies {
		if strings.EqualFold(fs.OnFailure.Action.Type, "PipelineRollback") {
			n.pipelineRollback = true
			break
		}
	}
	// Descend into spec.execution.steps when present. Absent for pipeline /
	// dynamic stages, which then act as opaque terminal units.
	if s.Spec != nil && s.Spec.Execution != nil {
		n.children = g.buildSteps(s.Spec.Execution.Steps, fqn+".spec.execution.steps", n)
		// rollbackSteps hang off n.rollback (not n.children) as a separate sub-DAG:
		// ordered internally but never entangled with forward ordering.
		if len(s.Spec.Execution.RollbackSteps) > 0 {
			rb := &graphNode{kind: kindRollback, fqn: fqn + ".spec.execution.rollbackSteps", parent: n}
			rb.children = g.buildSteps(s.Spec.Execution.RollbackSteps, rb.fqn, rb)
			n.rollback = rb
		}
	}
	g.register(n)
	return n
}

// buildSteps builds the ordered children of a steps array (stage execution or a
// stepGroup body). `parallel` is transparent (no FQN segment).
func (g *PipelineGraph) buildSteps(wrappers []rawStepWrapper, prefix string, parent *graphNode) []*graphNode {
	var out []*graphNode
	for _, w := range wrappers {
		switch {
		case w.Parallel != nil:
			pn := &graphNode{kind: kindParallel, parent: parent, concurrent: true, index: len(out)}
			pn.children = g.buildSteps(w.Parallel, prefix, pn)
			out = append(out, pn)
		case w.StepGroup != nil:
			out = append(out, g.buildStepGroup(w.StepGroup, prefix, parent, len(out)))
		case w.Step != nil:
			out = append(out, g.buildStep(w.Step, prefix, parent, len(out)))
		}
	}
	return out
}

func (g *PipelineGraph) buildStepGroup(sg *rawStepGroup, prefix string, parent *graphNode, index int) *graphNode {
	fqn := prefix + "." + sg.Identifier
	n := &graphNode{
		kind:     kindStepGroup,
		id:       sg.Identifier,
		fqn:      fqn,
		when:     makeWhen(sg.When, WhenStepGroup, fqn),
		strategy: sg.Strategy != nil,
		strat:    g.stratSpec(sg.Strategy, fqn),
		parent:   parent,
		index:    index,
	}
	n.children = g.buildSteps(sg.Steps, fqn+".steps", n)
	g.register(n)
	return n
}

func (g *PipelineGraph) buildStep(st *rawStep, prefix string, parent *graphNode, index int) *graphNode {
	fqn := prefix + "." + st.Identifier
	n := &graphNode{
		kind:     kindStep,
		id:       st.Identifier,
		fqn:      fqn,
		stepType: st.Type,
		when:     makeWhen(st.When, WhenStep, fqn),
		strategy: st.Strategy != nil,
		strat:    g.stratSpec(st.Strategy, fqn),
		parent:   parent,
		index:    index,
	}
	g.register(n)
	return n
}

// makeWhen converts a parsed rawWhen into the public When. Stage guards carry
// pipelineStatus; step/stepGroup guards carry stageStatus. Returns nil when
// there is no guard.
func makeWhen(rw *rawWhen, level WhenLevel, ownerFQN string) *When {
	if rw == nil {
		return nil
	}
	if rw.RuntimeInput {
		return &When{Level: level, OwnerFQN: ownerFQN, RuntimeInput: true}
	}
	status := rw.StageStatus
	if status == "" {
		status = rw.PipelineStatus
	}
	return &When{Level: level, OwnerFQN: ownerFQN, Status: WhenStatus(status), Expression: rw.Condition}
}

// Resolve answers, for the given step FQN, the minimal set of direct-ancestor
// steps that must have completed immediately before it, and the conditions
// under which it executes. The FQN may be a logical FQN or a runtime FQN with
// strategy index suffixes (e.g. "...steps.deploy_0"); suffixes are normalized
// away for the lookup.
func (g *PipelineGraph) Resolve(fqn string) Resolution {
	if g == nil {
		return Resolution{FQN: fqn}
	}
	node := g.lookup(fqn)
	if node == nil {
		return Resolution{FQN: fqn}
	}

	res := Resolution{FQN: node.fqn, Found: true, Type: node.stepType, HasStrategy: hasStrategyInChain(node)}
	// The evaluated step's OWN fan-out: an all-excluded matrix means it is skipped
	// (defined not to run); otherwise, a non-enumerable strategy sets Status so a
	// caller can see its instances aren't statically knowable. FanoutEnumerable ("")
	// when there is no strategy or it enumerates cleanly.
	if res.HasStrategy {
		if strategyExcluded(node) {
			res.Excluded = true
		} else if _, ok := instanceFQNs(node); !ok {
			res.Status = strategyStatus(node)
		}
	}
	if node.when != nil {
		res.Conditions = append(res.Conditions, *node.when)
		// A step guarded to run on scope failure is an OR gate (fires once any
		// predecessor failed, not after ALL complete). Derivable only when the
		// step's own `when` targets Failure/All.
		if node.when.Status == WhenStatusFailure || node.when.Status == WhenStatusAll {
			res.Join = JoinOR
		}
	}
	preds := g.climb(node, &res.Conditions)
	// A rollback entry's predecessors are the stage's forward leaves; it fires
	// once any of them has run, so gate them as an OR (failure-triggered).
	if atRollbackEntry(node) {
		res.Join = JoinOR
		// Only the entry gates on the forward run. Under PipelineRollback that run
		// lives in a separate, uncorrelated plan, so the entry can't be gated — flag
		// it for callers to fail open. Later rollback steps are ordered normally.
		if rb := rollbackContainer(node); rb != nil && rb.parent != nil && rb.parent.pipelineRollback {
			res.SeparatePlanRollback = true
		}
	}
	for _, p := range preds {
		a := Ancestor{FQN: p.fqn, Type: p.stepType, Expands: hasStrategyInChain(p)}
		if strategyExcluded(p) {
			a.Excluded = true // matrix excludes every combination: p is skipped, gates nothing
		} else if insts, ok := g.ancestorInstances(p, node, fqn); ok {
			a.Instances = insts
		} else if a.Expands {
			a.Status = strategyStatus(p)
		}
		res.Ancestors = append(res.Ancestors, a)
	}
	return res
}

// instanceFQNs enumerates the runtime instance FQNs leaf step p fans out into,
// composing every strategy in its ancestor chain as a cartesian product, each
// contributing its suffix at its own FQN boundary (a repeat step in a matrix
// stepGroup: "...sg_0.steps.deploy_1"). Returns (nil, false) when the chain has
// no strategy or any is not statically enumerable — callers then gate on the
// logical FQN.
func instanceFQNs(p *graphNode) ([]string, bool) {
	type level struct {
		prefix   string // FQN of the strategy-bearing node (a prefix of p.fqn)
		suffixes []string
	}
	// Innermost (p) first, walking up to the root; insertions at deeper
	// boundaries never disturb a shallower prefix, so this order is safe.
	var levels []level
	for c := p; c != nil; c = c.parent {
		if c.strat == nil {
			continue
		}
		if !c.strat.Known {
			return nil, false // runtime-input fan-out somewhere in the chain
		}
		levels = append(levels, level{prefix: c.fqn, suffixes: c.strat.Suffixes})
	}
	if len(levels) == 0 {
		return nil, false // no strategy in the chain: not an expanding ancestor
	}
	result := []string{p.fqn}
	for _, l := range levels {
		next := make([]string, 0, len(result)*len(l.suffixes))
		for _, base := range result {
			for _, suf := range l.suffixes {
				next = append(next, l.prefix+suf+base[len(l.prefix):])
			}
		}
		result = next
	}
	return result, true
}

// ancestorInstances enumerates the runtime instance FQNs of predecessor leaf p
// that target (resolved from rawTarget) must wait on. Target-aware counterpart of
// instanceFQNs: a strategy enclosing BOTH target and p (at or above their lowest
// common ancestor) does NOT multiply the dependency — shared levels are pinned to
// target's instance, and only strategies private to p (below the LCA) fan it out.
// Returns (nil, false) when p's chain has no strategy or a private one is not
// enumerable, so the ancestor gates on its logical FQN.
func (g *PipelineGraph) ancestorInstances(p, target *graphNode, rawTarget string) ([]string, bool) {
	lca := lowestCommonAncestor(p, target)

	// Strategy levels private to p: strictly below the LCA. These genuinely fan
	// the dependency out (each private instance of p must complete). Innermost
	// first, so insertions at deeper boundaries never disturb a shallower prefix.
	type level struct {
		prefix   string
		suffixes []string
	}
	var private []level
	for c := p; c != nil && c != lca; c = c.parent {
		if c.strat == nil {
			continue
		}
		if !c.strat.Known {
			return nil, false // runtime-input fan-out private to p: not enumerable
		}
		private = append(private, level{prefix: c.fqn, suffixes: c.strat.Suffixes})
	}

	// Any strategy at or above the LCA is shared between target and p: they are in
	// the same branch, so p's shared prefix is pinned to target's instance rather
	// than fanned over.
	shared := false
	for c := lca; c != nil; c = c.parent {
		if c.strat != nil {
			shared = true
			break
		}
	}

	if len(private) == 0 && !shared {
		return nil, false // no strategy anywhere in p's chain: not an expanding ancestor
	}

	// Fan the private levels out in the logical domain first (LCA and above stay
	// logical here), then swap in the pinned prefix below.
	result := []string{p.fqn}
	for _, l := range private {
		next := make([]string, 0, len(result)*len(l.suffixes))
		for _, base := range result {
			for _, suf := range l.suffixes {
				next = append(next, l.prefix+suf+base[len(l.prefix):])
			}
		}
		result = next
	}

	if !shared {
		return result, true
	}

	// Pin the shared prefix (LCA and above) to target's instance. If rawTarget
	// carries no instance suffix at that boundary (a logical target FQN, so
	// pinned == lca.fqn) or its segments don't align, target's branch is unknown —
	// fall back to the full, target-agnostic fan-out rather than gate on an
	// unnameable branch.
	pinned, ok := pinnedPrefix(rawTarget, target.fqn, lca.fqn)
	if !ok || pinned == lca.fqn {
		return instanceFQNs(p)
	}
	for i, base := range result {
		result[i] = pinned + base[len(lca.fqn):]
	}
	return result, true
}

// lowestCommonAncestor returns the deepest graph node that is an ancestor of both
// a and b (following parent links). Both descend from the shared root, so the
// result is never nil.
func lowestCommonAncestor(a, b *graphNode) *graphNode {
	seen := make(map[*graphNode]struct{})
	for c := a; c != nil; c = c.parent {
		seen[c] = struct{}{}
	}
	for c := b; c != nil; c = c.parent {
		if _, ok := seen[c]; ok {
			return c
		}
	}
	return nil
}

// pinnedPrefix returns the prefix of rawTarget (a possibly-runtime FQN) up to and
// including the segment corresponding to lcaFQN, carrying whatever strategy
// suffixes rawTarget holds at and above that boundary. targetLogical is
// rawTarget's logical FQN; the two align segment-for-segment because a fan-out
// suffix is appended within a '.'-separated segment and never adds one. Returns
// ok=false when the segments can't be aligned to lcaFQN.
func pinnedPrefix(rawTarget, targetLogical, lcaFQN string) (string, bool) {
	rawSegs := strings.Split(rawTarget, ".")
	logSegs := strings.Split(targetLogical, ".")
	if len(rawSegs) != len(logSegs) {
		return "", false
	}
	lcaSegs := strings.Count(lcaFQN, ".") + 1
	if lcaSegs > len(rawSegs) {
		return "", false
	}
	// The logical prefix at that boundary must be exactly lcaFQN, else rawTarget
	// and the LCA don't line up.
	if strings.Join(logSegs[:lcaSegs], ".") != lcaFQN {
		return "", false
	}
	return strings.Join(rawSegs[:lcaSegs], "."), true
}

// climb walks upward from node to its direct predecessor leaf steps, appending to
// conds the `when` guard of every container node is the entry (first child) of —
// a preceding sibling would otherwise already imply those guards held.
func (g *PipelineGraph) climb(node *graphNode, conds *[]When) []*graphNode {
	if node.kind == kindRollback {
		// Reached the rollback sub-DAG boundary. Its first step is failure-path
		// triggered, so gate it (OR, set in Resolve) on the stage's forward leaf
		// steps: a rollback may run only once some forward step has run — there
		// must be something to roll back. Never climb into forward ordering itself.
		return forwardLeaves(node.parent)
	}
	parent := node.parent
	if parent == nil {
		return nil // reached the pipeline root: no ancestors
	}
	if parent.concurrent {
		// Concurrent siblings share a predecessor: whatever precedes the
		// parallel block. The parallel wrapper carries no guard of its own.
		return g.climb(parent, conds)
	}
	if node.index > 0 {
		return terminalLeaves(parent.children[node.index-1])
	}
	// First unit in an ordered container: entering `parent` from outside, so
	// `parent`'s guard becomes relevant. Then continue from `parent`.
	if parent.when != nil {
		*conds = append(*conds, *parent.when)
	}
	return g.climb(parent, conds)
}

// terminalLeaves returns the leaf steps that complete last within n's subtree —
// i.e. the ones a following unit waits on. A parallel block joins on all
// branches; an ordered container joins on its last unit; a step (or an opaque
// container with no children) is its own terminal.
func terminalLeaves(n *graphNode) []*graphNode {
	if n.kind == kindStep || len(n.children) == 0 {
		return []*graphNode{n}
	}
	if n.concurrent {
		var out []*graphNode
		for _, c := range n.children {
			out = append(out, terminalLeaves(c)...)
		}
		return out
	}
	return terminalLeaves(n.children[len(n.children)-1])
}

// forwardLeaves returns every leaf step in stage's forward subtree, excluding the
// rollback sub-DAG. A rollback entry OR-gates on these. Nil when the stage has no
// forward step (leaving the entry ungated).
func forwardLeaves(stage *graphNode) []*graphNode {
	if stage == nil {
		return nil
	}
	var out []*graphNode
	var walk func(n *graphNode)
	walk = func(n *graphNode) {
		if n.kind == kindRollback {
			return // a nested rollback sub-DAG is not part of forward flow
		}
		if n.kind == kindStep {
			out = append(out, n)
			return
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, c := range stage.children {
		walk(c)
	}
	return out
}

// atRollbackEntry reports whether node is the entry unit of a stage's rollback
// sub-DAG: the first step reached by climbing without crossing a preceding
// ordered sibling. Mirrors the ascent in climb so the two stay consistent.
func atRollbackEntry(node *graphNode) bool {
	for c := node; ; {
		if c.kind == kindRollback {
			return true
		}
		parent := c.parent
		if parent == nil {
			return false // reached the pipeline root: a forward step, not rollback
		}
		if parent.concurrent {
			c = parent // concurrent siblings share the predecessor; keep climbing
			continue
		}
		if c.index > 0 {
			return false // a preceding sibling gates c, not the rollback trigger
		}
		c = parent // first unit of an ordered container; keep climbing
	}
}

// rollbackContainer returns the kindRollback ancestor of node (the stage's
// rollbackSteps root), or nil if node is not inside a rollback sub-DAG. Its
// parent is the owning stage node.
func rollbackContainer(node *graphNode) *graphNode {
	for c := node; c != nil; c = c.parent {
		if c.kind == kindRollback {
			return c
		}
	}
	return nil
}

// hasStrategyInChain reports whether n or any of its ancestors declares a
// strategy, meaning n's runtime FQN may carry "_<index>" suffixes.
func hasStrategyInChain(n *graphNode) bool {
	for c := n; c != nil; c = c.parent {
		if c.strategy {
			return true
		}
	}
	return false
}

// strategyStatus explains why n's fan-out could not be enumerated: the status of
// the innermost non-enumerable strategy in n's container chain. Every such status
// is benign (runtime_input or custom_names) — malformed shapes never build — so
// there is no severity ordering to apply; the innermost cause is the most specific.
// Returns FanoutRuntimeInput as a fail-closed default if a non-enumerable strategy
// somehow carries no status, and FanoutEnumerable when nothing is non-enumerable.
func strategyStatus(n *graphNode) FanoutStatus {
	for c := n; c != nil; c = c.parent {
		if c.strat == nil || c.strat.Known || c.strat.excluded {
			continue
		}
		if c.strat.status == FanoutEnumerable || c.strat.status == "" {
			return FanoutRuntimeInput // non-enumerable but unclassified: fail closed on the benign default
		}
		return c.strat.status
	}
	return FanoutEnumerable
}

// strategyExcluded reports whether any strategy in n's container chain excludes
// every combination, so n is skipped at runtime (never runs, gates nothing).
func strategyExcluded(n *graphNode) bool {
	for c := n; c != nil; c = c.parent {
		if c.strat != nil && c.strat.excluded {
			return true
		}
	}
	return false
}

// register records a node by its logical FQN, tracking collisions. Two nodes
// with the same FQN mean duplicate identifiers within one scope — a malformed
// pipeline (Harness requires unique identifiers per scope). Any collision makes
// BuildGraph fail closed with ErrInvalidPipeline.
func (g *PipelineGraph) register(n *graphNode) {
	if _, exists := g.byFQN[n.fqn]; exists {
		g.duplicates = append(g.duplicates, n.fqn)
	}
	g.byFQN[n.fqn] = n
}

// TreeNode is a read-only view of the ordering graph's nesting structure, for
// display or traversal. Children are in execution order; a "parallel" node
// groups concurrent siblings.
type TreeNode struct {
	Kind     string // pipeline | stage | stepGroup | step | parallel
	ID       string // identifier ("" for pipeline/parallel)
	FQN      string // logical FQN ("" for pipeline/parallel)
	Type     string // step/stage `type` ("" otherwise)
	Children []TreeNode
}

func (k nodeKind) String() string {
	switch k {
	case kindPipeline:
		return "pipeline"
	case kindStage:
		return "stage"
	case kindStepGroup:
		return "stepGroup"
	case kindStep:
		return "step"
	case kindParallel:
		return "parallel"
	case kindRollback:
		return "rollback"
	default:
		return "unknown"
	}
}

// Tree returns the root of a read-only structural view of the graph, preserving
// nesting and execution order (parallel blocks included as grouping nodes).
func (g *PipelineGraph) Tree() TreeNode {
	if g == nil {
		return TreeNode{}
	}
	return toTree(g.root)
}

func toTree(n *graphNode) TreeNode {
	t := TreeNode{Kind: n.kind.String(), ID: n.id, FQN: n.fqn, Type: n.stepType}
	for _, c := range n.children {
		t.Children = append(t.Children, toTree(c))
	}
	return t
}

// FQNs returns every logical FQN in the graph (steps, stepGroups and stages),
// sorted. Parallel wrappers are omitted as they carry no FQN of their own.
func (g *PipelineGraph) FQNs() []string {
	if g == nil {
		return nil
	}
	out := make([]string, 0, len(g.byFQN))
	for fqn := range g.byFQN {
		out = append(out, fqn)
	}
	sort.Strings(out)
	return out
}

// lookup resolves an FQN (logical or a runtime FQN carrying strategy "_<n>"
// index suffixes) to its logical node. It is graph-guided: a "_<n>" suffix is
// only stripped on a segment whose logical node actually declares a strategy,
// so a suffix on a non-strategy step does not silently resolve, and a literal
// identifier ending in "_<digits>" that genuinely exists is matched as-is.
func (g *PipelineGraph) lookup(fqn string) *graphNode {
	if n, ok := g.byFQN[fqn]; ok {
		return n // exact logical (or literal) FQN
	}
	if n, ok := g.instByFQN[fqn]; ok {
		return n // enumerated runtime instance FQN (index or value naming)
	}
	// Walk segment by segment, matching node boundaries against byFQN. Keyword
	// segments (spec/execution/steps/...) don't match a node and are absorbed
	// into the next identifier's boundary.
	segs := strings.Split(fqn, ".")
	var cand string
	var last *graphNode
	for _, s := range segs {
		if cand == "" {
			cand = s
		} else {
			cand += "." + s
		}
		if n, ok := g.byFQN[cand]; ok {
			last = n
			continue
		}
		if strings.IndexByte(s, '_') > 0 {
			// A runtime instance segment appends fan-out suffixes to its logical id
			// (indices "build_0_0" or axis values "cd_e2_i1_s1"). Strip trailing
			// "_<token>" groups longest-first; accept the first whose logical node
			// declares a fan-out.
			prefix := cand[:len(cand)-len(s)]
			for seg := s; ; {
				j := strings.LastIndexByte(seg, '_')
				if j <= 0 {
					break
				}
				seg = seg[:j]
				if n, ok := g.byFQN[prefix+seg]; ok && n.strategy {
					cand, last = prefix+seg, n
					break
				}
			}
		}
	}
	if last != nil && last.fqn == cand {
		return last
	}
	return nil
}

// MatchesAncestor reports whether a pool step (by FQN, possibly a runtime FQN
// with strategy suffixes) satisfies the given direct-ancestor requirement. It
// resolves the pool FQN, so an instance ("deploy_2") matches its logical ancestor
// ("deploy") only when "deploy" declares a strategy.
func (g *PipelineGraph) MatchesAncestor(poolFQN string, a Ancestor) bool {
	n := g.lookup(poolFQN)
	return n != nil && n.fqn == a.FQN
}
