package verifier

import "context"

type ctxKey struct{}

func WithTracker(ctx context.Context, t *Tracker) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

func TrackerFrom(ctx context.Context) *Tracker {
	t, _ := ctx.Value(ctxKey{}).(*Tracker)
	return t
}

// Tracker collects per-request validator results.
type Tracker struct {
	validatorsRun   []string
	failedValidator string
}

func NewTracker() *Tracker {
	return &Tracker{}
}

func (t *Tracker) Record(name string, failed bool) {
	t.validatorsRun = append(t.validatorsRun, name)
	if failed && t.failedValidator == "" {
		t.failedValidator = name
	}
}

func (t *Tracker) Results() (validatorsRun []string, failedValidator string) {
	return t.validatorsRun, t.failedValidator
}
