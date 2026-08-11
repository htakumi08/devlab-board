package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"devlab-board/backend/internal/financeseed"
	platformpostgres "devlab-board/backend/internal/platform/postgres"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("finance seed failed: %v", err)
	}
}

func run() error {
	config := financeseed.Config{
		AppEnv:    os.Getenv("APP_ENV"),
		UserEmail: os.Getenv("FINANCE_SEED_USER_EMAIL"),
		Action:    financeseed.Action(os.Getenv("FINANCE_SEED_ACTION")),
	}
	if err := financeseed.ValidateConfig(config); err != nil {
		return err
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("open finance seed database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect finance seed database: %w", err)
	}
	if err := platformpostgres.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate finance seed database: %w", err)
	}
	result, err := financeseed.Run(ctx, db, config)
	if err != nil {
		return err
	}
	log.Printf("finance synthetic fixtures completed: action=%s accounts=%d transactions=%d", config.Action, result.Accounts, result.Transactions)
	return nil
}
