package domainerror_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{trip.ErrTripNotFound, 404, "trip_not_found"},
		{trip.ErrTripCompleted, 409, "trip_completed"},
		{trip.ErrDriverBusy, 409, "driver_busy"},
		{trip.ErrIdempotencyConflict, 409, "idempotency_conflict"},
		{&trip.ValidationError{Field: "price", Reason: "must be non-negative"}, 400, "invalid_request"},
		{errors.New("password=secret SQL failure"), 500, "internal_error"},
	} {
		t.Run(test.code, func(t *testing.T) {
			status, body := domainerror.Problem(fmt.Errorf("wrapped: %w", test.err))
			if status != test.status || int(body.Status) != status || body.Code != test.code || body.Title == "" || body.Detail == nil || !strings.HasPrefix(body.Type, "https://tripgo.example/problems/") {
				t.Fatalf("Problem=%d %+v", status, body)
			}
			if strings.Contains(*body.Detail, "secret") || strings.Contains(*body.Detail, "wrapped") {
				t.Fatal("internal error leaked", *body.Detail)
			}
		})
	}
}
