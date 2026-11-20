package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

type Config struct {
	Workers        int
	QueueSize      int
	StaleAfter     time.Duration
	RequestTimeout time.Duration
}

type task struct {
	trip   position.StaleTrip
	source trace.SpanContext
	cancel context.CancelFunc
}

type Pool struct {
	cfg       Config
	trips     tripRepository
	push      gateway
	ticker    Ticker
	observer  observer
	logger    *slog.Logger
	tracer    trace.Tracer
	queue     chan *task
	mu        sync.Mutex
	pending   map[uuid.UUID]*task
	active    int64
	stopScan  context.CancelFunc
	abortJobs context.CancelFunc
	done      chan struct{}
}

func New(cfg Config, trips tripRepository, push gateway, ticker Ticker, observer observer, logger *slog.Logger, tracer trace.Tracer) *Pool {
	return &Pool{
		cfg: cfg, trips: trips, push: push, ticker: ticker, observer: observer,
		logger: logger, tracer: tracer, queue: make(chan *task, cfg.QueueSize),
		pending: make(map[uuid.UUID]*task), done: make(chan struct{}),
	}
}

func (p *Pool) Start(ctx context.Context) {
	scanCtx, stop := context.WithCancel(ctx)
	jobCtx, abort := context.WithCancel(context.WithoutCancel(ctx))
	p.stopScan, p.abortJobs = stop, abort
	go p.run(scanCtx, jobCtx)
}

func (p *Pool) run(scanCtx, jobCtx context.Context) {
	defer close(p.done)
	defer p.abortJobs()
	var workers sync.WaitGroup
	for range p.cfg.Workers {
		workers.Go(func() {
			for item := range p.queue {
				p.process(jobCtx, item)
			}
		})
	}
	p.observer.Queue(jobCtx, 0)
	p.observer.Active(jobCtx, 0)
	p.scan(scanCtx)
	p.ticker.Stop()
	close(p.queue)
	workers.Wait()
}

func (p *Pool) Stop(ctx context.Context) error {
	if p.stopScan == nil {
		p.ticker.Stop()
		return nil
	}
	p.stopScan()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		p.abortJobs()
		<-p.done
		return fmt.Errorf("position workers shutdown: %w", ctx.Err())
	}
}

func (p *Pool) scan(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.ticker.C():
			if ctx.Err() != nil {
				return
			}
			p.check(ctx)
		}
	}
}

func (p *Pool) check(ctx context.Context) {
	ctx, span := p.tracer.Start(ctx, "positions.scan")
	defer span.End()
	trips, err := p.trips.Stale(ctx, p.cfg.StaleAfter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "stale_query")
		p.logger.WarnContext(ctx, "stale trips scan failed", "error", err)
		return
	}
	for _, value := range trips {
		if ctx.Err() != nil {
			return
		}
		p.enqueue(ctx, value)
	}
}

func (p *Pool) enqueue(ctx context.Context, value position.StaleTrip) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.pending[value.ID]; exists {
		p.observer.Check(ctx, "duplicate")
		return
	}
	item := &task{trip: value, source: trace.SpanContextFromContext(ctx)}
	// Publish the token and channel item under the same lock as cancellation.
	p.pending[value.ID] = item
	select {
	case p.queue <- item:
		p.observer.Check(ctx, "queued")
		p.observer.Queue(ctx, int64(len(p.queue)))
	default:
		delete(p.pending, value.ID)
		p.observer.Check(ctx, "dropped")
		p.observer.Dropped(ctx)
		p.logger.WarnContext(ctx, "position request queue full", "trip_id", value.ID)
	}
}

func (p *Pool) Cancel(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if item, exists := p.pending[id]; exists {
		if item.cancel != nil {
			item.cancel()
		}
		delete(p.pending, id)
	}
}

func (p *Pool) process(ctx context.Context, item *task) {
	p.mu.Lock()
	p.observer.Queue(ctx, int64(len(p.queue)))
	if p.pending[item.trip.ID] != item {
		p.mu.Unlock()
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	item.cancel = cancel
	p.active++
	p.observer.Active(ctx, p.active)
	p.mu.Unlock()
	defer cancel()
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.pending[item.trip.ID] == item {
			delete(p.pending, item.trip.ID)
		}
		p.active--
		p.observer.Active(ctx, p.active)
	}()
	if requestCtx.Err() != nil {
		return
	}
	options := []trace.SpanStartOption{trace.WithNewRoot()}
	if item.source.IsValid() {
		options = append(options, trace.WithLinks(trace.Link{SpanContext: item.source}))
	}
	requestCtx, span := p.tracer.Start(requestCtx, "positions.request", options...)
	defer span.End()
	started := time.Now()
	result := "success"
	if err := p.push.RequestPosition(requestCtx, item.trip); err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "push_error")
		p.logger.WarnContext(requestCtx, "position request failed", "trip_id", item.trip.ID, "error", err)
	}
	p.observer.Requested(requestCtx, result, time.Since(started))
}
