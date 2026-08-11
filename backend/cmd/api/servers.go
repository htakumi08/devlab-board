package main

import (
	"fmt"
	"net"
	"net/http"

	"google.golang.org/grpc"
)

type serverResult struct {
	name string
	err  error
}

// newHTTPListener は、指定されたTCPポートにHTTP通信用の入口を作る。
func newHTTPListener(port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("listen on HTTP port %s: %w", port, err)
	}

	return listener, nil
}

// startServers はHTTPとgRPCの待受処理を別々のgoroutineで開始する。
// 結果用channelは両方のgoroutineが終了結果を送っても停止しないよう、サーバー数と同じ容量を持たせる。
func startServers(
	httpServer *http.Server,
	httpListener net.Listener,
	grpcServer *grpc.Server,
	grpcListener net.Listener,
) <-chan serverResult {
	results := make(chan serverResult, 2)

	go func() {
		results <- serverResult{
			name: "HTTP",
			err:  httpServer.Serve(httpListener),
		}
	}()

	go func() {
		results <- serverResult{
			name: "gRPC",
			err:  grpcServer.Serve(grpcListener),
		}
	}()

	return results
}
