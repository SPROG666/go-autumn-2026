package finish_test

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
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/finish"
)

func TestExecute(t *testing.T) {
	failure := errors.New("storage unavailable")
	const alreadyCompleted = "already completed"
	const commitFails = "commit fails"
	for _, scenario := range []string{"completed", alreadyCompleted, "missing", "save fails", "history fails", commitFails} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo, history, tx, clock, observer := NewMocktripRepository(ctrl), NewMockhistoryRepository(ctrl), NewMocktxManager(ctrl), NewMockclock(ctrl), NewMockobserver(ctrl)
			u := finish.New(repo, history, tx, clock, observer)
			ctx, id := context.Background(), uuid.New()
			at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
			value := model.Trip{ID: id, Status: trip.StatusActive}
			want := value
			if err := want.Finish(at); err != nil {
				t.Fatal(err)
			}
			tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				if err := fn(ctx); err != nil {
					return err
				}
				if scenario == commitFails {
					return failure
				}
				return nil
			})
			var wantErr error
			if scenario == "missing" {
				wantErr = trip.ErrTripNotFound
				repo.EXPECT().GetForUpdate(ctx, id).Return(model.Trip{}, wantErr)
			} else {
				if scenario == alreadyCompleted {
					value = want
					wantErr = trip.ErrTripCompleted
				}
				repo.EXPECT().GetForUpdate(ctx, id).Return(value, nil)
				clock.EXPECT().Now().Return(at)
				if scenario != alreadyCompleted {
					if scenario == "save fails" {
						wantErr = failure
						repo.EXPECT().Complete(ctx, id, at).Return(failure)
					} else {
						repo.EXPECT().Complete(ctx, id, at).Return(nil)
						if scenario == "history fails" {
							wantErr = failure
							history.EXPECT().Add(ctx, id, gomock.Eq(&value.Status), trip.StatusCompleted, at).Return(failure)
						} else {
							history.EXPECT().Add(ctx, id, gomock.Eq(&value.Status), trip.StatusCompleted, at).Return(nil)
						}
					}
				}
			}
			if scenario == commitFails {
				wantErr = failure
			}
			if wantErr == nil {
				observer.EXPECT().Completed(ctx, id)
			}
			got, err := u.Execute(ctx, id)
			if !errors.Is(err, wantErr) {
				t.Fatalf("Execute error = %v; want %v", err, wantErr)
			}
			if wantErr == nil && !reflect.DeepEqual(got, want) {
				t.Fatalf("Execute = %+v; want %+v", got, want)
			}
			if wantErr != nil && got != (model.Trip{}) {
				t.Fatalf("failed transaction returned %+v", got)
			}
		})
	}
}
