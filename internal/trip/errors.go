package trip

import "errors"

var (
	ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")
	ErrNotFound            = errors.New("trip not found")
	ErrDriverBusy          = errors.New("driver already has an active trip")
	ErrCompleted           = errors.New("trip already completed")
)
