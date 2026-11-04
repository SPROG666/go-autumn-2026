package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionManager struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewTransactionManager(pool *pgxpool.Pool, timeout time.Duration) *TransactionManager {
	return &TransactionManager{pool: pool, timeout: timeout}
}

func (m *TransactionManager) Do(ctx context.Context, fn func(context.Context) error) (err error) {
	if _, ok := ctx.Value(transactionKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	txCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	tx, err := m.pool.BeginTx(txCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), m.timeout)
		defer cleanupCancel()
		if rollbackErr := tx.Rollback(cleanupCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
	}()

	txCtx = context.WithValue(txCtx, transactionKey{}, tx)
	if err := fn(txCtx); err != nil {
		return fmt.Errorf("transaction callback: %w", err)
	}
	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
