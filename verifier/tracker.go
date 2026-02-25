package verifier

import (
	"context"
	"sync"
)

type ctxKey struct{}

// WithTracker returns a new context carrying the given tracker.
func WithTracker(ctx context.Context, t *Tracker) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// TrackerFrom extracts the tracker from the context, or nil if absent.
func TrackerFrom(ctx context.Context) *Tracker {
	t, _ := ctx.Value(ctxKey{}).(*Tracker)
	return t
}

// Tracker collects the names of validators that ran during a verify request.
// Thread-safe — validators may run concurrently in future.
type Tracker struct {
	mu              sync.Mutex
	validatorsRun   []string
	failedValidator string
}

// NewTracker creates a new empty tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// Record adds a validator name and its pass/fail result.
func (t *Tracker) Record(name string, failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validatorsRun = append(t.validatorsRun, name)
	if failed && t.failedValidator == "" {
		t.failedValidator = name
	}
}

// Results returns the list of validators run and the first validator that failed.
func (t *Tracker) Results() (validatorsRun []string, failedValidator string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.validatorsRun, t.failedValidator
}
