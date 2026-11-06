package trip

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-playground/validator/v10"
)

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

func (p CreateParams) Validate() error {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("field")
	})
	err := validate.Struct(p)
	if err == nil {
		return nil
	}

	failure := err.(validator.ValidationErrors)[0]
	reason := "out of range"
	switch {
	case failure.Tag() == "required":
		reason = "must be a non-zero UUID"
	case failure.Field() == "price":
		reason = "must be non-negative"
	}
	return &ValidationError{Field: failure.Field(), Reason: reason}
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	at = at.UTC().Truncate(time.Microsecond)
	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at
	return nil
}
