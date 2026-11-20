package position

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

func (p Position) Validate(now time.Time) error {
	switch {
	case p.TripID == uuid.Nil:
		return &trip.ValidationError{Field: "trip_id", Reason: "must be a non-zero UUID"}
	case math.IsNaN(p.Latitude) || math.IsInf(p.Latitude, 0) || p.Latitude < -90 || p.Latitude > 90:
		return &trip.ValidationError{Field: "latitude", Reason: "out of range"}
	case math.IsNaN(p.Longitude) || math.IsInf(p.Longitude, 0) || p.Longitude < -180 || p.Longitude > 180:
		return &trip.ValidationError{Field: "longitude", Reason: "out of range"}
	case p.RecordedAt.IsZero() || p.RecordedAt.After(now.Add(time.Minute)):
		return &trip.ValidationError{Field: "recorded_at", Reason: "must be set and no more than one minute in the future"}
	}
	return nil
}

type StaleTrip struct {
	ID       uuid.UUID
	DriverID uuid.UUID
}
