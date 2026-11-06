package runtime

import (
	"time"

	"github.com/google/uuid"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type IDs struct{}

func (IDs) New() (uuid.UUID, error) { return uuid.NewRandom() }
