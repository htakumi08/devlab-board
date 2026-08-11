package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"devlab-board/backend/internal/finance"
	"github.com/alexedwards/scs/v2"
)

type stubFinanceTransactionReader struct {
	page        finance.TransactionPage
	categories  []finance.Category
	err         error
	seenUserID  int64
	seenRequest finance.TransactionListRequest
	calledCount int
}

func (s *stubFinanceTransactionReader) Transactions(_ context.Context, userID int64, request finance.TransactionListRequest) (finance.TransactionPage, error) {
	s.calledCount++
	s.seenUserID = userID
	s.seenRequest = request
	return s.page, s.err
}

func (s *stubFinanceTransactionReader) Categories(_ context.Context) ([]finance.Category, error) {
	return s.categories, s.err
}

// TestFinanceTransactionsEndpoint は、session所有者とqueryだけで取引を取得し公開DTOを返すことを確認する。
// 必要な理由: client指定の所有者や内部IDを使わず、大きなminor amountも正確に返すため。
func TestFinanceTransactionsEndpoint(t *testing.T) {
	nextCursor := "opaque-next"
	reader := &stubFinanceTransactionReader{page: finance.TransactionPage{
		Transactions: []finance.Transaction{{
			ID:          "00000000-0000-4000-8000-000000000001",
			AccountID:   "00000000-0000-4000-8000-000000000011",
			AccountName: "Synthetic Checking",
			Name:        "Large transaction",
			AmountMinor: "9007199254740993",
			Currency:    "JPY",
			Direction:   "debit",
			Status:      "pending",
			OccurredAt:  time.Date(2026, time.August, 11, 1, 5, 0, 0, time.UTC),
		}},
		NextCursor: &nextCursor,
	}}
	handler := newFinanceTransactionsTestHandler(reader)

	// sessionなしではreaderを呼ばず401にすることを確認する。
	unauthorized := performRequest(handler, http.MethodGet, "/api/finance/transactions", "")
	if unauthorized.Code != http.StatusUnauthorized || reader.calledCount != 0 {
		t.Fatalf("expected unauthorized without reader call, got status=%d calls=%d", unauthorized.Code, reader.calledCount)
	}

	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"transactions@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/transactions?account_id=00000000-0000-4000-8000-000000000011&date_from=2026-08-01&date_to=2026-08-11&category=groceries&direction=debit&status=posted&sort=oldest&limit=2&cursor=opaque-current", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || reader.seenUserID != 1 {
		t.Fatalf("expected owner-scoped success, got status=%d owner=%d body=%s", response.Code, reader.seenUserID, response.Body.String())
	}
	if reader.seenRequest.Limit == nil || *reader.seenRequest.Limit != "2" || reader.seenRequest.Cursor == nil || *reader.seenRequest.Cursor != "opaque-current" {
		t.Fatalf("expected exact query values, got %#v", reader.seenRequest)
	}
	if reader.seenRequest.AccountID == nil || *reader.seenRequest.AccountID != "00000000-0000-4000-8000-000000000011" ||
		reader.seenRequest.DateFrom == nil || *reader.seenRequest.DateFrom != "2026-08-01" ||
		reader.seenRequest.DateTo == nil || *reader.seenRequest.DateTo != "2026-08-11" ||
		reader.seenRequest.Category == nil || *reader.seenRequest.Category != "groceries" ||
		reader.seenRequest.Direction == nil || *reader.seenRequest.Direction != "debit" ||
		reader.seenRequest.Status == nil || *reader.seenRequest.Status != "posted" ||
		reader.seenRequest.Sort == nil || *reader.seenRequest.Sort != "oldest" {
		t.Fatalf("expected exact filter and sort values, got %#v", reader.seenRequest)
	}
	for _, expected := range []string{
		`"transactions":[`,
		`"amountMinor":"9007199254740993"`,
		`"merchant":null`,
		`"category":null`,
		`"nextCursor":"opaque-next"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, response.Body.String())
		}
	}
}

// TestFinanceTransactionsEndpointRejectsUnknownDuplicateAndEmptyQuery は、曖昧なqueryをreaderより前で400にすることを確認する。
// 必要な理由: Values.Getによる先頭値の暗黙採用や未知parameterの無視で、URLと実際の絞り込みがずれるのを防ぐため。
func TestFinanceTransactionsEndpointRejectsUnknownDuplicateAndEmptyQuery(t *testing.T) {
	cases := []string{
		// 未知parameterを黙って無視しない。
		"unknown=value",
		// 同じparameterの複数値を先頭だけ採用しない。
		"status=posted&status=pending",
		// 口座filterの空文字を許可しない。
		"account_id=",
		// 期間開始日の空文字を許可しない。
		"date_from=",
		// 期間終了日の空文字を許可しない。
		"date_to=",
		// category filterの空文字を許可しない。
		"category=",
		// direction filterの空文字を許可しない。
		"direction=",
		// status filterの空文字を許可しない。
		"status=",
		// sortの空文字を許可しない。
		"sort=",
		// cursorの空文字を許可しない。
		"cursor=",
		// limitの空文字を許可しない。
		"limit=",
	}
	for _, rawQuery := range cases {
		t.Run(rawQuery, func(t *testing.T) {
			reader := &stubFinanceTransactionReader{}
			handler := newFinanceTransactionsTestHandler(reader)
			register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"transactions-strict-`+strings.NewReplacer("=", "-", "&", "-", "@", "-").Replace(rawQuery)+`@example.com","password":"Password1"}`)
			request := httptest.NewRequest(http.MethodGet, "/api/finance/transactions?"+rawQuery, nil)
			request.AddCookie(register.Result().Cookies()[0])
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || reader.calledCount != 0 {
				t.Fatalf("expected strict query 400 without reader call, got status=%d calls=%d body=%s", response.Code, reader.calledCount, response.Body.String())
			}
		})
	}
}

// TestFinanceCategoriesEndpoint は、認証済みユーザーへDB由来のcategory候補を返すことを確認する。
// 必要な理由: frontendがcategory masterを重複管理せず、filter optionをAPIから取得できるようにするため。
func TestFinanceCategoriesEndpoint(t *testing.T) {
	reader := &stubFinanceTransactionReader{categories: []finance.Category{{Code: "income", Label: "収入"}}}
	handler := newFinanceTransactionsTestHandler(reader)

	unauthorized := performRequest(handler, http.MethodGet, "/api/finance/categories", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated categories request to return 401, got %d", unauthorized.Code)
	}
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"categories@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/categories", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"categories\":[{\"code\":\"income\",\"label\":\"収入\"}]}\n" {
		t.Fatalf("expected category options, got status=%d body=%s", response.Code, response.Body.String())
	}
}

// TestFinanceCategoriesEndpointFailure は、DB障害を内部詳細のない固定500へ変換することを確認する。
// 必要な理由: category取得障害を空候補と誤認せず、SQL詳細をclientへ漏らさないため。
func TestFinanceCategoriesEndpointFailure(t *testing.T) {
	reader := &stubFinanceTransactionReader{err: errors.New("database details must stay internal")}
	handler := newFinanceTransactionsTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"categories-error@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/categories", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	want := "{\"error\":{\"code\":\"finance_categories_unavailable\",\"message\":\"Financeカテゴリーを取得できませんでした\"}}\n"
	if response.Code != http.StatusInternalServerError || response.Body.String() != want {
		t.Fatalf("expected fixed categories 500, got status=%d body=%s", response.Code, response.Body.String())
	}
}

// TestFinanceTransactionsEndpointPreservesAbsentQuery は、query未指定を既定値としてreaderへ渡すことを確認する。
// 必要な理由: optional queryを省略した既存Slice 3Aのrequestを引き続き有効にするため。
func TestFinanceTransactionsEndpointPreservesAbsentQuery(t *testing.T) {
	reader := &stubFinanceTransactionReader{page: finance.TransactionPage{Transactions: []finance.Transaction{}}}
	handler := newFinanceTransactionsTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"transactions-query@example.com","password":"Password1"}`)
	cookie := register.Result().Cookies()[0]

	// query未指定ではnilをreaderへ渡すことを確認する。
	request := httptest.NewRequest(http.MethodGet, "/api/finance/transactions", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || reader.seenRequest.Cursor != nil || reader.seenRequest.Limit != nil {
		t.Fatalf("expected absent query values, got status=%d request=%#v", response.Code, reader.seenRequest)
	}
	if response.Body.String() != "{\"transactions\":[],\"nextCursor\":null}\n" {
		t.Fatalf("expected empty array and null cursor, got %s", response.Body.String())
	}
}

// TestFinanceTransactionsEndpointErrors は、不正queryとDB障害を機密情報のない固定errorへ変換することを確認する。
// 必要な理由: cursor解析、所有者、SQLの詳細をresponseへ漏らさずclientが再試行可否を判断するため。
func TestFinanceTransactionsEndpointErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		// 不正・不存在・他所有者cursorは同じ400にする。
		{name: "invalid query", err: finance.ErrInvalidTransactionQuery, status: http.StatusBadRequest, body: "{\"error\":{\"code\":\"finance_transactions_invalid_query\",\"message\":\"取引一覧の指定が正しくありません\"}}\n"},
		// DB障害は内部errorを隠した500にする。
		{name: "database failure", err: errors.New("database details must stay internal"), status: http.StatusInternalServerError, body: "{\"error\":{\"code\":\"finance_transactions_unavailable\",\"message\":\"Finance取引一覧を取得できませんでした\"}}\n"},
	}
	for index, test := range tests {
		reader := &stubFinanceTransactionReader{err: test.err}
		handler := newFinanceTransactionsTestHandler(reader)
		register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"transactions-error-`+string(rune('a'+index))+`@example.com","password":"Password1"}`)
		request := httptest.NewRequest(http.MethodGet, "/api/finance/transactions", nil)
		request.AddCookie(register.Result().Cookies()[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status || response.Body.String() != test.body {
			t.Fatalf("%s: expected status=%d body=%s, got status=%d body=%s", test.name, test.status, test.body, response.Code, response.Body.String())
		}
	}
}

// TestFinanceTransactionsEndpointRejectsMalformedRawQuery は、解析不能なquery pairを400にしてreaderを呼ばないことを確認する。
// 必要な理由: 壊れたcursorが未指定として扱われ、意図せず先頭ページを200で返す回帰を防ぐため。
func TestFinanceTransactionsEndpointRejectsMalformedRawQuery(t *testing.T) {
	cases := []struct {
		name     string
		rawQuery string
	}{
		// 不正なURL escapeを含むcursorを拒否する。
		{name: "invalid URL escape", rawQuery: "cursor=%ZZ"},
		// query値を曖昧にする未escape semicolonを拒否する。
		{name: "unescaped semicolon", rawQuery: "cursor=opaque;limit=25"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			reader := &stubFinanceTransactionReader{}
			handler := newFinanceTransactionsTestHandler(reader)
			register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"transactions-malformed-`+strings.ReplaceAll(test.name, " ", "-")+`@example.com","password":"Password1"}`)
			request := httptest.NewRequest(http.MethodGet, "/api/finance/transactions", nil)
			request.URL.RawQuery = test.rawQuery
			request.AddCookie(register.Result().Cookies()[0])
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			wantBody := "{\"error\":{\"code\":\"finance_transactions_invalid_query\",\"message\":\"取引一覧の指定が正しくありません\"}}\n"
			if response.Code != http.StatusBadRequest || response.Body.String() != wantBody {
				t.Fatalf("expected malformed query 400, got status=%d body=%s", response.Code, response.Body.String())
			}
			if reader.calledCount != 0 {
				t.Fatalf("expected malformed query not to call reader, got %d calls", reader.calledCount)
			}
		})
	}
}

func newFinanceTransactionsTestHandler(reader FinanceTransactionReader) http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(
		Config{AppEnv: "test"},
		NewMemoryUserStore(),
		sessionManager,
		emptyFinanceSummaryReader{},
		emptyFinanceAccountReader{},
		reader,
	).Routes()
}
