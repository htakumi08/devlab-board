package main

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	grpcv1 "devlab-board/backend/gen/grpclab/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// テスト内容: 1つの起動処理からHTTPとgRPCが同時に待ち受け、両方のリクエストへ応答できることを確認する。
// 必要な理由: 一方のServeがmain処理を塞いで他方を起動できない回帰を防ぎ、backendが2つの通信方式を提供できることを保証するため。
func TestStartServersServesHTTPAndGRPC(t *testing.T) {
	httpListener := newTestListener(t)
	grpcListener := newTestListener(t)

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: time.Second,
	}
	grpcServer := newGRPCServer()

	results := startServers(httpServer, httpListener, grpcServer, grpcListener)
	t.Cleanup(func() {
		grpcServer.Stop()
		if err := httpServer.Close(); err != nil {
			t.Errorf("close HTTP server: %v", err)
		}
	})

	httpClient := &http.Client{Timeout: time.Second}
	response, err := httpClient.Get("http://" + loopbackAddress(t, httpListener))
	if err != nil {
		t.Fatalf("call HTTP server: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("HTTP status = %d; want %d", response.StatusCode, http.StatusOK)
	}

	connection, err := grpc.NewClient(
		loopbackAddress(t, grpcListener),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close gRPC connection: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reply, err := grpcv1.NewGreetingServiceClient(connection).Hello(
		ctx,
		&grpcv1.HelloRequest{Name: "Taro"},
	)
	if err != nil {
		t.Fatalf("call GreetingService.Hello: %v", err)
	}
	if reply.GetMessage() != "Hello, Taro!" {
		t.Errorf("gRPC message = %q; want %q", reply.GetMessage(), "Hello, Taro!")
	}

	select {
	case result := <-results:
		t.Fatalf("%s server stopped while both servers should be running: %v", result.name, result.err)
	default:
	}
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create test listener: %v", err)
	}
	return listener
}

func loopbackAddress(t *testing.T, listener net.Listener) string {
	t.Helper()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address has type %T; want *net.TCPAddr", listener.Addr())
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(address.Port))
}
