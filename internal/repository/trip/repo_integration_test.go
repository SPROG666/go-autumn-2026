//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/infra/postgres/transaction"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	repository "github.com/SPROG666/go-autumn-2026/internal/repository/trip"
)

type databaseFixture struct {
	pool    *pgxpool.Pool
	repo    *repository.Repository
	history *repository.HistoryRepository
	keys    *repository.IdempotencyRepository
	tx      *transaction.TransactionManager
}

func database(t *testing.T) *databaseFixture {
	t.Helper()
	cfg, err := integrationConfig()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("migration files not found", err)
	}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.SplitN(string(source), "-- +goose Down", 2)[0]
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatal("apply integration migration", file, err)
		}
	}
	return &databaseFixture{pool: pool, repo: repository.NewRepository(pool, 5*time.Second), history: repository.NewHistoryRepository(pool, 5*time.Second), keys: repository.NewIdempotencyRepository(pool, 5*time.Second, time.Hour), tx: transaction.NewTransactionManager(pool, 5*time.Second)}
}

func newTrip() model.Trip {
	return model.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), StartLatitude: 59, StartLongitude: 30, EndLatitude: 60, EndLongitude: 31, Price: 100, Status: trip.StatusActive, StartedAt: time.Now().UTC().Truncate(time.Microsecond)}
}

func TestRepositoryLifecycle(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	input := newTrip()
	created, err := f.repo.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.Get(ctx, input.ID)
	if err != nil || !reflect.DeepEqual(got, created) {
		t.Fatalf("Get=%+v %v; want %+v", got, err, created)
	}
	if _, err := f.repo.Get(ctx, uuid.New()); !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatal("missing trip", err)
	}
	if _, err := f.repo.GetForUpdate(ctx, input.ID); err == nil {
		t.Fatal("lock outside transaction accepted")
	}
	if err := f.tx.Do(ctx, func(ctx context.Context) error {
		locked, err := f.repo.GetForUpdate(ctx, input.ID)
		if err != nil {
			return err
		}
		if err := locked.Finish(input.StartedAt.Add(time.Minute)); err != nil {
			return err
		}
		return f.repo.Complete(ctx, locked.ID, *locked.FinishedAt)
	}); err != nil {
		t.Fatal(err)
	}
	finished, err := f.repo.Get(ctx, input.ID)
	if err != nil || finished.Status != trip.StatusCompleted || finished.FinishedAt == nil || !finished.FinishedAt.Equal(input.StartedAt.Add(time.Minute)) {
		t.Fatalf("finished=%+v %v", finished, err)
	}
	if count, err := f.repo.CountActive(ctx); err != nil || count != 0 {
		t.Fatal("active count", count, err)
	}
}

func TestAtomicRollback(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	value := newTrip()
	err := f.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := f.repo.Create(ctx, value); err != nil {
			return err
		}
		if err := f.history.Add(ctx, value.ID, nil, trip.StatusActive, value.StartedAt); err != nil {
			return err
		}
		return f.history.Add(ctx, value.ID, nil, "invalid-status", value.StartedAt)
	})
	if err == nil {
		t.Fatal("constraint violation was swallowed")
	}
	for _, query := range []string{"SELECT count(*) FROM trips", "SELECT count(*) FROM trip_status_history"} {
		var count int
		if err := f.pool.QueryRow(ctx, query).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial transaction survived", count, err)
		}
	}
}

func TestConcurrentDriverConstraint(t *testing.T) {
	f := database(t)
	first, second := newTrip(), newTrip()
	second.DriverID = first.DriverID
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, value := range []model.Trip{first, second} {
		wg.Go(func() { <-start; _, err := f.repo.Create(context.Background(), value); results <- err })
	}
	close(start)
	wg.Wait()
	close(results)
	var success, busy int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, trip.ErrDriverBusy):
			busy++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || busy != 1 {
		t.Fatal("concurrent results", success, busy)
	}
}

func TestIdempotency(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	value, key := newTrip(), uuid.New()
	params := model.CreateParams{UserID: value.UserID, DriverID: value.DriverID, Price: value.Price}
	if _, _, err := f.keys.Reserve(ctx, key, params); err == nil {
		t.Fatal("reservation outside a transaction accepted")
	}
	err := f.tx.Do(ctx, func(ctx context.Context) error {
		_, replay, err := f.keys.Reserve(ctx, key, params)
		if err != nil {
			return err
		}
		if replay {
			t.Fatal("new key replayed")
		}
		if _, err := f.repo.Create(ctx, value); err != nil {
			return err
		}
		return f.keys.Save(ctx, key, value.ID)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = f.tx.Do(ctx, func(ctx context.Context) error {
		id, replay, err := f.keys.Reserve(ctx, key, params)
		if err == nil && (!replay || id != value.ID) {
			t.Error("key lost its trip", id, replay)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	params.Price++
	err = f.tx.Do(ctx, func(ctx context.Context) error { _, _, err := f.keys.Reserve(ctx, key, params); return err })
	if !errors.Is(err, trip.ErrIdempotencyConflict) {
		t.Fatal("changed request accepted", err)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE trip_idempotency SET expires_at = clock_timestamp() - interval '1 second' WHERE key = $1", key); err != nil {
		t.Fatal(err)
	}
	if err := f.keys.CleanupExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM trip_idempotency").Scan(&count); err != nil || count != 0 {
		t.Fatal("expired key survived", count, err)
	}
}

func TestNestedTransactionPanicRollback(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	value := newTrip()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic was swallowed")
			}
		}()
		if err := f.tx.Do(ctx, func(ctx context.Context) error {
			return f.tx.Do(ctx, func(ctx context.Context) error {
				if _, err := f.repo.Create(ctx, value); err != nil {
					return err
				}
				panic("rollback")
			})
		}); err != nil {
			t.Fatal(err)
		}
	}()
	if _, err := f.repo.Get(ctx, value.ID); !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatal("panic did not roll back", err)
	}
}
