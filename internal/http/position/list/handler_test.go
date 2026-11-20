package list_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/http/position/list"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

func TestListPositions(t *testing.T) {
	for _, test := range []struct {
		name     string
		values   []position.Position
		err      error
		status   int
		contains string
	}{
		{name: "empty route", status: 200, contains: `"positions":[]`},
		{name: "route", values: []position.Position{{ID: 42}}, status: 200, contains: `"id":42`},
		{name: "missing", err: trip.ErrTripNotFound, status: 404, contains: `"code":"trip_not_found"`},
		{name: "internal", err: errors.New("private database detail"), status: 500, contains: `"code":"internal_error"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockusecase(ctrl)
			id := uuid.New()
			uc.EXPECT().Execute(gomock.Any(), id).Return(test.values, test.err)
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			handler := list.New(uc, response.New(logger))
			w := httptest.NewRecorder()
			handler.ListTripPositions(w, httptest.NewRequest(http.MethodGet, "/positions", nil), id)
			if w.Code != test.status || !strings.Contains(w.Body.String(), test.contains) || strings.Contains(w.Body.String(), "private database detail") {
				t.Fatalf("response=%d %s", w.Code, w.Body.String())
			}
		})
	}
}
