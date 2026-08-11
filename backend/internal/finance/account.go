package finance

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ErrAccountNotFound hides whether an account is absent, unowned, or addressed by an invalid public ID.
var ErrAccountNotFound = errors.New("finance account not found")

// Account is the public Finance account read model and never contains internal ownership identifiers.
type Account struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	AccountType          string       `json:"accountType"`
	Mask                 string       `json:"mask"`
	Currency             string       `json:"currency"`
	CurrentAmountMinor   MinorAmount  `json:"currentAmountMinor"`
	AvailableAmountMinor *MinorAmount `json:"availableAmountMinor"`
	Status               string       `json:"status"`
	BalanceAsOf          time.Time    `json:"balanceAsOf"`
}

// AccountDetail combines one public account with a bounded recent-transaction preview.
type AccountDetail struct {
	Account
	RecentTransactions []Transaction `json:"recentTransactions"`
}

// AccountRepository defines only the persistence operations required by account list and detail use cases.
type AccountRepository interface {
	Accounts(ctx context.Context, userID int64) ([]Account, error)
	Account(ctx context.Context, userID int64, publicAccountID string) (AccountDetail, error)
}

// AccountService coordinates Finance account reads independently of the HTTP adapter.
type AccountService struct {
	repository AccountRepository
}

// NewAccountService constructs the Finance account read use cases.
func NewAccountService(repository AccountRepository) *AccountService {
	return &AccountService{repository: repository}
}

// Accounts loads every account owned by the authenticated internal user ID.
func (s *AccountService) Accounts(ctx context.Context, userID int64) ([]Account, error) {
	return s.repository.Accounts(ctx, userID)
}

// Account rejects malformed public IDs before querying and otherwise preserves the owner boundary.
func (s *AccountService) Account(ctx context.Context, userID int64, publicAccountID string) (AccountDetail, error) {
	if !isCanonicalUUID(publicAccountID) {
		return AccountDetail{}, ErrAccountNotFound
	}
	return s.repository.Account(ctx, userID, publicAccountID)
}

func isCanonicalUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(compact)
	return err == nil
}
