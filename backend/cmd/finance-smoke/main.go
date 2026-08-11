package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"devlab-board/backend/internal/financesmoke"
)

const defaultSmokeTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("finance MVP smoke failed: %v", err)
	}
	log.Println("finance MVP smoke succeeded")
}

func run() error {
	timeout := defaultSmokeTimeout
	if value := os.Getenv("FINANCE_SMOKE_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("FINANCE_SMOKE_TIMEOUT is invalid")
		}
		timeout = parsed
	}
	return financesmoke.Run(context.Background(), financesmoke.Config{
		FrontendURL:       os.Getenv("FINANCE_SMOKE_FRONTEND_URL"),
		APIURL:            os.Getenv("FINANCE_SMOKE_API_URL"),
		Email:             os.Getenv("FINANCE_SMOKE_EMAIL"),
		Password:          os.Getenv("FINANCE_SMOKE_PASSWORD"),
		SecondaryEmail:    os.Getenv("FINANCE_SMOKE_SECONDARY_EMAIL"),
		SecondaryPassword: os.Getenv("FINANCE_SMOKE_SECONDARY_PASSWORD"),
		Timeout:           timeout,
	})
}
