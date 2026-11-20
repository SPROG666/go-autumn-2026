package positions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/worker/positions"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

type manualTicker struct {
	ticks   chan time.Time
	stopped chan struct{}
}

func (t *manualTicker) C() <-chan time.Time { return t.ticks }
func (t *manualTicker) Stop()               { close(t.stopped) }

type fixture struct {
	pool     *positions.Pool
	repo     *MocktripRepository
	push     *Mockgateway
	observer *Mockobserver
	ticker   *manualTicker
	checks   chan string
	requests chan string
}

func newFixture(t *testing.T, workers, size int) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &fixture{
		repo: NewMocktripRepository(ctrl), push: NewMockgateway(ctrl), observer: NewMockobserver(ctrl),
		ticker: &manualTicker{ticks: make(chan time.Time), stopped: make(chan struct{})}, checks: make(chan string, 100), requests: make(chan string, 100),
	}
	f.observer.EXPECT().Queue(gomock.Any(), gomock.Any()).AnyTimes()
	f.observer.EXPECT().Active(gomock.Any(), gomock.Any()).AnyTimes()
	f.observer.EXPECT().Check(gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string) { f.checks <- result }).AnyTimes()
	f.observer.EXPECT().Requested(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string, _ time.Duration) { f.requests <- result }).AnyTimes()
	f.pool = positions.New(positions.Config{Workers: workers, QueueSize: size, StaleAfter: time.Minute, RequestTimeout: time.Minute},
		f.repo, f.push, f.ticker, f.observer, slog.New(slog.NewJSONHandler(io.Discard, nil)), noop.NewTracerProvider().Tracer("test"))
	return f
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("synchronization timed out")
		var zero T
		return zero
	}
}

func stale() position.StaleTrip { return position.StaleTrip{ID: uuid.New(), DriverID: uuid.New()} }
func (f *fixture) tick(t *testing.T, trips []position.StaleTrip, results ...string) {
	t.Helper()
	f.repo.EXPECT().Stale(gomock.Any(), time.Minute).Return(trips, nil)
	f.ticker.ticks <- time.Now()
	for _, want := range results {
		if got := receive(t, f.checks); got != want {
			t.Fatalf("check=%s want=%s", got, want)
		}
	}
}

func (f *fixture) start(t *testing.T, ctx context.Context) {
	t.Helper()
	f.pool.Start(ctx)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := f.pool.Stop(cleanup); err != nil {
			t.Error(err)
		}
	})
}

func TestDuplicateCancelAndOverflow(t *testing.T) {
	f := newFixture(t, 1, 1)
	a, b, c := stale(), stale(), stale()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), a).DoAndReturn(func(ctx context.Context, _ position.StaleTrip) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("missing task deadline")
		}
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	f.observer.EXPECT().Dropped(gomock.Any())
	f.start(t, context.Background())
	f.tick(t, []position.StaleTrip{a}, "queued")
	receive(t, started)
	f.tick(t, []position.StaleTrip{a, b, b, c}, "duplicate", "queued", "duplicate", "dropped")
	f.pool.Cancel(b.ID)
	close(release)
	if got := receive(t, f.requests); got != "success" {
		t.Fatal(got)
	}
	if err := f.pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	receive(t, f.ticker.stopped)
	// No expectation for b: processing a canceled channel entry must not push.
}

func TestFixedWorkersDrainAndRecoverAfterError(t *testing.T) {
	f := newFixture(t, 2, 10)
	trips := make([]position.StaleTrip, 8)
	results := make([]string, len(trips))
	for i := range trips {
		trips[i], results[i] = stale(), "queued"
	}
	started := make(chan struct{}, len(trips))
	release := make(chan struct{})
	var active, maximum, calls atomic.Int64
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).Times(len(trips)).DoAndReturn(func(ctx context.Context, _ position.StaleTrip) error {
		n := active.Add(1)
		defer active.Add(-1)
		for current := maximum.Load(); n > current; current = maximum.Load() {
			if maximum.CompareAndSwap(current, n) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		if calls.Add(1) == 1 {
			return errors.New("temporary push failure")
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.start(t, ctx)
	f.tick(t, trips, results...)
	receive(t, started)
	receive(t, started)
	if maximum.Load() != 2 {
		t.Fatal("workers not running concurrently")
	}
	cancel() // stop the scanner, but let accepted jobs finish normally.
	close(release)
	if err := f.pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	errorsSeen := 0
	for range trips {
		if receive(t, f.requests) == "error" {
			errorsSeen++
		}
	}
	if calls.Load() != int64(len(trips)) || maximum.Load() != 2 || errorsSeen != 1 {
		t.Fatalf("calls=%d max=%d errors=%d", calls.Load(), maximum.Load(), errorsSeen)
	}
}

func TestShutdownDeadlineCancelsInFlightAndQueued(t *testing.T) {
	f := newFixture(t, 1, 2)
	a, b := stale(), stale()
	started := make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), a).DoAndReturn(func(ctx context.Context, _ position.StaleTrip) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	f.start(t, context.Background())
	f.tick(t, []position.StaleTrip{a}, "queued")
	receive(t, started)
	f.tick(t, []position.StaleTrip{b}, "queued")
	deadline, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.pool.Stop(deadline); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stop=%v", err)
	}
}

func TestTaskSpanLinksToScan(t *testing.T) {
	f := newFixture(t, 1, 2)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	f.pool = positions.New(positions.Config{Workers: 1, QueueSize: 2, StaleAfter: time.Minute, RequestTimeout: time.Second}, f.repo, f.push, f.ticker, f.observer,
		slog.New(slog.NewJSONHandler(io.Discard, nil)), provider.Tracer("test"))
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).Return(nil)
	f.start(t, context.Background())
	f.tick(t, []position.StaleTrip{stale()}, "queued")
	receive(t, f.requests)
	if err := f.pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("spans=%d", len(spans))
	}
	var scan, task sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == "positions.scan" {
			scan = span
		}
		if span.Name() == "positions.request" {
			task = span
		}
	}
	if scan == nil || task == nil || task.Parent().IsValid() || len(task.Links()) != 1 || !task.Links()[0].SpanContext.Equal(scan.SpanContext()) {
		t.Fatal("task must link to scan without becoming its child")
	}
}

func TestCanceledOldTokenCannotRemoveReplacement(t *testing.T) {
	f := newFixture(t, 1, 3)
	a, b := stale(), stale()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), a).DoAndReturn(func(ctx context.Context, _ position.StaleTrip) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	f.push.EXPECT().RequestPosition(gomock.Any(), b).Return(nil)
	f.start(t, context.Background())
	f.tick(t, []position.StaleTrip{a}, "queued")
	receive(t, started)
	f.tick(t, []position.StaleTrip{b}, "queued")
	f.pool.Cancel(b.ID)
	f.tick(t, []position.StaleTrip{b}, "queued")
	close(release)
	if err := f.pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	receive(t, f.requests)
	receive(t, f.requests)
}

func TestScannerErrorDoesNotStopNextTick(t *testing.T) {
	f := newFixture(t, 1, 2)
	queried := make(chan struct{})
	f.repo.EXPECT().Stale(gomock.Any(), time.Minute).DoAndReturn(func(context.Context, time.Duration) ([]position.StaleTrip, error) {
		close(queried)
		return nil, errors.New("database temporarily down")
	})
	f.start(t, context.Background())
	f.ticker.ticks <- time.Now()
	receive(t, queried)
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).Return(nil)
	f.tick(t, []position.StaleTrip{stale()}, "queued")
	receive(t, f.requests)
}
