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

type stubFinanceAccountReader struct {
	accounts            []finance.Account
	detail              finance.AccountDetail
	err                 error
	seenListUserID      int64
	seenDetailUserID    int64
	seenPublicAccountID string
}

func (s *stubFinanceAccountReader) Accounts(_ context.Context, userID int64) ([]finance.Account, error) {
	s.seenListUserID = userID
	return s.accounts, s.err
}

func (s *stubFinanceAccountReader) Account(_ context.Context, userID int64, publicAccountID string) (finance.AccountDetail, error) {
	s.seenDetailUserID = userID
	s.seenPublicAccountID = publicAccountID
	return s.detail, s.err
}

// TestFinanceAccountsEndpoints は、一覧と詳細がsession所有者だけを使い公開用DTOを返すことを確認する。
// 必要な理由: client指定の所有者IDや内部IDをFinance口座APIへ混入させないため。
func TestFinanceAccountsEndpoints(t *testing.T) {
	balanceAsOf := time.Date(2026, time.August, 12, 2, 0, 0, 0, time.UTC)
	account := finance.Account{
		ID:                   "00000000-0000-4000-8000-000000000001",
		Name:                 "Synthetic Checking",
		AccountType:          "checking",
		Mask:                 "1234",
		Currency:             "JPY",
		CurrentAmountMinor:   "9007199254740993",
		AvailableAmountMinor: minorAmountPointer("165000"),
		Status:               "active",
		BalanceAsOf:          balanceAsOf,
	}
	reader := &stubFinanceAccountReader{
		accounts: []finance.Account{account},
		detail: finance.AccountDetail{
			Account: account,
			RecentTransactions: []finance.Transaction{{
				ID:          "00000000-0000-4000-8000-000000000011",
				AccountID:   account.ID,
				AccountName: account.Name,
				Name:        "Grocery Store",
				AmountMinor: "4200",
				Currency:    "JPY",
				Direction:   "debit",
				Status:      "posted",
				OccurredAt:  balanceAsOf,
			}},
		},
	}
	handler := newFinanceAccountsTestHandler(reader)

	// sessionなしでは一覧・詳細ともreaderを呼ばず401にする。
	for _, path := range []string{
		"/api/finance/accounts",
		"/api/finance/accounts/00000000-0000-4000-8000-000000000001",
	} {
		response := performRequest(handler, http.MethodGet, path, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized for %s, got %d", path, response.Code)
		}
	}
	if reader.seenListUserID != 0 || reader.seenDetailUserID != 0 {
		t.Fatalf("expected readers not to run before authentication, got list=%d detail=%d", reader.seenListUserID, reader.seenDetailUserID)
	}

	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"accounts@example.com","password":"Password1"}`)
	if register.Code != http.StatusCreated {
		t.Fatalf("register user: status=%d body=%s", register.Code, register.Body.String())
	}
	cookie := register.Result().Cookies()[0]

	// 一覧は空でない配列と、精度を失わない金額文字列を返す。
	listRequest := httptest.NewRequest(http.MethodGet, "/api/finance/accounts", nil)
	listRequest.AddCookie(cookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d body=%s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	if reader.seenListUserID != 1 {
		t.Fatalf("expected list session user ID 1, got %d", reader.seenListUserID)
	}
	for _, expected := range []string{
		`"accounts":[`,
		`"currentAmountMinor":"9007199254740993"`,
		`"availableAmountMinor":"165000"`,
		`"mask":"1234"`,
	} {
		if !strings.Contains(listResponse.Body.String(), expected) {
			t.Fatalf("expected list response to contain %s, got %s", expected, listResponse.Body.String())
		}
	}

	// 詳細はpathの公開UUIDを変更せず渡し、口座とrecentTransactionsを別fieldで返す。
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/finance/accounts/"+account.ID, nil)
	detailRequest.AddCookie(cookie)
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("expected detail status %d, got %d body=%s", http.StatusOK, detailResponse.Code, detailResponse.Body.String())
	}
	if reader.seenDetailUserID != 1 || reader.seenPublicAccountID != account.ID {
		t.Fatalf("expected owner 1 and public account %q, got owner=%d account=%q", account.ID, reader.seenDetailUserID, reader.seenPublicAccountID)
	}
	if !strings.Contains(detailResponse.Body.String(), `"account":{`) ||
		!strings.Contains(detailResponse.Body.String(), `"recentTransactions":[`) {
		t.Fatalf("unexpected detail response: %s", detailResponse.Body.String())
	}
}

// TestFinanceAccountsEndpointEmpty は、口座がないユーザーへnullではなく空配列を返すことを確認する。
// 必要な理由: Frontendが正常なempty stateと契約不正を区別できるようにするため。
func TestFinanceAccountsEndpointEmpty(t *testing.T) {
	handler := newFinanceAccountsTestHandler(&stubFinanceAccountReader{accounts: []finance.Account{}})
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"accounts-empty@example.com","password":"Password1"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/finance/accounts", nil)
	request.AddCookie(register.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"accounts\":[]}\n" {
		t.Fatalf("expected empty accounts array, got status=%d body=%s", response.Code, response.Body.String())
	}
}

// TestFinanceAccountEndpointHidesInvalidAndUnownedIDs は、不正・未存在・他所有者IDを同じ404へ変換することを確認する。
// 必要な理由: ID形式や所有者の違いからresource存在を推測されないようにするため。
func TestFinanceAccountEndpointHidesInvalidAndUnownedIDs(t *testing.T) {
	wantBody := "{\"error\":{\"code\":\"finance_account_not_found\",\"message\":\"口座が見つかりません\"}}\n"
	for _, accountID := range []string{
		"not-a-uuid",
		"00000000-0000-4000-8000-000000000099",
		"00000000-0000-4000-8000-000000000100",
	} {
		reader := &stubFinanceAccountReader{err: finance.ErrAccountNotFound}
		handler := newFinanceAccountsTestHandler(reader)
		register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"account-not-found-`+accountID+`@example.com","password":"Password1"}`)
		request := httptest.NewRequest(http.MethodGet, "/api/finance/accounts/"+accountID, nil)
		request.AddCookie(register.Result().Cookies()[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound || response.Body.String() != wantBody {
			t.Fatalf("expected indistinguishable 404 for %q, got status=%d body=%s", accountID, response.Code, response.Body.String())
		}
	}
}

// TestFinanceAccountsEndpointFailures は、DB障害を一覧・詳細固有の安全な500へ変換することを確認する。
// 必要な理由: 内部errorやFinance識別子をresponseへ漏らさず、Frontendが失敗領域を識別するため。
func TestFinanceAccountsEndpointFailures(t *testing.T) {
	reader := &stubFinanceAccountReader{err: errors.New("database details must stay internal")}
	handler := newFinanceAccountsTestHandler(reader)
	register := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"accounts-error@example.com","password":"Password1"}`)
	cookie := register.Result().Cookies()[0]

	tests := []struct {
		path string
		body string
	}{
		{path: "/api/finance/accounts", body: "{\"error\":{\"code\":\"finance_accounts_unavailable\",\"message\":\"Finance口座一覧を取得できませんでした\"}}\n"},
		{path: "/api/finance/accounts/00000000-0000-4000-8000-000000000001", body: "{\"error\":{\"code\":\"finance_account_unavailable\",\"message\":\"Finance口座を取得できませんでした\"}}\n"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusInternalServerError || response.Body.String() != test.body {
			t.Fatalf("unexpected safe error for %s: status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func newFinanceAccountsTestHandler(reader FinanceAccountReader) http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(Config{AppEnv: "test"}, NewMemoryUserStore(), sessionManager, emptyFinanceSummaryReader{}, reader, emptyFinanceTransactionReader{}).Routes()
}
