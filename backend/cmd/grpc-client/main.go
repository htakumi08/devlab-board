package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	grpcv1 "devlab-board/backend/gen/grpclab/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := runCLI(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// runCLI はコマンド引数を読み取り、1回のHello RPCの結果を出力する。
func runCLI(parent context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("grpc-client", flag.ContinueOnError)
	flags.SetOutput(output)

	target := flags.String("target", "127.0.0.1:50051", "gRPC server address")
	name := flags.String("name", "Taro", "name sent to GreetingService.Hello")
	timeout := flags.Duration("timeout", 3*time.Second, "RPC timeout")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse command flags: %w", err)
	}

	message, err := run(parent, *target, *name, *timeout)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(output, message); err != nil {
		return fmt.Errorf("write greeting: %w", err)
	}

	return nil
}

// run はgRPCの通信路と型付きクライアントを作り、Hello RPCを呼び出す。
func run(
	parent context.Context,
	target string,
	name string,
	timeout time.Duration,
) (string, error) {
	// NewClientは仮想的な通信路を作る。実際の接続は通常、最初のRPCで開始される。
	connection, err := grpc.NewClient(
		target,
		// 現在のローカル学習環境はTLSを構成していないため、plaintextで接続する。
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return "", fmt.Errorf("create gRPC client connection: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			log.Printf("close gRPC client connection: %v", closeErr)
		}
	}()

	client := grpcv1.NewGreetingServiceClient(connection)

	return callHello(parent, client, name, timeout)
}

// callHello は指定された期限内にHello RPCを1回呼び出し、応答メッセージを返す。
func callHello(
	parent context.Context,
	client grpcv1.GreetingServiceClient,
	name string,
	timeout time.Duration,
) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	response, err := client.Hello(
		ctx,
		&grpcv1.HelloRequest{Name: name},
	)
	if err != nil {
		return "", fmt.Errorf("call GreetingService.Hello: %w", err)
	}

	return response.GetMessage(), nil
}
