package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devlab-board/backend/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run backend: %v", err)
	}
}

func run() error {
	config := app.LoadConfig()

	// DB 疎通や migration を含む初期化が無期限に待たないよう制限する。
	initCtx, cancelInit := context.WithTimeout(context.Background(), 10*time.Second)

	// 設定をもとに DB、session store、HTTP handler を組み立てる。
	appServer, err := app.NewServer(initCtx, config)
	cancelInit()
	if err != nil {
		return fmt.Errorf("initialize application server: %w", err)
	}
	defer func() {
		if err := appServer.Close(); err != nil {
			log.Printf("close application server: %v", err)
		}
	}()

	httpListener, err := newHTTPListener(config.HTTPPort)
	if err != nil {
		return err
	}
	defer closeListener(httpListener)

	grpcListener, err := newGRPCListener(config.GRPCPort)
	if err != nil {
		return err
	}
	defer closeListener(grpcListener)

	httpServer := &http.Server{
		Handler:           appServer.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	grpcServer := newGRPCServer()

	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	log.Printf("starting HTTP server on :%s", config.HTTPPort)
	log.Printf("starting gRPC server on :%s", config.GRPCPort)

	results := startServers(httpServer, httpListener, grpcServer, grpcListener)
	serveErr := waitForShutdown(signalCtx, results)
	stopSignals()

	if serveErr == nil {
		log.Println("shutdown signal received")
	} else {
		log.Printf("server stopped before shutdown signal: %v", serveErr)
	}

	// シグナル用contextは既にcancelされているため、終了処理専用の期限を新しく作る。
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	log.Println("gracefully shutting down HTTP and gRPC servers")
	shutdownErr := shutdownServers(shutdownCtx, httpServer, grpcServer)
	if shutdownErr == nil {
		log.Println("HTTP and gRPC servers stopped")
	}

	return errors.Join(serveErr, shutdownErr)
}

// newMux は外部 DB なしで HTTP handler を検証するためのテスト用入口を返す。
func newMux() http.Handler {
	return app.NewTestHandler()
}

func closeListener(listener net.Listener) {
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Printf("close listener %s: %v", listener.Addr(), err)
	}
}
