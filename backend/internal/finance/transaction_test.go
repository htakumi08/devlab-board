package finance

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubTransactionRepository struct {
	result      transactionPageResult
	categories  []Category
	err         error
	seenUserID  int64
	seenQuery   transactionPageQuery
	calledCount int
}

func (s *stubTransactionRepository) Transactions(_ context.Context, userID int64, query transactionPageQuery) (transactionPageResult, error) {
	s.calledCount++
	s.seenUserID = userID
	s.seenQuery = query
	return s.result, s.err
}

func (s *stubTransactionRepository) TransactionCategories(_ context.Context) ([]Category, error) {
	return s.categories, s.err
}

// TestTransactionServiceUsesDefaultsAndBuildsNextCursor は、既定25件と最後の公開取引ID由来cursorを返すことを確認する。
// 必要な理由: client未指定時のページサイズを安定させ、内部IDをpagination契約へ漏らさないため。
func TestTransactionServiceUsesDefaultsAndBuildsNextCursor(t *testing.T) {
	repository := &stubTransactionRepository{result: transactionPageResult{
		Transactions: []Transaction{
			{ID: "00000000-0000-4000-8000-000000000001"},
			{ID: "00000000-0000-4000-8000-000000000002"},
		},
		HasMore: true,
	}}

	page, err := NewTransactionService(repository).Transactions(context.Background(), 42, TransactionListRequest{})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if repository.seenUserID != 42 || repository.seenQuery.Limit != 25 || repository.seenQuery.CursorPublicID != nil || repository.seenQuery.Sort != transactionSortNewest {
		t.Fatalf("expected owner 42 and default query, got owner=%d query=%#v", repository.seenUserID, repository.seenQuery)
	}
	if page.NextCursor == nil {
		t.Fatal("expected a next cursor")
	}
	decoded, err := decodeTransactionCursor(*page.NextCursor)
	if err != nil || decoded != "00000000-0000-4000-8000-000000000002" {
		t.Fatalf("expected cursor from last public transaction ID, got id=%q err=%v", decoded, err)
	}
}

// TestTransactionServiceValidatesAndNormalizesFilters は、全filterとoldest sortを検証済み内部queryへ正規化することを確認する。
// 必要な理由: HTTP文字列を直接SQL断片へ渡さず、UTC日付境界と許可済み値だけをrepositoryへ渡すため。
func TestTransactionServiceValidatesAndNormalizesFilters(t *testing.T) {
	repository := &stubTransactionRepository{result: transactionPageResult{Transactions: []Transaction{}}}
	request := TransactionListRequest{
		AccountID: stringPointer("00000000-0000-4000-8000-000000000011"),
		DateFrom:  stringPointer("2026-08-01"),
		DateTo:    stringPointer("2026-08-11"),
		Category:  stringPointer("groceries"),
		Direction: stringPointer("debit"),
		Status:    stringPointer("posted"),
		Sort:      stringPointer("oldest"),
	}

	if _, err := NewTransactionService(repository).Transactions(context.Background(), 42, request); err != nil {
		t.Fatalf("list filtered transactions: %v", err)
	}
	wantFrom := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	wantToExclusive := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	query := repository.seenQuery
	if query.AccountPublicID == nil || *query.AccountPublicID != *request.AccountID ||
		query.DateFrom == nil || !query.DateFrom.Equal(wantFrom) ||
		query.DateToExclusive == nil || !query.DateToExclusive.Equal(wantToExclusive) ||
		query.Category == nil || *query.Category != "groceries" ||
		query.Direction == nil || *query.Direction != "debit" ||
		query.Status == nil || *query.Status != "posted" || query.Sort != transactionSortOldest {
		t.Fatalf("unexpected normalized query: %#v", query)
	}
}

// TestTransactionServiceAcceptsUncategorized は、予約値uncategorizedを有効なカテゴリーfilterとして渡すことを確認する。
// 必要な理由: category_codeがNULLの取引もカテゴリーfilterから明示的に取得できるようにするため。
func TestTransactionServiceAcceptsUncategorized(t *testing.T) {
	repository := &stubTransactionRepository{result: transactionPageResult{Transactions: []Transaction{}}}
	if _, err := NewTransactionService(repository).Transactions(context.Background(), 42, TransactionListRequest{Category: stringPointer("uncategorized")}); err != nil {
		t.Fatalf("list uncategorized transactions: %v", err)
	}
	if repository.seenQuery.Category == nil || *repository.seenQuery.Category != "uncategorized" {
		t.Fatalf("expected uncategorized query, got %#v", repository.seenQuery)
	}
}

// TestTransactionServiceRejectsInvalidFilters は、不正filterをrepository呼び出し前に同じ公開errorへ正規化することを確認する。
// 必要な理由: SQL cast errorや内部enumを公開せず、過大入力と曖昧な期間を一貫して拒否するため。
func TestTransactionServiceRejectsInvalidFilters(t *testing.T) {
	invalidRequests := []TransactionListRequest{
		// account_idはcanonical UUIDだけを受け付ける。
		{AccountID: stringPointer("not-an-account")},
		// 日付はUTC暦日のYYYY-MM-DDだけを受け付ける。
		{DateFrom: stringPointer("2026-8-1")},
		{DateTo: stringPointer("2026-08-32")},
		// 終了日が開始日より前の期間を拒否する。
		{DateFrom: stringPointer("2026-08-12"), DateTo: stringPointer("2026-08-11")},
		// category codeは小文字ASCIIの安定形式だけを受け付ける。
		{Category: stringPointer("Groceries")},
		{Category: stringPointer(strings.Repeat("a", 65))},
		// direction、status、sortは許可済みenum以外を拒否する。
		{Direction: stringPointer("out")},
		{Status: stringPointer("complete")},
		{Sort: stringPointer("descending")},
	}
	for _, request := range invalidRequests {
		repository := &stubTransactionRepository{}
		_, err := NewTransactionService(repository).Transactions(context.Background(), 42, request)
		if !errors.Is(err, ErrInvalidTransactionQuery) {
			t.Fatalf("expected invalid query for %#v, got %v", request, err)
		}
		if repository.calledCount != 0 {
			t.Fatalf("expected invalid filter not to reach repository, got %d calls", repository.calledCount)
		}
	}
}

// TestTransactionServiceReturnsCategories は、masterがnilでもAPI向けに空配列へ正規化することを確認する。
// 必要な理由: category候補0件を取得障害と区別し、response shapeを安定させるため。
func TestTransactionServiceReturnsCategories(t *testing.T) {
	repository := &stubTransactionRepository{}
	categories, err := NewTransactionService(repository).Categories(context.Background())
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	if categories == nil || len(categories) != 0 {
		t.Fatalf("expected non-nil empty categories, got %#v", categories)
	}
}

// TestTransactionServiceReturnsCategoryFailure は、category repositoryの失敗を呼び出し元へ返すことを確認する。
// 必要な理由: 取得障害を空候補へ変換せず、HTTP境界が500として扱えるようにするため。
func TestTransactionServiceReturnsCategoryFailure(t *testing.T) {
	repository := &stubTransactionRepository{err: errors.New("category query unavailable")}
	if _, err := NewTransactionService(repository).Categories(context.Background()); err == nil {
		t.Fatal("expected category repository failure")
	}
}

// TestTransactionServiceAcceptsLimitBoundaries は、1件と100件のlimit境界をrepositoryへ渡すことを確認する。
// 必要な理由: 最小・最大の有効値を誤って拒否する境界回帰を防ぐため。
func TestTransactionServiceAcceptsLimitBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		// 最小値1が有効であることを確認する。
		{name: "minimum", value: "1", want: 1},
		// 最大値100が有効であることを確認する。
		{name: "maximum", value: "100", want: 100},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubTransactionRepository{result: transactionPageResult{Transactions: []Transaction{}}}
			_, err := NewTransactionService(repository).Transactions(context.Background(), 42, TransactionListRequest{Limit: stringPointer(test.value)})
			if err != nil {
				t.Fatalf("expected limit %s to be valid: %v", test.value, err)
			}
			if repository.seenQuery.Limit != test.want {
				t.Fatalf("expected limit %d, got %#v", test.want, repository.seenQuery)
			}
		})
	}
}

// TestTransactionServiceRejectsInvalidQuery は、空・非整数・範囲外limitと不正cursorを同じ公開errorへ正規化することを確認する。
// 必要な理由: cursor解析詳細やresource存在を開示せず、DBへ不正queryを渡さないため。
func TestTransactionServiceRejectsInvalidQuery(t *testing.T) {
	invalidRequests := []TransactionListRequest{
		// 明示的な空limitを拒否する。
		{Limit: stringPointer("")},
		// 10進整数ではないlimitを拒否する。
		{Limit: stringPointer("abc")},
		// 下限未満のlimitを拒否する。
		{Limit: stringPointer("0")},
		// 上限超過のlimitを拒否する。
		{Limit: stringPointer("101")},
		// 明示的な空cursorを拒否する。
		{Cursor: stringPointer("")},
		// base64urlとして不正なcursorを拒否する。
		{Cursor: stringPointer("not-an-opaque-cursor")},
		// base64urlでも公開UUIDを含まないcursorを拒否する。
		{Cursor: stringPointer(encodeTransactionCursor("not-a-public-uuid"))},
		// 想定長でもbase64url alphabet外のcursorを拒否する。
		{Cursor: stringPointer(strings.Repeat("*", base64CursorLength()))},
		// 想定長を超えるcursorをdecode前に拒否する。
		{Cursor: stringPointer("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")},
	}
	for _, request := range invalidRequests {
		// 各不正値がrepository呼び出し前に拒否されることを確認する。
		repository := &stubTransactionRepository{}
		_, err := NewTransactionService(repository).Transactions(context.Background(), 42, request)
		if !errors.Is(err, ErrInvalidTransactionQuery) {
			t.Fatalf("expected invalid query for %#v, got %v", request, err)
		}
		if repository.calledCount != 0 {
			t.Fatalf("expected invalid query not to reach repository, got %d calls", repository.calledCount)
		}
	}
}

// TestTransactionServicePassesDecodedCursorAndNormalizesRepositoryMiss は、cursorを公開UUIDへ戻し所有者付きrepositoryで解決することを確認する。
// 必要な理由: cursorだけで他ユーザー取引の位置や存在を推測できないようにするため。
func TestTransactionServicePassesDecodedCursorAndNormalizesRepositoryMiss(t *testing.T) {
	publicID := "00000000-0000-4000-8000-000000000099"
	cursor := encodeTransactionCursor(publicID)
	repository := &stubTransactionRepository{err: ErrInvalidTransactionQuery}

	_, err := NewTransactionService(repository).Transactions(context.Background(), 42, TransactionListRequest{Cursor: &cursor})
	if !errors.Is(err, ErrInvalidTransactionQuery) {
		t.Fatalf("expected invalid query from missing or unowned cursor, got %v", err)
	}
	if repository.seenQuery.CursorPublicID == nil || *repository.seenQuery.CursorPublicID != publicID || repository.seenUserID != 42 {
		t.Fatalf("expected decoded public cursor and owner, got owner=%d query=%#v", repository.seenUserID, repository.seenQuery)
	}
}

// TestTransactionServiceReturnsNullCursorOnLastPage は、追加行がないページでnextCursorをnullにすることを確認する。
// 必要な理由: clientが最終ページを判定し、無効な次ページ操作を表示しないため。
func TestTransactionServiceReturnsNullCursorOnLastPage(t *testing.T) {
	repository := &stubTransactionRepository{result: transactionPageResult{Transactions: []Transaction{}}}
	page, err := NewTransactionService(repository).Transactions(context.Background(), 42, TransactionListRequest{})
	if err != nil {
		t.Fatalf("list final transaction page: %v", err)
	}
	if page.Transactions == nil || page.NextCursor != nil {
		t.Fatalf("expected empty array and null cursor, got %#v", page)
	}
}

// TestPostgresRepositoryTransactionsQueryFailure は、取引一覧queryの失敗を空配列へ変換せず返すことを確認する。
// 必要な理由: DB障害を正常な取引0件と誤表示せず、HTTP境界で安全な500へ変換するため。
func TestPostgresRepositoryTransactionsQueryFailure(t *testing.T) {
	_, err := NewPostgresRepository(failingTransactionQueryer{}).Transactions(context.Background(), 42, transactionPageQuery{Limit: 25})
	if err == nil {
		t.Fatal("expected the repository to return its query error")
	}
}

// TestPostgresRepositoryTransactionCategoriesQueryFailure は、category master取得失敗を空配列に変換しないことを確認する。
// 必要な理由: DB障害を候補0件と誤認せず、HTTP境界で再試行可能な500へ変換するため。
func TestPostgresRepositoryTransactionCategoriesQueryFailure(t *testing.T) {
	_, err := NewPostgresRepository(failingTransactionQueryer{}).TransactionCategories(context.Background())
	if err == nil {
		t.Fatal("expected the repository to return its category query error")
	}
}

type failingTransactionQueryer struct{}

func (failingTransactionQueryer) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, errors.New("transaction query unavailable")
}

func stringPointer(value string) *string {
	return &value
}

func base64CursorLength() int {
	return len(encodeTransactionCursor("00000000-0000-4000-8000-000000000001"))
}
