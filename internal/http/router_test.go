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

func TestRouterValidation(t *testing.T) {
	const tripsPath = "/api/v1/trips"
	const invalidRequest = "invalid_request"
	for _, test := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"missing fields", http.MethodPost, tripsPath, `{}`, 400, invalidRequest},
		{"bad uuid", "GET", "/api/v1/trips/no-uuid", "", 400, invalidRequest},
		{"finish body", http.MethodPost, "/api/v1/trips/00000000-0000-0000-0000-000000000001/finish", `{}`, 400, invalidRequest},
		{"unknown route", "GET", "/missing", "", 404, "route_not_found"},
		{"method", "DELETE", tripsPath, "", 405, "method_not_allowed"},
		{"coordinate range", http.MethodPost, tripsPath, `{"user_id":"00000000-0000-0000-0000-000000000001","driver_id":"00000000-0000-0000-0000-000000000002","start_point":{"latitude":91,"longitude":0},"end_point":{"latitude":0,"longitude":0},"price":0}`, 400, invalidRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := NewMockServerInterface(gomock.NewController(t))
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			router, err := httptransport.NewRouter(logger, server, response.New(logger))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != test.status || !strings.Contains(w.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("response=%d %s", w.Code, w.Body.String())
			}
			if test.status == http.StatusMethodNotAllowed && w.Header().Get("Allow") != "POST" {
				t.Fatal("missing Allow header", w.Header())
			}
		})
	}
}
