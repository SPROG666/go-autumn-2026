package create_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/create"
)

type fixture struct {
	ctx      context.Context
	params   model.CreateParams
	value    model.Trip
	key      uuid.UUID
	repo     *MocktripRepository
	history  *MockhistoryRepository
	tx       *MocktxManager
	keys     *MockidempotencyRepository
	clock    *Mockclock
	ids      *MockidGenerator
	observer *Mockobserver
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	params := model.CreateParams{UserID: uuid.New(), DriverID: uuid.New(), Price: 100}
	return &fixture{
		ctx: context.Background(), params: params, key: uuid.New(),
		value: model.Trip{ID: uuid.New(), UserID: params.UserID, DriverID: params.DriverID, Price: 100, Status: trip.StatusActive, StartedAt: time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)},
		repo:  NewMocktripRepository(ctrl), history: NewMockhistoryRepository(ctrl), tx: NewMocktxManager(ctrl), keys: NewMockidempotencyRepository(ctrl), clock: NewMockclock(ctrl), ids: NewMockidGenerator(ctrl), observer: NewMockobserver(ctrl),
	}
}

func (f *fixture) transaction(commitErr error) {
	f.tx.EXPECT().Do(f.ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		if err := fn(ctx); err != nil {
			return err
		}
		return commitErr
	})
}

func (f *fixture) creation(saveErr, historyErr error) {
	f.ids.EXPECT().New().Return(f.value.ID, nil)
	f.clock.EXPECT().Now().Return(f.value.StartedAt)
	f.repo.EXPECT().Create(f.ctx, f.value).Return(f.value, saveErr)
	if saveErr == nil {
		f.history.EXPECT().Add(f.ctx, f.value.ID, nil, trip.StatusActive, f.value.StartedAt).Return(historyErr)
	}
}

func (f *fixture) reserve(replay bool, err error) {
	f.keys.EXPECT().CleanupExpired(f.ctx).Return(nil)
	f.keys.EXPECT().Reserve(f.ctx, f.key, f.params).Return(f.value.ID, replay, err)
}

func TestExecute(t *testing.T) {
	failure := errors.New("storage unavailable")
	for _, test := range []struct {
		name        string
		key, replay bool
		wantErr     error
		setup       func(*fixture)
	}{
		{name: "created", setup: func(f *fixture) {
			f.transaction(nil)
			f.creation(nil, nil)
			f.observer.EXPECT().Created(f.ctx, f.value.ID)
		}},
		{name: "key created", key: true, setup: func(f *fixture) {
			f.transaction(nil)
			f.reserve(false, nil)
			f.creation(nil, nil)
			f.keys.EXPECT().Save(f.ctx, f.key, f.value.ID).Return(nil)
			f.observer.EXPECT().Created(f.ctx, f.value.ID)
		}},
		{name: "replay", key: true, replay: true, setup: func(f *fixture) {
			f.transaction(nil)
			f.reserve(true, nil)
			f.repo.EXPECT().Get(f.ctx, f.value.ID).Return(f.value, nil)
		}},
		{name: "busy", wantErr: trip.ErrDriverBusy, setup: func(f *fixture) { f.transaction(nil); f.creation(trip.ErrDriverBusy, nil) }},
		{name: "history fails", wantErr: failure, setup: func(f *fixture) { f.transaction(nil); f.creation(nil, failure) }},
		{name: "commit fails", wantErr: failure, setup: func(f *fixture) { f.transaction(failure); f.creation(nil, nil) }},
		{name: "reserve fails", key: true, wantErr: trip.ErrIdempotencyConflict, setup: func(f *fixture) { f.transaction(nil); f.reserve(false, trip.ErrIdempotencyConflict) }},
		{name: "save key fails", key: true, wantErr: failure, setup: func(f *fixture) {
			f.transaction(nil)
			f.reserve(false, nil)
			f.creation(nil, nil)
			f.keys.EXPECT().Save(f.ctx, f.key, f.value.ID).Return(failure)
		}},
		{name: "cleanup fails", key: true, wantErr: failure, setup: func(f *fixture) { f.keys.EXPECT().CleanupExpired(f.ctx).Return(failure) }},
		{name: "id fails", wantErr: failure, setup: func(f *fixture) { f.transaction(nil); f.ids.EXPECT().New().Return(uuid.Nil, failure) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			test.setup(f)
			u := create.New(f.repo, f.history, f.tx, f.keys, f.clock, f.ids, f.observer)
			var key *uuid.UUID
			if test.key {
				key = &f.key
			}
			got, replay, err := u.Execute(f.ctx, f.params, key)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error=%v; want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				if got != (model.Trip{}) || replay {
					t.Fatal("failed transaction returned a trip")
				}
				return
			}
			if !reflect.DeepEqual(got, f.value) || replay != test.replay {
				t.Fatalf("Execute=%+v, replay=%v", got, replay)
			}
		})
	}
}

func TestInvalidParams(t *testing.T) {
	f := newFixture(t)
	f.params.Price = -1
	u := create.New(f.repo, f.history, f.tx, f.keys, f.clock, f.ids, f.observer)
	_, _, err := u.Execute(f.ctx, f.params, nil)
	var validation *trip.ValidationError
	if !errors.As(err, &validation) || validation.Field != "price" {
		t.Fatal(err)
	}
}
