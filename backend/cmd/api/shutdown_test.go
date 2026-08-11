package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// テスト内容: 終了用contextがキャンセルされると、サーバー停止待ちを正常終了として抜けることを確認する。
// 必要な理由: Ctrl+CやDockerからの終了通知をエラー扱いせず、Graceful Shutdownへ進めるため。
func TestWaitForShutdownReturnsOnSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := make(chan serverResult)
	if err := waitForShutdown(ctx, results); err != nil {
		t.Fatalf("waitForShutdown() error = %v; want nil", err)
	}
}

// テスト内容: HTTPまたはgRPCが予期せず停止した場合に、サーバー名と元のエラーを呼び出し元へ返すことを確認する。
// 必要な理由: 片方の障害を正常終了として隠さず、ログから停止したサーバーと原因を特定できるようにするため。
func TestWaitForShutdownReturnsServerError(t *testing.T) {
	wantErr := errors.New("serve failed")
	results := make(chan serverResult, 1)
	results <- serverResult{name: "HTTP", err: wantErr}

	err := waitForShutdown(context.Background(), results)
	if !errors.Is(err, wantErr) {
		t.Fatalf("waitForShutdown() error = %v; want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "HTTP server stopped") {
		t.Errorf("waitForShutdown() error = %q; want server name", err)
	}
}

// テスト内容: Serveがエラーなしで終了しても、予期しないサーバー停止としてエラーを返すことを確認する。
// 必要な理由: 待受処理が静かに終了してbackendの一部だけが停止した状態を、正常稼働と誤認しないため。
func TestWaitForShutdownRejectsUnexpectedNilResult(t *testing.T) {
	results := make(chan serverResult, 1)
	results <- serverResult{name: "gRPC", err: nil}

	err := waitForShutdown(context.Background(), results)
	if err == nil {
		t.Fatal("waitForShutdown() error = nil; want unexpected stop error")
	}
	if !strings.Contains(err.Error(), "gRPC server stopped unexpectedly") {
		t.Errorf("waitForShutdown() error = %q; want unexpected gRPC stop", err)
	}
}

// テスト内容: 終了期限内にHTTP ShutdownとgRPC GracefulStopが完了し、強制停止を使わないことを確認する。
// 必要な理由: 処理中のリクエストを途中で切らず、通常の終了では安全な停止方法を優先するため。
func TestShutdownServersGracefully(t *testing.T) {
	httpServer := newHTTPShutdownRecorder(nil)
	grpcServer := newGRPCShutdownRecorder(false)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := shutdownServers(ctx, httpServer, grpcServer); err != nil {
		t.Fatalf("shutdownServers() error = %v; want nil", err)
	}

	assertChannelClosed(t, httpServer.shutdownCalled, "HTTP Shutdown")
	assertChannelClosed(t, grpcServer.gracefulStopCalled, "gRPC GracefulStop")

	select {
	case <-grpcServer.stopCalled:
		t.Error("gRPC Stop was called during graceful shutdown")
	default:
	}
}

// テスト内容: 終了期限を超えた場合にgRPCのGracefulStop待ちを打ち切り、Stopで強制停止することを確認する。
// 必要な理由: 終わらないRPCがあってもプロセスが無期限に残らず、Dockerの停止処理を完了できるようにするため。
func TestShutdownServersFallsBackToStop(t *testing.T) {
	httpServer := newHTTPShutdownRecorder(nil)
	grpcServer := newGRPCShutdownRecorder(true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := shutdownServers(ctx, httpServer, grpcServer)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdownServers() error = %v; want context canceled", err)
	}

	assertChannelClosed(t, httpServer.shutdownCalled, "HTTP Shutdown")
	assertChannelClosed(t, grpcServer.gracefulStopCalled, "gRPC GracefulStop")
	assertChannelClosed(t, grpcServer.stopCalled, "gRPC Stop")
}

// テスト内容: HTTPの終了エラーとgRPCのタイムアウトが同時に起きても、両方の原因を返すことを確認する。
// 必要な理由: 一方のエラーで他方を上書きせず、複合的な終了失敗を調査できるようにするため。
func TestShutdownServersPreservesBothErrors(t *testing.T) {
	httpErr := errors.New("HTTP shutdown failed")
	httpServer := newHTTPShutdownRecorder(httpErr)
	grpcServer := newGRPCShutdownRecorder(true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := shutdownServers(ctx, httpServer, grpcServer)
	if !errors.Is(err, httpErr) {
		t.Errorf("shutdownServers() error = %v; want wrapped HTTP error", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("shutdownServers() error = %v; want context canceled", err)
	}
}

type httpShutdownRecorder struct {
	shutdownCalled chan struct{}
	err            error
}

func newHTTPShutdownRecorder(err error) *httpShutdownRecorder {
	return &httpShutdownRecorder{
		shutdownCalled: make(chan struct{}),
		err:            err,
	}
}

func (s *httpShutdownRecorder) Shutdown(context.Context) error {
	close(s.shutdownCalled)
	return s.err
}

type grpcShutdownRecorder struct {
	gracefulStopCalled chan struct{}
	stopCalled         chan struct{}
	waitForStop        bool
	stopOnce           sync.Once
}

func newGRPCShutdownRecorder(waitForStop bool) *grpcShutdownRecorder {
	return &grpcShutdownRecorder{
		gracefulStopCalled: make(chan struct{}),
		stopCalled:         make(chan struct{}),
		waitForStop:        waitForStop,
	}
}

func (s *grpcShutdownRecorder) GracefulStop() {
	close(s.gracefulStopCalled)
	if s.waitForStop {
		<-s.stopCalled
	}
}

func (s *grpcShutdownRecorder) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCalled)
	})
}

func assertChannelClosed(t *testing.T, channel <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-channel:
	default:
		t.Errorf("%s was not called", name)
	}
}
