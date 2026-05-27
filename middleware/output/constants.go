// Package output ships the first-party zts.OutputMiddleware constructors
// used by the ZTS SDK: Logging, Metrics, and Audit. Each constructor
// returns a zts.OutputMiddleware so customers can compose them with their
// own middlewares via zts.WithOutputMiddleware.
package output

const (
	statusSuccess = "success"
	statusError   = "error"

	keyStatus    = "status"
	keyAccountID = "account_id"
)
