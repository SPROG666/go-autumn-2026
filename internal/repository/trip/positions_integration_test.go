//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	positionrepo "github.com/SPROG666/go-autumn-2026/internal/repository/position"
)

func savePosition(t *testing.T, f *databaseFixture, p position.Position) position.Position {
	t.Helper()
	repo := positionrepo.New(f.pool, 5*time.Second)
	var got position.Position
	if err := f.tx.Do(context.Background(), func(ctx context.Context) error {
		if _, err := f.repo.GetForUpdate(ctx, p.TripID); err != nil {
			return err
		}
		var err error
		got, err = repo.Save(ctx, p)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPositionsAtomicityOrderingAndLateArrival(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	value := newTrip()
	if _, err := f.repo.Create(ctx, value); err != nil {
		t.Fatal(err)
	}
	repo := positionrepo.New(f.pool, 5*time.Second)
	at := value.StartedAt
	fresh := savePosition(t, f, position.Position{TripID: value.ID, Latitude: 1, RecordedAt: at})
	old := savePosition(t, f, position.Position{TripID: value.ID, Latitude: 2, RecordedAt: at.Add(-time.Minute)})
	same := savePosition(t, f, position.Position{TripID: value.ID, Latitude: 3, RecordedAt: at})
	got, err := f.repo.Get(ctx, value.ID)
	if err != nil || got.LastPositionAt == nil || !got.LastPositionAt.Equal(at) {
		t.Fatalf("last position=%+v err=%v", got.LastPositionAt, err)
	}
	positions, err := repo.List(ctx, value.ID)
	if err != nil || !reflect.DeepEqual(positions, []position.Position{old, fresh, same}) {
		t.Fatalf("positions=%+v err=%v", positions, err)
	}
	// Make UPDATE fail after INSERT; both the inserted point and time must roll back.
	if _, err := f.pool.Exec(ctx, `ALTER TABLE trips ADD CONSTRAINT reject_new_position CHECK (last_position_at <= started_at)`); err != nil {
		t.Fatal(err)
	}
	err = f.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := f.repo.GetForUpdate(ctx, value.ID); err != nil {
			return err
		}
		_, err := repo.Save(ctx, position.Position{TripID: value.ID, RecordedAt: at.Add(time.Second)})
		return err
	})
	if err == nil {
		t.Fatal("UPDATE failure accepted")
	}
	positions, err = repo.List(ctx, value.ID)
	if err != nil || len(positions) != 3 {
		t.Fatalf("partial INSERT survived: %+v %v", positions, err)
	}
	got, err = f.repo.Get(ctx, value.ID)
	if err != nil || got.LastPositionAt == nil || !got.LastPositionAt.Equal(at) {
		t.Fatal("last position changed after rollback", err)
	}
	if _, err := repo.Save(ctx, position.Position{TripID: value.ID, RecordedAt: at}); err == nil {
		t.Fatal("save outside transaction accepted")
	}
}

func TestStaleSelectionAndIndex(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	without, older, fresh, completed := newTrip(), newTrip(), newTrip(), newTrip()
	without.StartedAt = now.Add(-3 * time.Minute)
	older.StartedAt = now.Add(-10 * time.Minute)
	fresh.StartedAt = now.Add(-10 * time.Minute)
	completed.StartedAt = now.Add(-time.Hour)
	completed.Status = trip.StatusCompleted
	finished := now
	completed.FinishedAt = &finished
	for _, v := range []model.Trip{without, older, fresh, completed} {
		if _, err := f.repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	savePosition(t, f, position.Position{TripID: older.ID, RecordedAt: now.Add(-5 * time.Minute)})
	savePosition(t, f, position.Position{TripID: fresh.ID, RecordedAt: now})
	values, err := f.repo.Stale(ctx, time.Minute)
	want := []position.StaleTrip{{ID: older.ID, DriverID: older.DriverID}, {ID: without.ID, DriverID: without.DriverID}}
	if err != nil || !reflect.DeepEqual(values, want) {
		t.Fatalf("stale=%+v err=%v want=%+v", values, err, want)
	}
	// Representative selective workload: the planner must pick our expression index.
	if _, err := f.pool.Exec(ctx, `INSERT INTO trips(id,user_id,driver_id,start_latitude,start_longitude,end_latitude,end_longitude,price,status,started_at)
        SELECT gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),0,0,0,0,0,'active',CURRENT_TIMESTAMP
        FROM generate_series(1,20000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `ANALYZE trips`); err != nil {
		t.Fatal(err)
	}
	var plan string
	rows, err := f.pool.Query(ctx, `EXPLAIN (ANALYZE, FORMAT JSON) SELECT id,driver_id FROM trips
        WHERE status='active' AND COALESCE(last_position_at,started_at)<CURRENT_TIMESTAMP-$1*INTERVAL '1 second'
        ORDER BY COALESCE(last_position_at,started_at),id`, 60.0)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("empty explain")
	}
	if err := rows.Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "trips_active_last_position_idx") || strings.Contains(plan, "Seq Scan") {
		t.Fatal("expected index scan", plan)
	}
	t.Log("EXPLAIN (ANALYZE):", plan)
}

func TestPositionMigrationsDownUp(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	for _, name := range []string{"00005_index_stale_trips.sql", "00004_create_trip_positions.sql"} {
		source, err := os.ReadFile("../../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(string(source), "-- +goose Down", 2)
		if len(parts) != 2 {
			t.Fatal("missing down migration")
		}
		if _, err := f.pool.Exec(ctx, parts[1]); err != nil {
			t.Fatal("down", name, err)
		}
	}
	for _, name := range []string{"00004_create_trip_positions.sql", "00005_index_stale_trips.sql"} {
		source, err := os.ReadFile("../../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.SplitN(string(source), "-- +goose Down", 2)[0]
		if _, err := f.pool.Exec(ctx, up); err != nil {
			t.Fatal("up", name, err)
		}
	}
}

func TestPositionSaveSerializesWithFinish(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	value := newTrip()
	if _, err := f.repo.Create(ctx, value); err != nil {
		t.Fatal(err)
	}
	locked, release := make(chan struct{}), make(chan struct{})
	finish := make(chan error, 1)
	go func() {
		finish <- f.tx.Do(ctx, func(ctx context.Context) error {
			if _, err := f.repo.GetForUpdate(ctx, value.ID); err != nil {
				return err
			}
			close(locked)
			<-release
			return f.repo.Complete(ctx, value.ID, time.Now())
		})
	}()
	<-locked
	outcome := make(chan error, 1)
	go func() {
		outcome <- f.tx.Do(ctx, func(ctx context.Context) error {
			v, err := f.repo.GetForUpdate(ctx, value.ID)
			if err != nil {
				return err
			}
			if v.Status != trip.StatusActive {
				return trip.ErrTripCompleted
			}
			return nil
		})
	}()
	close(release)
	if err := <-finish; err != nil {
		t.Fatal(err)
	}
	if err := <-outcome; !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatal("accepted a position after completion", err)
	}
	if _, err := positionrepo.New(f.pool, time.Second).List(ctx, uuid.New()); err != nil {
		t.Fatal(err)
	}
}
