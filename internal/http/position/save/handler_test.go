package save_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/http/position/save"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

func TestCreatePosition(t *testing.T) {
	valid := `{"coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-16T12:00:00Z"}`
	for _, test := range []struct {
		name, body string
		failure    error
		status     int
	}{
		{name: "saved", body: valid, status: 201},
		{name: "malformed", body: `{`, status: 400},
		{name: "extra JSON", body: valid + ` {}`, status: 400},
		{name: "unknown field", body: `{"unexpected":true}`, status: 400},
		{name: "timestamp", body: `{"recorded_at":"bad"}`, status: 400},
		{name: "missing trip", body: valid, failure: trip.ErrTripNotFound, status: 404},
		{name: "completed", body: valid, failure: trip.ErrTripCompleted, status: 409},
		{name: "future", body: valid, failure: &trip.ValidationError{Field: "recorded_at", Reason: "future"}, status: 400},
		{name: "internal", body: valid, failure: errors.New("private database detail"), status: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc, queue := NewMockusecase(ctrl), NewMockcanceller(ctrl)
			id := uuid.New()
			value := position.Position{ID: 42, TripID: id, Latitude: 59.93, Longitude: 30.33, RecordedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}
			if test.body == valid {
				input := value
				input.ID = 0
				call := uc.EXPECT().Execute(gomock.Any(), input).Return(value, test.failure)
				if test.failure == nil {
					queue.EXPECT().Cancel(id).After(call)
				}
			}
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			handler := save.New(uc, queue, response.New(logger))
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader(test.body))
			handler.CreateTripPosition(w, request, id)
			if w.Code != test.status {
				t.Fatalf("response=%d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private database detail") {
				t.Fatal("internal detail leaked")
			}
			if test.status == 201 {
				var body api.TripPosition
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Id != 42 || body.TripId != id {
					t.Fatalf("body=%+v err=%v", body, err)
				}
			} else if w.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatal("wrong error format")
			}
		})
	}
}
