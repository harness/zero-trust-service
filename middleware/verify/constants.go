// Package verify ships the first-party zts.VerifyMiddleware constructors
// used by the ZTS SDK: Logging, Metrics, Audit, and MissingMetadata. Each
// constructor returns a zts.VerifyMiddleware so customers can compose
// them with their own middlewares via zts.WithVerifyMiddleware.
package verify

const (
	statusAuthorized   = "authorized"
	statusUnauthorized = "unauthorized"
	statusError        = "error"

	keyStatus    = "status"
	keyAccountID = "account_id"
	keyField     = "field"

	fieldZTSMetadata = "zts_metadata"
	fieldAccountID   = "account_id"
	fieldTaskType    = "task_type"
)
