package http_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	httptransport "github.com/SPROG666/go-autumn-2026/internal/http"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

func TestPositionRouterValidation(t *testing.T) {
	path := "/api/v1/trips/00000000-0000-0000-0000-000000000001/positions"
	for _, body := range []string{
		`{}`, `{"recorded_at":"2026-11-16T12:00:00Z"}`, `{"coordinates":{"latitude":0,"longitude":0}}`,
		`{"coordinates":{"latitude":91,"longitude":0},"recorded_at":"2026-11-16T12:00:00Z"}`,
		`{"coordinates":{"latitude":0,"longitude":181},"recorded_at":"2026-11-16T12:00:00Z"}`,
		`{"coordinates":{"latitude":0},"recorded_at":"2026-11-16T12:00:00Z"}`,
		`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"bad"}`,
	} {
		server := NewMockServerInterface(gomock.NewController(t))
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		router, err := httptransport.NewRouter(logger, server, response.New(logger))
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
			t.Fatalf("request=%s response=%d %s", body, w.Code, w.Body.String())
		}
	}
}
