package verifier

const (
	metricVerifiersRegistered = "zts_verifiers_registered"

	scopeGlobal   = "global"
	scopeTaskType = "task_type"
	scopeCustom   = "custom"

	keyScope = "scope"
)

// WrapFunc decorates a verifier (e.g. with metrics / tracking).
type WrapFunc func(name string, v Interface) Interface
