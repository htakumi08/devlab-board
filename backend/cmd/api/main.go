package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"devlab-board/backend/internal/app"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// DB 疎通や migration を含む初期化が無期限に待たないよう制限する。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 設定をもとに DB、session store、HTTP handler を組み立てる。
	server, err := app.NewServer(ctx, app.LoadConfig())
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	defer server.Close()

	log.Printf("starting devlab-board backend on :%s", port)
	// ここからリクエストを待ち受け、サーバー停止またはエラーまで処理を継続する。
	if err := http.ListenAndServe(":"+port, server.Handler); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// newMux は外部 DB なしで HTTP handler を検証するためのテスト用入口を返す。
func newMux() http.Handler {
	return app.NewTestHandler()
}
