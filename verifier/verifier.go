package verifier

import zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"

type Interface interface {
	Handle(request zts.VerifyRequest) error
}

type middlewareFunc func(request zts.VerifyRequest) error

func (f middlewareFunc) Handle(request zts.VerifyRequest) error {
	return f(request)
}

func From(f func(request zts.VerifyRequest) error) Interface {
	return middlewareFunc(f)
}
