package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"devlab-board/backend/internal/finance"
	"github.com/alexedwards/scs/v2"
)

type stubFinanceSummaryReader struct {
	summary    finance.Summary
	err        error
	seenUserID int64
}

func (s *stubFinanceSummaryReader) Summary(_ context.Context, userID int64) (finance.Summary, error) {
	s.seenUserID = userID
	return s.summary, s.err
}

// TestFinanceSummaryEndpoint は、認証sessionのuser IDだけでFinance summaryを取得する契約を確認する。
// 必要な理由: Finance APIが未認証アクセスやclient指定の所有者IDを受け付けない境界を守るため。
func TestFinanceSummaryEndpoint(t *testing.T) {
	asOf := time.Date(2026, time.August, 11, 1, 5, 0, 0, time.UTC)
	reader := &stubFinanceSummaryReader{summary: finance.Summary{
		AccountCount: 1,
		Balances: []finance.Balance{{
			Currency:             "JPY",
			CurrentAmountMinor:   "180000",
			AvailableAmountMinor: minorAmountPointer("165000"),
		}},
		RecentTransactions: []finance.Transaction{{
			ID:          "024c4ea1-e906-4f2a-a32b-f70f95762f76",
			AccountID:   "370fdd2d-aeb9-492d-a981-c40923531411",
			AccountName: "Synthetic Checking",
			Name:        "Grocery Store",
			AmountMinor: "4200",
			Currency:    "JPY",
			Direction:   "debit",
			Status:      "posted",
			OccurredAt:  asOf,
		}},
		AsOf: &asOf,
	}}
	handler := newFinanceTestHandler(reader)

	// sessionがなければrepositoryを呼ばず401を返すことを確認する。
	unauthorized := performRequest(handler, http.MethodGet, "/api/finance/summary", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status %d, got %d", http.StatusUnauthorized, unauthorized.Code)
	}
	if reader.seenUserID != 0 {
		t.Fatalf("expected repository not to be called, got user ID %d", reader.seenUserID)
	}

	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"summary@example.com","password":"Password1"}`)
	if register.Code != http.StatusCreated {
		t.Fatalf("register user: status=%d body=%s", register.Code, register.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/finance/summary", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	if reader.seenUserID != 1 {
		t.Fatalf("expected session user ID 1, got %d", reader.seenUserID)
	}

	var payload struct {
		Summary finance.Summary `json:"summary"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Summary.AccountCount != 1 || len(payload.Summary.RecentTransactions) != 1 {
		t.Fatalf("unexpected summary response: %#v", payload.Summary)
	}
}

// TestFinanceSummaryEndpointSerializesMinorAmountsAsDecimalStrings は、安全整数上限を超える金額も10進文字列で返すことを確認する。
// 必要な理由: PostgreSQL BIGINTをJSON numberで返すと、JavaScriptで丸められて金融金額が変わるため。
func TestFinanceSummaryEndpointSerializesMinorAmountsAsDecimalStrings(t *testing.T) {
	const amountBeyondJavaScriptSafeInteger finance.MinorAmount = "9007199254740993"
	reader := &stubFinanceSummaryReader{summary: finance.Summary{
		Balances: []finance.Balance{{
			Currency:             "JPY",
			CurrentAmountMinor:   amountBeyondJavaScriptSafeInteger,
			AvailableAmountMinor: minorAmountPointer(amountBeyondJavaScriptSafeInteger),
		}},
		RecentTransactions: []finance.Transaction{{
			ID:          "024c4ea1-e906-4f2a-a32b-f70f95762f76",
			AccountID:   "370fdd2d-aeb9-492d-a981-c40923531411",
			AccountName: "Synthetic Checking",
			Name:        "Large transaction",
			AmountMinor: amountBeyondJavaScriptSafeInteger,
			Currency:    "JPY",
			Direction:   "debit",
			Status:      "posted",
			OccurredAt:  time.Date(2026, time.August, 11, 1, 5, 0, 0, time.UTC),
		}},
	}}
	handler := newFinanceTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"summary-large@example.com","password":"Password1"}`)
	if register.Code != http.StatusCreated {
		t.Fatalf("register user: status=%d body=%s", register.Code, register.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/finance/summary", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}

	for _, expected := range []string{
		`"currentAmountMinor":"9007199254740993"`,
		`"availableAmountMinor":"9007199254740993"`,
		`"amountMinor":"9007199254740993"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("expected exact decimal string %s, got %s", expected, response.Body.String())
		}
	}
}

// TestFinanceSummaryEndpointFailure は、DB取得失敗を機密情報のない共通errorへ変換することを確認する。
// 必要な理由: SQLや金融データをresponseへ漏らさず、UIがretry可能なerrorを識別できるようにするため。
func TestFinanceSummaryEndpointFailure(t *testing.T) {
	reader := &stubFinanceSummaryReader{err: errors.New("database details must stay internal")}
	handler := newFinanceTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"summary-error@example.com","password":"Password1"}`)

	request := httptest.NewRequest(http.MethodGet, "/api/finance/summary", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
	if response.Body.String() != "{\"error\":{\"code\":\"finance_summary_unavailable\",\"message\":\"Finance概要を取得できませんでした\"}}\n" {
		t.Fatalf("unexpected safe error response: %s", response.Body.String())
	}
}

func newFinanceTestHandler(reader FinanceSummaryReader) http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(
		Config{AppEnv: "test"},
		NewMemoryUserStore(),
		sessionManager,
		reader,
		emptyFinanceAccountReader{},
		emptyFinanceTransactionReader{},
	).Routes()
}

func minorAmountPointer(value finance.MinorAmount) *finance.MinorAmount {
	return &value
}
