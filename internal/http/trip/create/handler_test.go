package create_test

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
	"github.com/SPROG666/go-autumn-2026/internal/http/trip/create"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

func TestHandler(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"success", nil, 201, ""},
		{"internal", errors.New("SQL password=secret"), 500, "internal_error"},
		{"busy", trip.ErrDriverBusy, 409, "driver_busy"},
		{"key conflict", trip.ErrIdempotencyConflict, 409, "idempotency_conflict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := NewMockusecase(gomock.NewController(t))
			writer := response.New(slog.New(slog.NewJSONHandler(io.Discard, nil)))
			handler := create.New(repo, writer)
			id, user, driver := uuid.New(), uuid.New(), uuid.New()
			value := model.Trip{ID: id, UserID: user, DriverID: driver, Status: trip.StatusActive, Price: 100}
			params := model.CreateParams{UserID: user, DriverID: driver, StartLatitude: 10, StartLongitude: 20, EndLatitude: 30, EndLongitude: 40, Price: 100}
			body := `{"user_id":"` + user.String() + `","driver_id":"` + driver.String() + `","start_point":{"latitude":10,"longitude":20},"end_point":{"latitude":30,"longitude":40},"price":100}`
			repo.EXPECT().Execute(gomock.Any(), params, nil).Return(value, false, test.err)
			r := httptest.NewRequest("POST", "/api/v1/trips/"+id.String(), strings.NewReader(body))
			w := httptest.NewRecorder()
			handler.CreateTrip(w, r, api.CreateTripParams{})
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

func TestInvalidBody(t *testing.T) {
	for _, body := range []string{"", `{`, `{"unexpected":1}`, `{} {}`, `{"price":"invalid"}`} {
		t.Run(body, func(t *testing.T) {
			repo := NewMockusecase(gomock.NewController(t))
			handler := create.New(repo, response.New(slog.New(slog.NewJSONHandler(io.Discard, nil))))
			w := httptest.NewRecorder()
			handler.CreateTrip(w, httptest.NewRequest("POST", "/api/v1/trips", strings.NewReader(body)), api.CreateTripParams{})
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockusecase(ctrl)
	key, id := uuid.New(), uuid.New()
	repo.EXPECT().Execute(gomock.Any(), gomock.Any(), &key).Return(model.Trip{ID: id, Status: trip.StatusCompleted}, true, nil)
	handler := create.New(repo, response.New(slog.New(slog.NewJSONHandler(io.Discard, nil))))
	w := httptest.NewRecorder()
	handler.CreateTrip(w, httptest.NewRequest("POST", "/api/v1/trips", strings.NewReader(`{}`)), api.CreateTripParams{IdempotencyKey: &key})
	if w.Code != 200 || w.Header().Get("Location") != "/api/v1/trips/"+id.String() || !strings.Contains(w.Body.String(), "completed") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
