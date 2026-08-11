package finance

import (
	"context"
	"database/sql"
	"time"
)

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// PostgresRepository reads Finance data from PostgreSQL.
type PostgresRepository struct {
	db queryer
}

// NewPostgresRepository constructs the PostgreSQL Finance adapter.
func NewPostgresRepository(db queryer) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Summary performs every query with user_id so ownership is enforced at the data-access boundary.
func (r *PostgresRepository) Summary(ctx context.Context, userID int64) (Summary, error) {
	summary := Summary{
		Balances:           make([]Balance, 0),
		RecentTransactions: make([]Transaction, 0),
	}

	balanceRows, err := r.db.QueryContext(ctx, `
		SELECT
			currency,
			COUNT(*),
			SUM(current_balance_minor)::text,
			CASE
				WHEN COUNT(available_balance_minor) = COUNT(*) THEN SUM(available_balance_minor)::text
				ELSE NULL
			END,
			MIN(balance_as_of)
		FROM finance_accounts
		WHERE user_id = $1 AND status = 'active'
		GROUP BY currency
		ORDER BY currency
	`, userID)
	if err != nil {
		return Summary{}, err
	}
	for balanceRows.Next() {
		var balance Balance
		var accountCount int64
		var available sql.NullString
		var asOf time.Time
		if err := balanceRows.Scan(
			&balance.Currency,
			&accountCount,
			&balance.CurrentAmountMinor,
			&available,
			&asOf,
		); err != nil {
			balanceRows.Close()
			return Summary{}, err
		}
		if available.Valid {
			amount := MinorAmount(available.String)
			balance.AvailableAmountMinor = &amount
		}
		summary.AccountCount += accountCount
		if summary.AsOf == nil || asOf.Before(*summary.AsOf) {
			summary.AsOf = &asOf
		}
		summary.Balances = append(summary.Balances, balance)
	}
	if err := balanceRows.Err(); err != nil {
		balanceRows.Close()
		return Summary{}, err
	}
	balanceRows.Close()

	transactionRows, err := r.db.QueryContext(ctx, `
		SELECT
			t.public_id::text,
			a.public_id::text,
			a.name,
			t.name,
			t.merchant,
			t.amount_minor::text,
			t.currency,
			t.direction,
			t.status,
			c.code,
			c.label,
			COALESCE(t.posted_at, t.authorized_at)
		FROM finance_transactions t
		JOIN finance_accounts a
			ON a.id = t.account_id AND a.user_id = t.user_id
		LEFT JOIN finance_transaction_categories c ON c.code = t.category_code
		WHERE t.user_id = $1
		ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return Summary{}, err
	}
	defer transactionRows.Close()

	for transactionRows.Next() {
		var transaction Transaction
		var merchant sql.NullString
		var categoryCode sql.NullString
		var categoryLabel sql.NullString
		if err := transactionRows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.AccountName,
			&transaction.Name,
			&merchant,
			&transaction.AmountMinor,
			&transaction.Currency,
			&transaction.Direction,
			&transaction.Status,
			&categoryCode,
			&categoryLabel,
			&transaction.OccurredAt,
		); err != nil {
			return Summary{}, err
		}
		if merchant.Valid {
			transaction.Merchant = &merchant.String
		}
		if categoryCode.Valid && categoryLabel.Valid {
			transaction.Category = &Category{Code: categoryCode.String, Label: categoryLabel.String}
		}
		summary.RecentTransactions = append(summary.RecentTransactions, transaction)
	}
	if err := transactionRows.Err(); err != nil {
		return Summary{}, err
	}
	return summary, nil
}
