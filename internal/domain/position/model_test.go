package position_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

func TestValidate(t *testing.T) {
	const latitude = "latitude"
	const longitude = "longitude"
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		change func(*position.Position)
		field  string
	}{
		{name: "valid past", change: func(p *position.Position) { p.RecordedAt = now.Add(-time.Hour) }},
		{name: "one minute boundary", change: func(p *position.Position) { p.RecordedAt = now.Add(time.Minute); p.Latitude = -90; p.Longitude = 180 }},
		{name: "missing trip", change: func(p *position.Position) { p.TripID = uuid.Nil }, field: "trip_id"},
		{name: latitude, change: func(p *position.Position) { p.Latitude = 91 }, field: latitude},
		{name: longitude, change: func(p *position.Position) { p.Longitude = -181 }, field: longitude},
		{name: "NaN", change: func(p *position.Position) { p.Latitude = math.NaN() }, field: latitude},
		{name: "infinity", change: func(p *position.Position) { p.Longitude = math.Inf(1) }, field: longitude},
		{name: "missing time", change: func(p *position.Position) { p.RecordedAt = time.Time{} }, field: "recorded_at"},
		{name: "future", change: func(p *position.Position) { p.RecordedAt = now.Add(time.Minute + time.Nanosecond) }, field: "recorded_at"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := position.Position{TripID: uuid.New(), RecordedAt: now}
			test.change(&p)
			err := p.Validate(now)
			if test.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validation *trip.ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("validation=%v", err)
			}
		})
	}
}
