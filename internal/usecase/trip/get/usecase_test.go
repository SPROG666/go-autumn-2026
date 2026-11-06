package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/get"
)

func TestExecute(t *testing.T) {
	for _, wantErr := range []error{nil, trip.ErrTripNotFound, errors.New("connection failed")} {
		t.Run(fmtError(wantErr), func(t *testing.T) {
			repo := NewMocktripRepository(gomock.NewController(t))
			ctx, id := context.Background(), uuid.New()
			want := model.Trip{ID: id, Status: trip.StatusActive}
			repo.EXPECT().Get(ctx, id).Return(want, wantErr)
			got, err := get.New(repo).Execute(ctx, id)
			if !errors.Is(err, wantErr) {
				t.Fatalf("Execute error = %v; want %v", err, wantErr)
			}
			if wantErr == nil && got != want {
				t.Fatalf("Execute = %+v; want %+v", got, want)
			}
			if wantErr != nil && got != (model.Trip{}) {
				t.Fatal("error returned a trip")
			}
		})
	}
}

func fmtError(err error) string {
	if err == nil {
		return "success"
	}
	return err.Error()
}
