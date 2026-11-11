// Package finance contains Finance Dashboard use cases and persistence boundaries.
package finance

import (
	"context"
	"time"
)

// Summary is the current user's currency-separated balance and recent transaction view.
type Summary struct {
	AccountCount       int64         `json:"accountCount"`
	Balances           []Balance     `json:"balances"`
	RecentTransactions []Transaction `json:"recentTransactions"`
	AsOf               *time.Time    `json:"asOf"`
}

// MinorAmount is an exact base-10 minor-unit value serialized as a JSON string.
// PostgreSQL SUM(bigint) returns numeric and can exceed both int64 and JavaScript's safe integer range.
type MinorAmount string

// Balance represents one currency group without converting between currencies.
type Balance struct {
	Currency             string       `json:"currency"`
	CurrentAmountMinor   MinorAmount  `json:"currentAmountMinor"`
	AvailableAmountMinor *MinorAmount `json:"availableAmountMinor"`
}

// Category is the optional application-facing classification of a transaction.
type Category struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Transaction is a recent transaction DTO containing public IDs only.
type Transaction struct {
	ID          string      `json:"id"`
	AccountID   string      `json:"accountId"`
	AccountName string      `json:"accountName"`
	Name        string      `json:"name"`
	Merchant    *string     `json:"merchant"`
	AmountMinor MinorAmount `json:"amountMinor"`
	Currency    string      `json:"currency"`
	Direction   string      `json:"direction"`
	Status      string      `json:"status"`
	Category    *Category   `json:"category"`
	OccurredAt  time.Time   `json:"occurredAt"`
}

// Repository defines Finance summary persistence operations.
type Repository interface {
	Summary(ctx context.Context, userID int64) (Summary, error)
}

// Service coordinates Finance summary use cases independently of the HTTP and PostgreSQL adapters.
type Service struct {
	repository Repository
}

// NewService constructs the Finance summary use case.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Summary loads only data owned by the authenticated internal user ID.
func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	return s.repository.Summary(ctx, userID)
}
