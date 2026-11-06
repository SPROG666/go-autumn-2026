package get_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
	"github.com/SPROG666/go-autumn-2026/internal/http/trip/get"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

func TestHandler(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"success", nil, 200, ""},
		{"internal", errors.New("SQL password=secret"), 500, "internal_error"},
		{"missing", trip.ErrTripNotFound, 404, "trip_not_found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := NewMockusecase(gomock.NewController(t))
			writer := response.New(slog.New(slog.NewJSONHandler(io.Discard, nil)))
			handler := get.New(repo, writer)
			id, user, driver := uuid.New(), uuid.New(), uuid.New()
			value := model.Trip{ID: id, UserID: user, DriverID: driver, Status: trip.StatusActive, Price: 100}
			body := ""
			repo.EXPECT().Execute(gomock.Any(), id).Return(value, test.err)
			r := httptest.NewRequest("POST", "/api/v1/trips/"+id.String(), strings.NewReader(body))
			w := httptest.NewRecorder()
			handler.GetTrip(w, r, id)
			if w.Code != test.status {
				t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
			}
			if test.err != nil {
				var problem api.Problem
				if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != test.code || int(problem.Status) != test.status || w.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatalf("problem=%+v", problem)
				}
				if strings.Contains(w.Body.String(), "password") {
					t.Fatal("leaked internal error")
				}
			} else {
				var got api.Trip
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Id != id || got.UserId != user || got.DriverId != driver || got.Price != 100 {
					t.Fatalf("trip=%+v", got)
				}
			}
		})
	}
}
