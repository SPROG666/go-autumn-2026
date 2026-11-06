package container

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

func (a *App) Run(ctx context.Context) (runErr error) {
	defer func() {
		if a.pool != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
			defer cancel()
			runErr = errors.Join(runErr, a.Close(cleanup))
		}
	}()
	var listen net.ListenConfig
	listener, err := listen.Listen(ctx, "tcp", a.Server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- a.Server.Serve(listener) }()
	a.Logger.InfoContext(ctx, "service started", "addr", listener.Addr().String())
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
	defer cancel()
	a.Logger.InfoContext(shutdown, "service stopping")
	if err := a.Server.Shutdown(shutdown); err != nil {
		return errors.Join(fmt.Errorf("graceful shutdown: %w", err), a.Server.Close(), a.Close(shutdown))
	}
	err = a.Close(shutdown)
	a.Logger.InfoContext(shutdown, "service stopped")
	return err
}

func (a *App) Close(ctx context.Context) error {
	var closeErr error
	if a.telemetry != nil {
		if err := a.telemetry.Shutdown(ctx); err != nil {
			a.Logger.WarnContext(ctx, "telemetry shutdown incomplete", "error", err)
		}
		a.telemetry = nil
	}
	if a.pool != nil {
		closed := make(chan struct{})
		pool := a.pool
		a.pool = nil
		go func() { pool.Close(); close(closed) }()
		select {
		case <-closed:
		case <-ctx.Done():
			closeErr = fmt.Errorf("shutdown budget exhausted closing PostgreSQL: %w", ctx.Err())
		}
	}
	return closeErr
}
