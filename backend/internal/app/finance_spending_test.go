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

type stubFinanceSpendingReader struct {
	summary     finance.SpendingSummary
	err         error
	seenUserID  int64
	seenRequest finance.SpendingSummaryRequest
	calledCount int
}

func (s *stubFinanceSpendingReader) SpendingSummary(_ context.Context, userID int64, request finance.SpendingSummaryRequest) (finance.SpendingSummary, error) {
	s.calledCount++
	s.seenUserID = userID
	s.seenRequest = request
	return s.summary, s.err
}

// TestFinanceSpendingSummaryEndpoint は、session所有者と必須期間だけで通貨別支出集計を返すことを確認する。
// 必要な理由: client指定の所有者を受け付けず、巨大なminor unitと未分類カテゴリーを正確に公開するため。
func TestFinanceSpendingSummaryEndpoint(t *testing.T) {
	asOf := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	reader := &stubFinanceSpendingReader{summary: finance.SpendingSummary{
		DateFrom: "2026-08-01", DateTo: "2026-08-31", Timezone: "UTC", AsOf: &asOf,
		Currencies: []finance.SpendingCurrency{{
			Currency: "JPY", TotalAmountMinor: "9007199254740993",
			Categories: []finance.SpendingCategory{{Category: nil, AmountMinor: "9007199254740993", TransactionCount: 2}},
		}},
	}}
	handler := newFinanceSpendingTestHandler(reader)

	// sessionなしではreaderを呼ばず401にすることを確認する。
	unauthorized := performRequest(handler, http.MethodGet, "/api/finance/spending-summary?date_from=2026-08-01&date_to=2026-08-31", "")
	if unauthorized.Code != http.StatusUnauthorized || reader.calledCount != 0 {
		t.Fatalf("expected unauthorized without reader call, got status=%d calls=%d", unauthorized.Code, reader.calledCount)
	}

	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"spending@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/spending-summary?date_from=2026-08-01&date_to=2026-08-31", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || reader.seenUserID != 1 {
		t.Fatalf("expected owner-scoped success, got status=%d owner=%d body=%s", response.Code, reader.seenUserID, response.Body.String())
	}
	if reader.seenRequest.DateFrom != "2026-08-01" || reader.seenRequest.DateTo != "2026-08-31" {
		t.Fatalf("expected exact period, got %#v", reader.seenRequest)
	}
	for _, expected := range []string{
		`"spendingSummary":{`,
		`"timezone":"UTC"`,
		`"totalAmountMinor":"9007199254740993"`,
		`"category":null`,
		`"transactionCount":2`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, response.Body.String())
		}
	}
}

// TestFinanceSpendingSummaryEndpointRejectsInvalidQuery は、欠落・未知・重複・空queryをreaderより前で拒否することを確認する。
// 必要な理由: 暗黙の期間や先頭値採用を避け、表示期間と実際の集計範囲を一致させるため。
func TestFinanceSpendingSummaryEndpointRejectsInvalidQuery(t *testing.T) {
	cases := []string{
		// 両方の期間境界を必須にする。
		"date_from=2026-08-01",
		"date_to=2026-08-31",
		// 未知parameterを黙って無視しない。
		"date_from=2026-08-01&date_to=2026-08-31&unknown=value",
		// 同じparameterの複数値を先頭だけ採用しない。
		"date_from=2026-08-01&date_from=2026-08-02&date_to=2026-08-31",
		// 空文字を既定期間へ暗黙変換しない。
		"date_from=&date_to=2026-08-31",
		"date_from=2026-08-01&date_to=",
	}
	for index, rawQuery := range cases {
		reader := &stubFinanceSpendingReader{}
		handler := newFinanceSpendingTestHandler(reader)
		register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"spending-query-`+string(rune('a'+index))+`@example.com","password":"Password1"}`)
		request := httptest.NewRequest(http.MethodGet, "/api/finance/spending-summary?"+rawQuery, nil)
		request.AddCookie(register.Result().Cookies()[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest || reader.calledCount != 0 {
			t.Fatalf("expected strict query 400 without reader call, got query=%q status=%d calls=%d body=%s", rawQuery, response.Code, reader.calledCount, response.Body.String())
		}
	}
}

// TestFinanceSpendingSummaryEndpointRejectsMalformedRawQuery は、解析不能なqueryを400にしてreaderを呼ばないことを確認する。
// 必要な理由: 壊れた期間値を欠落扱いにせず、strict query契約を維持するため。
func TestFinanceSpendingSummaryEndpointRejectsMalformedRawQuery(t *testing.T) {
	reader := &stubFinanceSpendingReader{}
	handler := newFinanceSpendingTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"spending-malformed@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/spending-summary", nil)
	request.URL.RawQuery = "date_from=%ZZ&date_to=2026-08-31"
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || reader.calledCount != 0 {
		t.Fatalf("expected malformed query 400 without reader call, got status=%d calls=%d body=%s", response.Code, reader.calledCount, response.Body.String())
	}
}

// TestFinanceSpendingSummaryEndpointErrors は、不正期間とDB障害を固定された安全なerrorへ変換することを確認する。
// 必要な理由: validationやSQLの内部詳細をresponseへ漏らさず、clientが修正と再試行を区別できるようにするため。
func TestFinanceSpendingSummaryEndpointErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		// domain validation errorは同じ400契約にする。
		{name: "invalid period", err: finance.ErrInvalidSpendingQuery, status: http.StatusBadRequest, body: "{\"error\":{\"code\":\"finance_spending_summary_invalid_query\",\"message\":\"支出集計の期間指定が正しくありません\"}}\n"},
		// DB障害は内部errorを隠した500にする。
		{name: "database failure", err: errors.New("database details must stay internal"), status: http.StatusInternalServerError, body: "{\"error\":{\"code\":\"finance_spending_summary_unavailable\",\"message\":\"Finance支出集計を取得できませんでした\"}}\n"},
	}
	for index, test := range tests {
		reader := &stubFinanceSpendingReader{err: test.err}
		handler := newFinanceSpendingTestHandler(reader)
		register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"spending-error-`+string(rune('a'+index))+`@example.com","password":"Password1"}`)
		request := httptest.NewRequest(http.MethodGet, "/api/finance/spending-summary?date_from=2026-08-01&date_to=2026-08-31", nil)
		request.AddCookie(register.Result().Cookies()[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status || response.Body.String() != test.body {
			t.Fatalf("%s: expected status=%d body=%s, got status=%d body=%s", test.name, test.status, test.body, response.Code, response.Body.String())
		}
	}
}

func newFinanceSpendingTestHandler(reader FinanceSpendingReader) http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(
		Config{AppEnv: "test"},
		NewMemoryUserStore(),
		sessionManager,
		emptyFinanceSummaryReader{},
		emptyFinanceAccountReader{},
		emptyFinanceTransactionReader{},
		reader,
	).Routes()
}
