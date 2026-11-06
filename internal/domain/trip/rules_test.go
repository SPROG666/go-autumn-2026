package trip_test

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

func TestFinish(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 123456789, time.FixedZone("test", 3600))
	for _, status := range []string{trip.StatusActive, trip.StatusCompleted, "unknown"} {
		t.Run(status, func(t *testing.T) {
			value := trip.Trip{Status: status, UpdatedAt: at.Add(-time.Hour)}
			before := value
			err := value.Finish(at)
			if status != trip.StatusActive {
				if !errors.Is(err, trip.ErrTripCompleted) || !reflect.DeepEqual(value, before) {
					t.Fatalf("Finish changed rejected trip: %+v, %v", value, err)
				}
				return
			}
			want := at.UTC().Truncate(time.Microsecond)
			if err != nil || value.Status != trip.StatusCompleted || value.FinishedAt == nil || *value.FinishedAt != want || value.UpdatedAt != want {
				t.Fatalf("Finish = %+v, %v", value, err)
			}
			if err := value.Finish(at.Add(time.Hour)); !errors.Is(err, trip.ErrTripCompleted) || *value.FinishedAt != want {
				t.Fatal("second finish changed timestamp", err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	const (
		startLatitudeField = "start_point.latitude"
		endLongitudeField  = "end_point.longitude"
	)
	valid := trip.CreateParams{UserID: uuid.New(), DriverID: uuid.New(), StartLatitude: -90, StartLongitude: 180, EndLatitude: 90, EndLongitude: -180}
	for _, test := range []struct {
		name, field string
		change      func(*trip.CreateParams)
	}{
		{"valid", "", func(*trip.CreateParams) {}},
		{"zero coordinates", "", func(p *trip.CreateParams) {
			p.StartLatitude, p.StartLongitude, p.EndLatitude, p.EndLongitude = 0, 0, 0, 0
		}},
		{"opposite boundaries", "", func(p *trip.CreateParams) {
			p.StartLatitude, p.StartLongitude, p.EndLatitude, p.EndLongitude = 90, -180, -90, 180
		}},
		{"user", "user_id", func(p *trip.CreateParams) { p.UserID = uuid.Nil }},
		{"driver", "driver_id", func(p *trip.CreateParams) { p.DriverID = uuid.Nil }},
		{"latitude", startLatitudeField, func(p *trip.CreateParams) { p.StartLatitude = -90.1 }},
		{"latitude above maximum", startLatitudeField, func(p *trip.CreateParams) { p.StartLatitude = 90.1 }},
		{"longitude", "start_point.longitude", func(p *trip.CreateParams) { p.StartLongitude = 180.1 }},
		{"longitude below minimum", "start_point.longitude", func(p *trip.CreateParams) { p.StartLongitude = -180.1 }},
		{"end latitude", "end_point.latitude", func(p *trip.CreateParams) { p.EndLatitude = 91 }},
		{"end latitude below minimum", "end_point.latitude", func(p *trip.CreateParams) { p.EndLatitude = -91 }},
		{"end longitude", endLongitudeField, func(p *trip.CreateParams) { p.EndLongitude = -181 }},
		{"end longitude above maximum", endLongitudeField, func(p *trip.CreateParams) { p.EndLongitude = 181 }},
		{"nan", startLatitudeField, func(p *trip.CreateParams) { p.StartLatitude = math.NaN() }},
		{"infinity", endLongitudeField, func(p *trip.CreateParams) { p.EndLongitude = math.Inf(1) }},
		{"negative infinity", startLatitudeField, func(p *trip.CreateParams) { p.StartLatitude = math.Inf(-1) }},
		{"price", "price", func(p *trip.CreateParams) { p.Price = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			params := valid
			test.change(&params)
			err := params.Validate()
			if test.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validation *trip.ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field || validation.Reason == "" || validation.Error() == "" {
				t.Fatalf("Validate = %v; want %s", err, test.field)
			}
		})
	}
}
