package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("push HTTP status %d", e.StatusCode) }

type TransportError struct{ Err error }

func (e *TransportError) Error() string { return fmt.Sprintf("push transport: %v", e.Err) }
func (e *TransportError) Unwrap() error { return e.Err }

type Gateway struct {
	url       string
	client    *http.Client
	transport *http.Transport
}

func New(baseURL string, timeout time.Duration, provider trace.TracerProvider) (*Gateway, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("push base URL must be an absolute HTTP URL without credentials, query or fragment")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("push timeout must be positive")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	transport.TLSHandshakeTimeout = timeout
	transport.IdleConnTimeout = timeout
	transport.MaxIdleConnsPerHost = 100
	client := &http.Client{
		Timeout: timeout, Transport: otelhttp.NewTransport(transport, otelhttp.WithTracerProvider(provider)),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Gateway{url: strings.TrimRight(baseURL, "/") + "/api/v1/push", client: client, transport: transport}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, value position.StaleTrip) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate push request ID: %w", err)
	}
	body, err := json.Marshal(struct {
		RequestID   string            `json:"request_id"`
		RecipientID uuid.UUID         `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}{RequestID: id.String(), RecipientID: value.DriverID, Kind: "REQUEST_POSITION", Data: map[string]string{"trip_id": value.ID.String()}})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(request)
	if err != nil {
		return &TransportError{Err: err}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return &HTTPError{StatusCode: response.StatusCode}
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10)); err != nil {
		return &TransportError{Err: err}
	}
	return nil
}

func (g *Gateway) Close() { g.transport.CloseIdleConnections() }
