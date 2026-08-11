package finance

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// TransactionCategories returns the application-facing category master in display order.
func (r *PostgresRepository) TransactionCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT code, label
		FROM finance_transaction_categories
		ORDER BY display_order, code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.Code, &category.Label); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

// Transactions returns a filtered page and resolves its account and cursor within the same owner boundary.
func (r *PostgresRepository) Transactions(ctx context.Context, userID int64, query transactionPageQuery) (transactionPageResult, error) {
	if query.AccountPublicID != nil {
		accountID, found, err := r.resolveTransactionAccountID(ctx, userID, *query.AccountPublicID)
		if err != nil {
			return transactionPageResult{}, err
		}
		if !found {
			if query.CursorPublicID != nil {
				return transactionPageResult{}, ErrInvalidTransactionQuery
			}
			return transactionPageResult{Transactions: make([]Transaction, 0)}, nil
		}
		query.AccountInternalID = &accountID
	}

	var cursorOccurredAt time.Time
	var cursorInternalID int64
	if query.CursorPublicID != nil {
		cursorStatement := strings.Builder{}
		cursorStatement.WriteString(`
			SELECT COALESCE(t.posted_at, t.authorized_at), t.id
			FROM finance_transactions t
			JOIN finance_accounts a
				ON a.id = t.account_id AND a.user_id = t.user_id
			WHERE t.user_id = $1 AND t.public_id = $2::uuid`)
		cursorArguments := []any{userID, *query.CursorPublicID}
		appendTransactionFilters(&cursorStatement, &cursorArguments, query)
		cursorRows, err := r.db.QueryContext(ctx, cursorStatement.String(), cursorArguments...)
		if err != nil {
			return transactionPageResult{}, err
		}
		if !cursorRows.Next() {
			err := cursorRows.Err()
			cursorRows.Close()
			if err != nil {
				return transactionPageResult{}, err
			}
			return transactionPageResult{}, ErrInvalidTransactionQuery
		}
		if err := cursorRows.Scan(&cursorOccurredAt, &cursorInternalID); err != nil {
			cursorRows.Close()
			return transactionPageResult{}, err
		}
		if err := cursorRows.Err(); err != nil {
			cursorRows.Close()
			return transactionPageResult{}, err
		}
		if err := cursorRows.Close(); err != nil {
			return transactionPageResult{}, err
		}
	}

	statement := strings.Builder{}
	statement.WriteString(`
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
		WHERE t.user_id = $1`)
	arguments := []any{userID}
	appendTransactionFilters(&statement, &arguments, query)

	comparison := "<"
	order := "DESC"
	if query.Sort == transactionSortOldest {
		comparison = ">"
		order = "ASC"
	}
	if query.CursorPublicID != nil {
		statement.WriteString(`
			AND (COALESCE(t.posted_at, t.authorized_at), t.id) ` + comparison + ` ($` + strconv.Itoa(len(arguments)+1) + `, $` + strconv.Itoa(len(arguments)+2) + `)`)
		arguments = append(arguments, cursorOccurredAt, cursorInternalID)
	}
	statement.WriteString(`
		ORDER BY COALESCE(t.posted_at, t.authorized_at) ` + order + `, t.id ` + order + `
		LIMIT $` + strconv.Itoa(len(arguments)+1))
	arguments = append(arguments, query.Limit+1)

	rows, err := r.db.QueryContext(ctx, statement.String(), arguments...)
	if err != nil {
		return transactionPageResult{}, err
	}
	defer rows.Close()

	transactions := make([]Transaction, 0, query.Limit+1)
	for rows.Next() {
		var transaction Transaction
		var merchant sql.NullString
		var categoryCode sql.NullString
		var categoryLabel sql.NullString
		if err := rows.Scan(
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
			return transactionPageResult{}, err
		}
		if merchant.Valid {
			transaction.Merchant = &merchant.String
		}
		if categoryCode.Valid && categoryLabel.Valid {
			transaction.Category = &Category{Code: categoryCode.String, Label: categoryLabel.String}
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return transactionPageResult{}, err
	}

	result := transactionPageResult{Transactions: transactions}
	if len(transactions) > query.Limit {
		result.HasMore = true
		result.Transactions = transactions[:query.Limit]
	}
	return result, nil
}

func (r *PostgresRepository) resolveTransactionAccountID(ctx context.Context, userID int64, publicID string) (int64, bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id
		FROM finance_accounts
		WHERE user_id = $1 AND public_id = $2::uuid
	`, userID, publicID)
	if err != nil {
		return 0, false, err
	}
	if !rows.Next() {
		err := rows.Err()
		rows.Close()
		if err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	var accountID int64
	if err := rows.Scan(&accountID); err != nil {
		rows.Close()
		return 0, false, err
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, false, err
	}
	if err := rows.Close(); err != nil {
		return 0, false, err
	}
	return accountID, true, nil
}

func appendTransactionFilters(statement *strings.Builder, arguments *[]any, query transactionPageQuery) {
	appendValue := func(column string, value any, cast string) {
		statement.WriteString("\n\t\t\tAND " + column + " = $" + strconv.Itoa(len(*arguments)+1) + cast)
		*arguments = append(*arguments, value)
	}

	if query.AccountInternalID != nil {
		appendValue("t.account_id", *query.AccountInternalID, "")
	}
	if query.DateFrom != nil {
		statement.WriteString("\n\t\t\tAND COALESCE(t.posted_at, t.authorized_at) >= $" + strconv.Itoa(len(*arguments)+1))
		*arguments = append(*arguments, *query.DateFrom)
	}
	if query.DateToExclusive != nil {
		statement.WriteString("\n\t\t\tAND COALESCE(t.posted_at, t.authorized_at) < $" + strconv.Itoa(len(*arguments)+1))
		*arguments = append(*arguments, *query.DateToExclusive)
	}
	if query.Category != nil {
		if *query.Category == "uncategorized" {
			statement.WriteString("\n\t\t\tAND t.category_code IS NULL")
		} else {
			appendValue("t.category_code", *query.Category, "")
		}
	}
	if query.Direction != nil {
		appendValue("t.direction", *query.Direction, "")
	}
	if query.Status != nil {
		appendValue("t.status", *query.Status, "")
	}
}
