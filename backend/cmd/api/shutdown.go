package main

import (
	"context"
	"errors"
	"fmt"
)

type httpShutdowner interface {
	Shutdown(context.Context) error
}

type grpcStopper interface {
	GracefulStop()
	Stop()
}

// waitForShutdown は、終了通知または最初のサーバー停止まで待機する。
func waitForShutdown(ctx context.Context, results <-chan serverResult) error {
	select {
	case <-ctx.Done():
		return nil
	case result := <-results:
		if result.err == nil {
			return fmt.Errorf("%s server stopped unexpectedly", result.name)
		}
		return fmt.Errorf("%s server stopped: %w", result.name, result.err)
	}
}

// shutdownServers は、HTTPとgRPCを同じ期限内で並行して安全に停止する。
func shutdownServers(
	ctx context.Context,
	httpServer httpShutdowner,
	grpcServer grpcStopper,
) error {
	results := make(chan error, 2)

	go func() {
		err := httpServer.Shutdown(ctx)
		if err != nil {
			err = fmt.Errorf("shutdown HTTP server: %w", err)
		}
		results <- err
	}()

	go func() {
		results <- gracefulStopGRPC(ctx, grpcServer)
	}()

	return errors.Join(<-results, <-results)
}

// gracefulStopGRPC は処理中のRPCを待ち、期限を超えた場合だけ強制停止へ切り替える。
func gracefulStopGRPC(ctx context.Context, server grpcStopper) error {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		server.Stop()
		<-done
		return fmt.Errorf("graceful stop gRPC: %w", ctx.Err())
	}
}
