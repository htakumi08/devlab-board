package main

import (
	"strings"
	"testing"
)

// テスト内容: 空きポートを指定するとgRPC用のTCP listenerを作成できることを確認する。
// 必要な理由: 実際のサーバー起動前に通信の入口を確保できなければgRPCを提供できないため。
func TestNewGRPCListener(t *testing.T) {
	listener, err := newGRPCListener("0")
	if err != nil {
		t.Fatalf("newGRPCListener() returned an error: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close gRPC listener: %v", err)
		}
	})

	if listener.Addr() == nil {
		t.Fatal("newGRPCListener() returned a listener without an address")
	}
}

// テスト内容: 不正なポート指定をエラーとして返し、原因を識別できる情報が含まれることを確認する。
// 必要な理由: 設定ミスを曖昧な失敗やpanicにせず、運用者がログから修正箇所を判断できるようにするため。
func TestNewGRPCListenerRejectsInvalidPort(t *testing.T) {
	_, err := newGRPCListener("invalid")
	if err == nil {
		t.Fatal("newGRPCListener() returned nil error for an invalid port")
	}
	if !strings.Contains(err.Error(), "listen on gRPC port invalid") {
		t.Errorf("error = %q; want gRPC port context", err)
	}
}

// テスト内容: GreetingServiceと開発用ReflectionがgRPCサーバーへ登録されることを確認する。
// 必要な理由: Hello RPCの呼び出しとgrpcurlによるサービス調査が可能な状態を、起動前に保証するため。
func TestNewGRPCServerRegistersServices(t *testing.T) {
	server := newGRPCServer()
	t.Cleanup(server.Stop)

	services := server.GetServiceInfo()
	for _, serviceName := range []string{
		"grpclab.v1.GreetingService",
		"grpc.reflection.v1.ServerReflection",
	} {
		if _, ok := services[serviceName]; !ok {
			t.Errorf("service %q is not registered", serviceName)
		}
	}
}
