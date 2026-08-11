// Package financeseed manages deterministic synthetic Finance fixtures for non-production environments.
package financeseed

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Action string

const (
	ActionApply Action = "apply"
	ActionClean Action = "clean"

	seedAccountCount     = 2
	seedTransactionCount = 40
)

var (
	ErrInvalidConfig     = errors.New("invalid finance seed configuration")
	ErrUnsafeEnvironment = errors.New("finance seed is disabled in this environment")
	ErrUserNotFound      = errors.New("finance seed user was not found")
)

type Config struct {
	AppEnv    string
	UserEmail string
	Action    Action
}

type Result struct {
	Accounts     int
	Transactions int
}

type fixtureAccount struct {
	PublicID       string
	Name           string
	AccountType    string
	Mask           string
	Currency       string
	CurrentMinor   int64
	AvailableMinor int64
	BalanceAsOf    time.Time
}

type fixtureTransaction struct {
	PublicID     string
	AccountID    int64
	Name         string
	Merchant     *string
	AmountMinor  int64
	Currency     string
	Direction    string
	Status       string
	CategoryCode *string
	AuthorizedAt time.Time
	PostedAt     *time.Time
}

func ValidateConfig(config Config) error {
	switch config.AppEnv {
	case "local", "development", "test":
	case "":
		return fmt.Errorf("%w: APP_ENV is required", ErrUnsafeEnvironment)
	default:
		return ErrUnsafeEnvironment
	}
	if strings.TrimSpace(config.UserEmail) == "" {
		return fmt.Errorf("%w: FINANCE_SEED_USER_EMAIL is required", ErrInvalidConfig)
	}
	if config.Action != ActionApply && config.Action != ActionClean {
		return fmt.Errorf("%w: FINANCE_SEED_ACTION must be apply or clean", ErrInvalidConfig)
	}
	return nil
}

// Run applies or removes only the deterministic fixture accounts owned by the configured existing user.
func Run(ctx context.Context, db *sql.DB, config Config) (Result, error) {
	if err := ValidateConfig(config); err != nil {
		return Result{}, err
	}
	if db == nil {
		return Result{}, errors.New("finance seed database is unavailable")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin finance seed transaction: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	var userPublicID string
	err = tx.QueryRowContext(ctx, `
		SELECT id, public_id::text
		FROM users
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(config.UserEmail))).Scan(&userID, &userPublicID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrUserNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("find finance seed user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userPublicID); err != nil {
		return Result{}, fmt.Errorf("lock finance seed user: %w", err)
	}

	accountPublicIDs := []string{
		deterministicUUID(userPublicID, "account-checking"),
		deterministicUUID(userPublicID, "account-savings"),
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM finance_accounts
		WHERE user_id = $1 AND public_id = ANY($2::uuid[])
	`, userID, pq.Array(accountPublicIDs)); err != nil {
		return Result{}, fmt.Errorf("remove existing finance seed fixtures: %w", err)
	}

	if config.Action == ActionClean {
		if err := tx.Commit(); err != nil {
			return Result{}, fmt.Errorf("commit finance seed cleanup: %w", err)
		}
		return Result{}, nil
	}

	accounts := buildAccounts(accountPublicIDs)
	accountIDs := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		var accountID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO finance_accounts (
				public_id, user_id, name, account_type, mask, currency,
				current_balance_minor, available_balance_minor, status, balance_as_of
			)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, 'active', $9)
			RETURNING id
		`, account.PublicID, userID, account.Name, account.AccountType, account.Mask,
			account.Currency, account.CurrentMinor, account.AvailableMinor, account.BalanceAsOf,
		).Scan(&accountID)
		if err != nil {
			return Result{}, fmt.Errorf("insert finance seed account: %w", err)
		}
		accountIDs = append(accountIDs, accountID)
	}

	for _, transaction := range buildTransactions(userPublicID, accountIDs) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_transactions (
				public_id, user_id, account_id, name, merchant, amount_minor, currency,
				direction, status, category_code, authorized_at, posted_at
			)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, transaction.PublicID, userID, transaction.AccountID, transaction.Name,
			transaction.Merchant, transaction.AmountMinor, transaction.Currency,
			transaction.Direction, transaction.Status, transaction.CategoryCode,
			transaction.AuthorizedAt, transaction.PostedAt,
		); err != nil {
			return Result{}, fmt.Errorf("insert finance seed transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit finance seed fixtures: %w", err)
	}
	return Result{Accounts: seedAccountCount, Transactions: seedTransactionCount}, nil
}

func buildAccounts(publicIDs []string) []fixtureAccount {
	baseTime := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)
	return []fixtureAccount{
		{
			PublicID: publicIDs[0], Name: "Synthetic Everyday", AccountType: "checking",
			Mask: "4242", Currency: "JPY", CurrentMinor: 245000, AvailableMinor: 220000,
			BalanceAsOf: baseTime,
		},
		{
			PublicID: publicIDs[1], Name: "Synthetic Reserve", AccountType: "savings",
			Mask: "0088", Currency: "USD", CurrentMinor: 125075, AvailableMinor: 125075,
			BalanceAsOf: baseTime,
		},
	}
}

func buildTransactions(userPublicID string, accountIDs []int64) []fixtureTransaction {
	baseTime := time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)
	categories := []*string{
		stringPointer("income"), stringPointer("groceries"), stringPointer("transportation"),
		stringPointer("utilities"), stringPointer("other"), nil,
	}
	transactions := make([]fixtureTransaction, 0, seedTransactionCount)
	for index := 0; index < seedTransactionCount; index++ {
		accountIndex := index % len(accountIDs)
		authorizedAt := baseTime.Add(-time.Duration(index) * 24 * time.Hour)
		status := []string{"posted", "pending", "reversed"}[index%3]
		if index < 2 {
			authorizedAt = baseTime
			status = "pending"
		}
		var postedAt *time.Time
		if status != "pending" {
			value := authorizedAt.Add(2 * time.Hour)
			postedAt = &value
		}
		var merchant *string
		if index%7 != 0 {
			merchant = stringPointer("Synthetic Merchant")
		}
		amount := int64((index + 1) * 137)
		if index == 0 {
			amount = 9007199254740993
		}
		transactions = append(transactions, fixtureTransaction{
			PublicID:  deterministicUUID(userPublicID, fmt.Sprintf("transaction-%02d", index)),
			AccountID: accountIDs[accountIndex], Name: fmt.Sprintf("Synthetic Transaction %02d", index+1),
			Merchant: merchant, AmountMinor: amount,
			Currency:  []string{"JPY", "USD"}[accountIndex],
			Direction: []string{"debit", "credit"}[index%2], Status: status,
			CategoryCode: categories[index%len(categories)], AuthorizedAt: authorizedAt, PostedAt: postedAt,
		})
	}
	return transactions
}

func deterministicUUID(namespace, key string) string {
	digest := sha256.Sum256([]byte("devlab-board/finance-seed/" + namespace + "/" + key))
	value := digest[:16]
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func stringPointer(value string) *string {
	return &value
}
