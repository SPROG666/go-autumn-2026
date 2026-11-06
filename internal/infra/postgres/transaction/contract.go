package transaction

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type database interface {
	BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error)
}
