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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := app.NewServer(ctx, app.LoadConfig())
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	defer server.Close()

	log.Printf("starting devlab-board backend on :%s", port)
	if err := http.ListenAndServe(":"+port, server.Handler); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func newMux() http.Handler {
	return app.NewTestHandler()
}
