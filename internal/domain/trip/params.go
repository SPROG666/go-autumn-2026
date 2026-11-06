package trip

import "github.com/google/uuid"

type CreateParams struct {
	UserID         uuid.UUID `validate:"required" field:"user_id"`
	DriverID       uuid.UUID `validate:"required" field:"driver_id"`
	StartLatitude  float64   `validate:"gte=-90,lte=90" field:"start_point.latitude"`
	StartLongitude float64   `validate:"gte=-180,lte=180" field:"start_point.longitude"`
	EndLatitude    float64   `validate:"gte=-90,lte=90" field:"end_point.latitude"`
	EndLongitude   float64   `validate:"gte=-180,lte=180" field:"end_point.longitude"`
	Price          int64     `validate:"gte=0" field:"price"`
}
