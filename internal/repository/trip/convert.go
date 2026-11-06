package trip

import "github.com/SPROG666/go-autumn-2026/internal/domain/trip"

func normalize(value trip.Trip) trip.Trip {
	value.StartedAt = value.StartedAt.UTC()
	value.CreatedAt = value.CreatedAt.UTC()
	value.UpdatedAt = value.UpdatedAt.UTC()
	if value.FinishedAt != nil {
		at := value.FinishedAt.UTC()
		value.FinishedAt = &at
	}
	return value
}
