package save_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/position/save"
)

func TestExecute(t *testing.T) {
	failure := errors.New("storage failure")
	for _, test := range []struct {
		name                                 string
		status                               string
		lockErr, saveErr, commitErr, wantErr error
	}{
		{name: "saved", status: trip.StatusActive},
		{name: "completed", status: trip.StatusCompleted, wantErr: trip.ErrTripCompleted},
		{name: "missing", lockErr: trip.ErrTripNotFound, wantErr: trip.ErrTripNotFound},
		{name: "save failure", status: trip.StatusActive, saveErr: failure, wantErr: failure},
		{name: "commit failure", status: trip.StatusActive, commitErr: failure, wantErr: failure},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			trips, positions, tx, clock, observer := NewMocktripRepository(ctrl), NewMockpositionRepository(ctrl), NewMocktxManager(ctrl), NewMockclock(ctrl), NewMockobserver(ctrl)
			now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
			input := position.Position{TripID: uuid.New(), RecordedAt: now}
			clock.EXPECT().Now().Return(now)
			tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				if err := fn(ctx); err != nil {
					return err
				}
				return test.commitErr
			})
			trips.EXPECT().GetForUpdate(gomock.Any(), input.TripID).Return(model.Trip{Status: test.status}, test.lockErr)
			output := input
			output.ID = 42
			if test.lockErr == nil && test.status == trip.StatusActive {
				positions.EXPECT().Save(gomock.Any(), input).Return(output, test.saveErr)
			}
			if test.wantErr == nil {
				observer.EXPECT().Saved(gomock.Any())
			}
			got, err := save.New(trips, positions, tx, clock, observer).Execute(context.Background(), input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error=%v want=%v", err, test.wantErr)
			}
			if err == nil && got != output {
				t.Fatalf("position=%+v", got)
			}
		})
	}
}

func TestInvalidPositionDoesNotOpenTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	clock := NewMockclock(ctrl)
	clock.EXPECT().Now().Return(time.Now())
	_, err := save.New(NewMocktripRepository(ctrl), NewMockpositionRepository(ctrl), NewMocktxManager(ctrl), clock, NewMockobserver(ctrl)).Execute(context.Background(), position.Position{})
	var validation *trip.ValidationError
	if !errors.As(err, &validation) {
		t.Fatal(err)
	}
}
