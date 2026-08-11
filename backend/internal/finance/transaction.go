package finance

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"time"
)

const (
	defaultTransactionPageLimit = 25
	maxTransactionPageLimit     = 100
)

// ErrInvalidTransactionQuery intentionally combines malformed, missing, and unowned cursor failures.
var ErrInvalidTransactionQuery = errors.New("invalid finance transactions query")

// TransactionListRequest preserves whether optional HTTP query values were omitted or explicitly empty.
type TransactionListRequest struct {
	AccountID *string
	DateFrom  *string
	DateTo    *string
	Category  *string
	Direction *string
	Status    *string
	Sort      *string
	Cursor    *string
	Limit     *string
}

// TransactionPage is one owner-scoped page and an opaque cursor for the following page.
type TransactionPage struct {
	Transactions []Transaction `json:"transactions"`
	NextCursor   *string       `json:"nextCursor"`
}

type transactionPageQuery struct {
	CursorPublicID    *string
	AccountPublicID   *string
	AccountInternalID *int64
	DateFrom          *time.Time
	DateToExclusive   *time.Time
	Category          *string
	Direction         *string
	Status            *string
	Sort              transactionSort
	Limit             int
}

type transactionSort string

const (
	transactionSortNewest transactionSort = "newest"
	transactionSortOldest transactionSort = "oldest"
)

type transactionPageResult struct {
	Transactions []Transaction
	HasMore      bool
}

// TransactionRepository defines the owner-scoped persistence operation used by transaction history.
type TransactionRepository interface {
	Transactions(ctx context.Context, userID int64, query transactionPageQuery) (transactionPageResult, error)
	TransactionCategories(ctx context.Context) ([]Category, error)
}

// TransactionService validates the public query before crossing the persistence boundary.
type TransactionService struct {
	repository TransactionRepository
}

// NewTransactionService constructs the Finance transaction list use case.
func NewTransactionService(repository TransactionRepository) *TransactionService {
	return &TransactionService{repository: repository}
}

// Transactions validates filter and sort input before loading one authenticated owner-scoped page.
func (s *TransactionService) Transactions(ctx context.Context, userID int64, request TransactionListRequest) (TransactionPage, error) {
	query := transactionPageQuery{Limit: defaultTransactionPageLimit, Sort: transactionSortNewest}
	if request.AccountID != nil {
		if !isCanonicalUUID(*request.AccountID) {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.AccountPublicID = request.AccountID
	}
	if request.DateFrom != nil {
		dateFrom, parseErr := parseTransactionDate(*request.DateFrom)
		if parseErr != nil {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.DateFrom = &dateFrom
	}
	if request.DateTo != nil {
		dateTo, parseErr := parseTransactionDate(*request.DateTo)
		if parseErr != nil {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		dateToExclusive := dateTo.AddDate(0, 0, 1)
		query.DateToExclusive = &dateToExclusive
	}
	if query.DateFrom != nil && query.DateToExclusive != nil && !query.DateFrom.Before(*query.DateToExclusive) {
		return TransactionPage{}, ErrInvalidTransactionQuery
	}
	if request.Category != nil {
		if !isTransactionCategoryCode(*request.Category) {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.Category = request.Category
	}
	if request.Direction != nil {
		if *request.Direction != "debit" && *request.Direction != "credit" {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.Direction = request.Direction
	}
	if request.Status != nil {
		switch *request.Status {
		case "pending", "posted", "reversed":
			query.Status = request.Status
		default:
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
	}
	if request.Sort != nil {
		switch *request.Sort {
		case string(transactionSortNewest):
			query.Sort = transactionSortNewest
		case string(transactionSortOldest):
			query.Sort = transactionSortOldest
		default:
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
	}
	if request.Limit != nil {
		limit, parseErr := strconv.Atoi(*request.Limit)
		if parseErr != nil || limit < 1 || limit > maxTransactionPageLimit {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.Limit = limit
	}
	if request.Cursor != nil {
		publicID, decodeErr := decodeTransactionCursor(*request.Cursor)
		if decodeErr != nil {
			return TransactionPage{}, ErrInvalidTransactionQuery
		}
		query.CursorPublicID = &publicID
	}

	result, err := s.repository.Transactions(ctx, userID, query)
	if err != nil {
		return TransactionPage{}, err
	}
	if result.Transactions == nil {
		result.Transactions = make([]Transaction, 0)
	}
	page := TransactionPage{Transactions: result.Transactions}
	if result.HasMore && len(result.Transactions) > 0 {
		cursor := encodeTransactionCursor(result.Transactions[len(result.Transactions)-1].ID)
		page.NextCursor = &cursor
	}
	return page, nil
}

func parseTransactionDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, ErrInvalidTransactionQuery
	}
	return parsed, nil
}

func isTransactionCategoryCode(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for index, character := range value {
		isLowerLetter := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if !isLowerLetter && !isDigit && (index == 0 || (character != '_' && character != '-')) {
			return false
		}
	}
	return true
}

// Categories returns the ordered category master used by the transaction filter.
func (s *TransactionService) Categories(ctx context.Context) ([]Category, error) {
	categories, err := s.repository.TransactionCategories(ctx)
	if err != nil {
		return nil, err
	}
	if categories == nil {
		categories = make([]Category, 0)
	}
	return categories, nil
}

func encodeTransactionCursor(publicID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(publicID))
}

func decodeTransactionCursor(cursor string) (string, error) {
	if len(cursor) != base64.RawURLEncoding.EncodedLen(36) {
		return "", ErrInvalidTransactionQuery
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", ErrInvalidTransactionQuery
	}
	publicID := string(decoded)
	if !isCanonicalUUID(publicID) || encodeTransactionCursor(publicID) != cursor {
		return "", ErrInvalidTransactionQuery
	}
	return publicID, nil
}
