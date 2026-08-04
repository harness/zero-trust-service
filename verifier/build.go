package verifier

// WrapFunc decorates a verifier (e.g. with metrics / tracking).
type WrapFunc func(name string, v Interface) Interface
