package container

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

func (a *App) Run(ctx context.Context) error {
	var listen net.ListenConfig
	listener, err := listen.Listen(ctx, "tcp", a.Server.Addr)
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
		defer cancel()
		return errors.Join(fmt.Errorf("listen HTTP: %w", err), a.Close(cleanup))
	}
	a.positions.Start(context.WithoutCancel(ctx))
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- a.Server.Serve(listener) }()
	a.Logger.InfoContext(ctx, "service started", "addr", listener.Addr().String())
	var runErr error
	select {
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve HTTP: %w", err)
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
	defer cancel()
	a.Logger.InfoContext(shutdown, "service stopping")
	if err := a.Server.Shutdown(shutdown); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("graceful shutdown: %w", err), a.Server.Close())
	}
	runErr = errors.Join(runErr, a.Close(shutdown))
	if shutdown.Err() != nil {
		a.Logger.ErrorContext(shutdown, "shutdown budget exhausted", "error", shutdown.Err())
	} else {
		a.Logger.InfoContext(shutdown, "service stopped")
	}
	return runErr
}

func (a *App) Close(ctx context.Context) error {
	var closeErr error
	if a.positions != nil {
		closeErr = a.positions.Stop(ctx)
		a.positions = nil
	}
	if a.push != nil {
		a.push.Close()
		a.push = nil
	}
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
			closeErr = errors.Join(closeErr, fmt.Errorf("shutdown budget exhausted closing PostgreSQL: %w", ctx.Err()))
		}
	}
	return closeErr
}
