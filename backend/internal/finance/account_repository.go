package finance

import (
	"context"
	"database/sql"
)

const accountRecentTransactionLimit = 5

// Accounts returns all owner-scoped accounts with active accounts first and a stable display order.
func (r *PostgresRepository) Accounts(ctx context.Context, userID int64) ([]Account, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			public_id::text,
			name,
			account_type,
			mask,
			currency,
			current_balance_minor::text,
			available_balance_minor::text,
			status,
			balance_as_of
		FROM finance_accounts
		WHERE user_id = $1
		ORDER BY (status = 'active') DESC, name, public_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		account, _, err := scanAccount(rows, false)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

// Account returns one owner-scoped account and a fixed recent-transaction preview.
func (r *PostgresRepository) Account(ctx context.Context, userID int64, publicAccountID string) (AccountDetail, error) {
	accountRows, err := r.db.QueryContext(ctx, `
		SELECT
			public_id::text,
			name,
			account_type,
			mask,
			currency,
			current_balance_minor::text,
			available_balance_minor::text,
			status,
			balance_as_of,
			id
		FROM finance_accounts
		WHERE user_id = $1 AND public_id = $2::uuid
	`, userID, publicAccountID)
	if err != nil {
		return AccountDetail{}, err
	}

	if !accountRows.Next() {
		err := accountRows.Err()
		accountRows.Close()
		if err != nil {
			return AccountDetail{}, err
		}
		return AccountDetail{}, ErrAccountNotFound
	}
	account, internalAccountID, err := scanAccount(accountRows, true)
	if err != nil {
		accountRows.Close()
		return AccountDetail{}, err
	}
	if err := accountRows.Err(); err != nil {
		accountRows.Close()
		return AccountDetail{}, err
	}
	if err := accountRows.Close(); err != nil {
		return AccountDetail{}, err
	}

	transactionRows, err := r.db.QueryContext(ctx, `
		SELECT
			public_id::text,
			name,
			merchant,
			amount_minor::text,
			currency,
			direction,
			status,
			c.code,
			c.label,
			COALESCE(posted_at, authorized_at)
		FROM finance_transactions t
		LEFT JOIN finance_transaction_categories c ON c.code = t.category_code
		WHERE t.account_id = $1 AND t.user_id = $2
		ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
		LIMIT $3
	`, internalAccountID, userID, accountRecentTransactionLimit)
	if err != nil {
		return AccountDetail{}, err
	}
	defer transactionRows.Close()

	detail := AccountDetail{Account: account, RecentTransactions: make([]Transaction, 0)}
	for transactionRows.Next() {
		var transaction Transaction
		var merchant sql.NullString
		var categoryCode sql.NullString
		var categoryLabel sql.NullString
		if err := transactionRows.Scan(
			&transaction.ID,
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
			return AccountDetail{}, err
		}
		transaction.AccountID = account.ID
		transaction.AccountName = account.Name
		if merchant.Valid {
			transaction.Merchant = &merchant.String
		}
		if categoryCode.Valid && categoryLabel.Valid {
			transaction.Category = &Category{Code: categoryCode.String, Label: categoryLabel.String}
		}
		detail.RecentTransactions = append(detail.RecentTransactions, transaction)
	}
	if err := transactionRows.Err(); err != nil {
		return AccountDetail{}, err
	}
	return detail, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner, includeInternalID bool) (Account, int64, error) {
	var account Account
	var available sql.NullString
	var internalAccountID sql.NullInt64
	destinations := []any{
		&account.ID,
		&account.Name,
		&account.AccountType,
		&account.Mask,
		&account.Currency,
		&account.CurrentAmountMinor,
		&available,
		&account.Status,
		&account.BalanceAsOf,
	}
	if includeInternalID {
		destinations = append(destinations, &internalAccountID)
	}
	if err := row.Scan(destinations...); err != nil {
		return Account{}, 0, err
	}
	if available.Valid {
		amount := MinorAmount(available.String)
		account.AvailableAmountMinor = &amount
	}
	return account, internalAccountID.Int64, nil
}
