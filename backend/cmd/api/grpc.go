package main

import (
	"fmt"
	"net"

	grpcv1 "devlab-board/backend/gen/grpclab/v1"
	"devlab-board/backend/internal/grpclab"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// newGRPCListener は、指定されたTCPポートにgRPC通信用の入口を作る。
func newGRPCListener(port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("listen on gRPC port %s: %w", port, err)
	}

	return listener, nil
}

// newGRPCServer は、gRPCの受付本体を作り、利用可能なサービスを登録する。
// TCP listenerの作成とServeの開始は、HTTPとの起動・停止を調整するmainが担当する。
func newGRPCServer() *grpc.Server {
	server := grpc.NewServer()
	grpcv1.RegisterGreetingServiceServer(server, grpclab.NewGreetingServer())
	reflection.Register(server)

	return server
}
