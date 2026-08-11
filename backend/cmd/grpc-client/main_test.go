package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	grpcv1 "devlab-board/backend/gen/grpclab/v1"
	"devlab-board/backend/internal/grpclab"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type greetingClientFunc func(
	context.Context,
	*grpcv1.HelloRequest,
	...grpc.CallOption,
) (*grpcv1.HelloResponse, error)

func (f greetingClientFunc) Hello(
	ctx context.Context,
	request *grpcv1.HelloRequest,
	options ...grpc.CallOption,
) (*grpcv1.HelloResponse, error) {
	return f(ctx, request, options...)
}

// テスト内容: 指定した名前をHelloRequestへ設定し、HelloResponseのmessageを返すことを確認する。
// 必要な理由: CLI入力とprotobufのrequest/responseの対応が崩れる回帰を防ぐため。
func TestCallHelloReturnsGreeting(t *testing.T) {
	client := greetingClientFunc(func(
		_ context.Context,
		request *grpcv1.HelloRequest,
		_ ...grpc.CallOption,
	) (*grpcv1.HelloResponse, error) {
		if request.GetName() != "Taro" {
			t.Errorf("request name = %q; want %q", request.GetName(), "Taro")
		}

		return &grpcv1.HelloResponse{Message: "Hello, Taro!"}, nil
	})

	message, err := callHello(context.Background(), client, "Taro", time.Second)
	if err != nil {
		t.Fatalf("callHello() returned an error: %v", err)
	}
	if message != "Hello, Taro!" {
		t.Errorf("message = %q; want %q", message, "Hello, Taro!")
	}
}

// テスト内容: Hello RPCがエラーを返したとき、gRPCステータスコードを保ったまま呼び出し元へ返すことを確認する。
// 必要な理由: 接続障害などを成功として扱わず、上位処理がエラー種別を判定できる状態を守るため。
func TestCallHelloPreservesRPCError(t *testing.T) {
	client := greetingClientFunc(func(
		context.Context,
		*grpcv1.HelloRequest,
		...grpc.CallOption,
	) (*grpcv1.HelloResponse, error) {
		return nil, status.Error(codes.Unavailable, "server unavailable")
	})

	_, err := callHello(context.Background(), client, "Taro", time.Second)
	if err == nil {
		t.Fatal("callHello() returned nil error; want an RPC error")
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("status code = %s; want %s", status.Code(err), codes.Unavailable)
	}
}

// テスト内容: RPCへ期限付きcontextを渡し、期限超過時にDeadlineExceededを返すことを確認する。
// 必要な理由: サーバーが応答しない場合にクライアントが無期限に停止する回帰を防ぐため。
func TestCallHelloAppliesTimeout(t *testing.T) {
	client := greetingClientFunc(func(
		ctx context.Context,
		_ *grpcv1.HelloRequest,
		_ ...grpc.CallOption,
	) (*grpcv1.HelloResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	_, err := callHello(context.Background(), client, "Taro", 0)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v; want context deadline exceeded", err)
	}
}

// テスト内容: CLI引数から接続先・名前・timeoutを受け取り、実際のgRPCサーバーの応答を標準出力へ書くことを確認する。
// 必要な理由: コマンドの入口から生成クライアント、RPC、出力までの配線ミスを利用者の実行方法に近い形で防ぐため。
func TestRunCLIRequestsGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create test listener: %v", err)
	}

	server := grpc.NewServer()
	grpcv1.RegisterGreetingServiceServer(server, grpclab.NewGreetingServer())
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		if err := <-serveErrors; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve test gRPC server: %v", err)
		}
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close test listener: %v", err)
		}
	})

	var output bytes.Buffer
	err = runCLI(
		context.Background(),
		[]string{
			"-target", listener.Addr().String(),
			"-name", "Taro",
			"-timeout", "1s",
		},
		&output,
	)
	if err != nil {
		t.Fatalf("runCLI() returned an error: %v", err)
	}
	if strings.TrimSpace(output.String()) != "Hello, Taro!" {
		t.Errorf("output = %q; want %q", output.String(), "Hello, Taro!\n")
	}
}

// テスト内容: timeoutへ時間として解釈できない値を渡したとき、RPCを実行せず引数エラーを返すことを確認する。
// 必要な理由: 入力ミスを通信障害として扱わず、利用者がコマンド指定を修正できるようにするため。
func TestRunCLIRejectsInvalidTimeout(t *testing.T) {
	var output bytes.Buffer

	err := runCLI(
		context.Background(),
		[]string{"-timeout", "invalid"},
		&output,
	)
	if err == nil {
		t.Fatal("runCLI() returned nil error; want a flag parsing error")
	}
}

// テスト内容: help指定では使用方法を表示し、実行エラーとして扱わないことを確認する。
// 必要な理由: 利用者が引数を確認する正常操作で、コマンドが異常終了する回帰を防ぐため。
func TestRunCLIShowsHelpSuccessfully(t *testing.T) {
	var output bytes.Buffer

	err := runCLI(context.Background(), []string{"-help"}, &output)
	if err != nil {
		t.Fatalf("runCLI() returned an error for help: %v", err)
	}
	if !strings.Contains(output.String(), "Usage of grpc-client:") {
		t.Errorf("output = %q; want usage text", output.String())
	}
}

// テスト内容: 接続できないgRPCサーバーへのRPCが失敗したとき、CLI入口までエラーを返すことを確認する。
// 必要な理由: 通信失敗時に空の成功結果を表示せず、コマンドを非正常終了できる状態を保証するため。
func TestRunCLIPropagatesRPCError(t *testing.T) {
	var output bytes.Buffer

	err := runCLI(
		context.Background(),
		[]string{
			"-target", "127.0.0.1:1",
			"-timeout", "0s",
		},
		&output,
	)
	if err == nil {
		t.Fatal("runCLI() returned nil error; want an RPC error")
	}
	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("status code = %s; want %s", status.Code(err), codes.DeadlineExceeded)
	}
}
