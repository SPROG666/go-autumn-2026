package push_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/goleak"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/gateway/push"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestRequestPosition(t *testing.T) {
	for _, status := range []int{202, 400, 429, 500, 503, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			value := position.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/api/v1/push" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				var body struct {
					RequestID   string            `json:"request_id"`
					RecipientID string            `json:"recipient_id"`
					Kind        string            `json:"kind"`
					Data        map[string]string `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.RequestID == "" || body.RecipientID != value.DriverID.String() || body.Kind != "REQUEST_POSITION" || body.Data["trip_id"] != value.ID.String() {
					t.Errorf("body=%+v", body)
				}
				if status == 302 {
					w.Header().Set("Location", "/redirect")
				}
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)
			gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(gateway.Close)
			err = gateway.RequestPosition(context.Background(), value)
			if status == 202 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var response *push.HTTPError
				if !errors.As(err, &response) || response.StatusCode != status {
					t.Fatalf("error=%v", err)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("calls=%d", calls.Load())
			}
		})
	}
}

func TestTransportDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Consume the request so net/http can observe the client's disconnect.
		var body any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	gateway, err := push.New(server.URL, 50*time.Millisecond, noop.NewTracerProvider())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gateway.Close)
	err = gateway.RequestPosition(context.Background(), position.StaleTrip{ID: uuid.New(), DriverID: uuid.New()})
	var transport *push.TransportError
	if !errors.As(err, &transport) {
		t.Fatalf("error=%v", err)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, address := range []string{"", ":bad", "ftp://localhost", "http://user:secret@localhost", "http://localhost?token=x"} {
		if _, err := push.New(address, time.Second, noop.NewTracerProvider()); err == nil {
			t.Fatal("accepted invalid URL")
		}
	}
	if _, err := push.New("http://localhost", 0, noop.NewTracerProvider()); err == nil {
		t.Fatal("accepted zero timeout")
	}
}
