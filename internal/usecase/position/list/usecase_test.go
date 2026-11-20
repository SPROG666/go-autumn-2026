package list_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/position/list"
)

func TestExecute(t *testing.T) {
	failure := errors.New("storage failure")
	for _, test := range []struct {
		name             string
		tripErr, listErr error
	}{
		{name: "route"}, {name: "missing", tripErr: trip.ErrTripNotFound}, {name: "database error", listErr: failure},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			trips, repo := NewMocktripRepository(ctrl), NewMockpositionRepository(ctrl)
			id := uuid.New()
			values := []position.Position{{ID: 1, TripID: id}}
			trips.EXPECT().Get(gomock.Any(), id).Return(model.Trip{}, test.tripErr)
			if test.tripErr == nil {
				repo.EXPECT().List(gomock.Any(), id).Return(values, test.listErr)
			}
			got, err := list.New(trips, repo).Execute(context.Background(), id)
			wantErr := test.tripErr
			if wantErr == nil {
				wantErr = test.listErr
			}
			if !errors.Is(err, wantErr) {
				t.Fatal(err)
			}
			if err == nil && !reflect.DeepEqual(got, values) {
				t.Fatalf("route=%+v", got)
			}
		})
	}
}
