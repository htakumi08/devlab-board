package finance

import (
	"context"
	"database/sql"
	"time"
)

// SpendingSummary aggregates only posted debits and keeps owner and currency boundaries in SQL.
func (r *PostgresRepository) SpendingSummary(ctx context.Context, userID int64, query spendingSummaryQuery) (SpendingSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			t.currency,
			c.code,
			c.label,
			SUM(t.amount_minor)::text,
			COUNT(*),
			SUM(SUM(t.amount_minor)) OVER (PARTITION BY t.currency)::text,
			MAX(MAX(t.updated_at)) OVER ()
		FROM finance_transactions t
		JOIN finance_accounts a
			ON a.id = t.account_id AND a.user_id = t.user_id
		LEFT JOIN finance_transaction_categories c ON c.code = t.category_code
		WHERE t.user_id = $1
			AND t.direction = 'debit'
			AND t.status = 'posted'
			AND COALESCE(t.posted_at, t.authorized_at) >= $2
			AND COALESCE(t.posted_at, t.authorized_at) < $3
		GROUP BY t.currency, c.code, c.label, c.display_order
		ORDER BY t.currency, SUM(t.amount_minor) DESC, c.display_order NULLS LAST, c.code NULLS LAST
	`, userID, query.DateFrom, query.DateToExclusive)
	if err != nil {
		return SpendingSummary{}, err
	}
	defer rows.Close()

	summary := SpendingSummary{Currencies: make([]SpendingCurrency, 0)}
	for rows.Next() {
		var currency string
		var categoryCode sql.NullString
		var categoryLabel sql.NullString
		var category SpendingCategory
		var total MinorAmount
		var updatedAt time.Time
		if err := rows.Scan(
			&currency,
			&categoryCode,
			&categoryLabel,
			&category.AmountMinor,
			&category.TransactionCount,
			&total,
			&updatedAt,
		); err != nil {
			return SpendingSummary{}, err
		}
		if categoryCode.Valid && categoryLabel.Valid {
			category.Category = &Category{Code: categoryCode.String, Label: categoryLabel.String}
		}
		if summary.AsOf == nil || updatedAt.After(*summary.AsOf) {
			summary.AsOf = &updatedAt
		}

		lastIndex := len(summary.Currencies) - 1
		if lastIndex < 0 || summary.Currencies[lastIndex].Currency != currency {
			summary.Currencies = append(summary.Currencies, SpendingCurrency{
				Currency:         currency,
				TotalAmountMinor: total,
				Categories:       make([]SpendingCategory, 0),
			})
			lastIndex++
		}
		summary.Currencies[lastIndex].Categories = append(summary.Currencies[lastIndex].Categories, category)
	}
	if err := rows.Err(); err != nil {
		return SpendingSummary{}, err
	}
	return summary, nil
}
