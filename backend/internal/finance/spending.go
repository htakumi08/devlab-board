package finance

import (
	"context"
	"errors"
	"time"
)

const maxSpendingPeriodDays = 366

// ErrInvalidSpendingQuery is returned when the requested UTC date range is invalid.
var ErrInvalidSpendingQuery = errors.New("invalid finance spending query")

// SpendingSummaryRequest is the public date range accepted by the spending summary use case.
type SpendingSummaryRequest struct {
	DateFrom string
	DateTo   string
}

// SpendingSummary is a currency-separated view of posted debit transactions.
type SpendingSummary struct {
	DateFrom   string             `json:"dateFrom"`
	DateTo     string             `json:"dateTo"`
	Timezone   string             `json:"timezone"`
	Currencies []SpendingCurrency `json:"currencies"`
	AsOf       *time.Time         `json:"asOf"`
}

// SpendingCurrency keeps totals separate when an owner has multiple currencies.
type SpendingCurrency struct {
	Currency         string             `json:"currency"`
	TotalAmountMinor MinorAmount        `json:"totalAmountMinor"`
	Categories       []SpendingCategory `json:"categories"`
}

// SpendingCategory is one category contribution within a currency total.
type SpendingCategory struct {
	Category         *Category   `json:"category"`
	AmountMinor      MinorAmount `json:"amountMinor"`
	TransactionCount int64       `json:"transactionCount"`
}

type spendingSummaryQuery struct {
	DateFrom        time.Time
	DateToExclusive time.Time
}

// SpendingRepository defines the owner-scoped persistence operation used by spending aggregation.
type SpendingRepository interface {
	SpendingSummary(ctx context.Context, userID int64, query spendingSummaryQuery) (SpendingSummary, error)
}

// SpendingService validates the public period before crossing the persistence boundary.
type SpendingService struct {
	repository SpendingRepository
}

// NewSpendingService constructs the Finance spending aggregation use case.
func NewSpendingService(repository SpendingRepository) *SpendingService {
	return &SpendingService{repository: repository}
}

// SpendingSummary returns the authenticated owner's posted debit totals for one UTC date range.
func (s *SpendingService) SpendingSummary(ctx context.Context, userID int64, request SpendingSummaryRequest) (SpendingSummary, error) {
	dateFrom, err := parseTransactionDate(request.DateFrom)
	if err != nil {
		return SpendingSummary{}, ErrInvalidSpendingQuery
	}
	dateTo, err := parseTransactionDate(request.DateTo)
	if err != nil {
		return SpendingSummary{}, ErrInvalidSpendingQuery
	}
	dateToExclusive := dateTo.AddDate(0, 0, 1)
	periodDays := int(dateToExclusive.Sub(dateFrom) / (24 * time.Hour))
	if periodDays < 1 || periodDays > maxSpendingPeriodDays {
		return SpendingSummary{}, ErrInvalidSpendingQuery
	}

	summary, err := s.repository.SpendingSummary(ctx, userID, spendingSummaryQuery{
		DateFrom:        dateFrom,
		DateToExclusive: dateToExclusive,
	})
	if err != nil {
		return SpendingSummary{}, err
	}
	if summary.Currencies == nil {
		summary.Currencies = make([]SpendingCurrency, 0)
	}
	summary.DateFrom = request.DateFrom
	summary.DateTo = request.DateTo
	summary.Timezone = "UTC"
	return summary, nil
}
