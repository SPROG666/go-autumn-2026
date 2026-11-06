package trip

import "errors"

var (
	ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")
	ErrTripNotFound        = errors.New("trip not found")
	ErrDriverBusy          = errors.New("driver already has an active trip")
	ErrTripCompleted       = errors.New("trip already completed")
)
