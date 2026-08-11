package financesmoke

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRunChecksFinanceMVPJourney は、認証を含むFinance読み取りMVPの共通smoke順序を確認する。
// 必要な理由: localとAWS variantで同じ公開URL・API contractを反復検証できるようにするため。
func TestRunChecksFinanceMVPJourney(t *testing.T) {
	const (
		email              = "smoke@example.invalid"
		password           = "synthetic-secret"
		secondaryEmail     = "smoke-secondary@example.invalid"
		secondaryPassword  = "secondary-synthetic-secret"
		accountID          = "00000000-0000-4000-8000-000000000001"
		secondaryAccountID = "00000000-0000-4000-8000-000000000002"
	)
	var mu sync.Mutex
	seen := map[string]int{}
	mark := func(key string) {
		mu.Lock()
		defer mu.Unlock()
		seen[key]++
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/finance-lab" || r.URL.Path == "/finance-lab/transactions"):
			mark("frontend:" + r.URL.Path)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><title>devlab-board</title>")
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			mark("healthz")
			writeSmokeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
			var input struct{ Email, Password string }
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Errorf("unexpected login request")
			}
			session := ""
			switch {
			case input.Email == email && input.Password == password:
				mark("login-primary")
				session = "primary-session"
			case input.Email == secondaryEmail && input.Password == secondaryPassword:
				mark("login-secondary")
				session = "secondary-session"
			default:
				t.Errorf("unexpected login credentials")
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: session, Path: "/", HttpOnly: true})
			writeSmokeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{"id": "user"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth/logout":
			mark("logout-" + smokeSession(r))
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
			writeSmokeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case strings.HasPrefix(r.URL.Path, "/api/") && smokeSession(r) == "":
			mark("unauthorized:" + r.URL.Path)
			writeSmokeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthorized"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/summary":
			mark("summary")
			writeSmokeJSON(w, http.StatusOK, map[string]any{"summary": map[string]any{"accountCount": 2}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/accounts":
			mark("accounts")
			responseAccountID := accountID
			if smokeSession(r) == "secondary-session" {
				responseAccountID = secondaryAccountID
				mark("secondary-accounts")
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"accounts": []map[string]string{{"id": responseAccountID, "name": "Synthetic Everyday"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/accounts/"+accountID:
			mark("account-detail")
			writeSmokeJSON(w, http.StatusOK, map[string]any{"account": map[string]string{"id": accountID}, "recentTransactions": []any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/accounts/"+secondaryAccountID && smokeSession(r) == "primary-session":
			mark("ownership-denied")
			writeSmokeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "finance_account_not_found"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/categories":
			mark("categories")
			writeSmokeJSON(w, http.StatusOK, map[string]any{"categories": []map[string]string{{"code": "income", "label": "収入"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/finance/transactions":
			handleSmokeTransactions(t, w, r, accountID, mark)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	err := Run(context.Background(), Config{
		FrontendURL:       server.URL,
		APIURL:            server.URL,
		Email:             email,
		Password:          password,
		SecondaryEmail:    secondaryEmail,
		SecondaryPassword: secondaryPassword,
		Timeout:           3 * time.Second,
	})
	if err != nil {
		t.Fatalf("run smoke journey: %v", err)
	}

	for _, key := range []string{
		"frontend:/", "frontend:/finance-lab", "frontend:/finance-lab/transactions",
		"healthz", "unauthorized:/api/finance/summary", "login-primary", "login-secondary", "summary", "accounts",
		"account-detail", "categories", "transactions-default", "transactions-filtered",
		"transactions-oldest", "transactions-page-1", "transactions-page-2", "secondary-accounts",
		"ownership-denied", "logout-primary-session", "logout-secondary-session",
	} {
		if seen[key] == 0 {
			t.Errorf("expected smoke step %q to run", key)
		}
	}
	if seen["unauthorized:/api/finance/summary"] != 2 {
		t.Errorf("expected unauthorized summary before login and after logout, got %d", seen["unauthorized:/api/finance/summary"])
	}
}

// TestRunDoesNotExposeCredentialsOrResponseBodies は、失敗時もpasswordとserver bodyをerrorへ含めないことを確認する。
// 必要な理由: CI logから認証情報や金融レスポンスが漏れることを防ぐため。
func TestRunDoesNotExposeCredentialsOrResponseBodies(t *testing.T) {
	const password = "do-not-print-this-password"
	const secondaryPassword = "do-not-print-secondary-password"
	server := newSmokePayloadServer("secondary-login-error")
	defer server.Close()

	err := Run(context.Background(), Config{
		FrontendURL: server.URL, APIURL: server.URL,
		Email: "smoke@example.invalid", Password: password,
		SecondaryEmail: "secondary@example.invalid", SecondaryPassword: secondaryPassword,
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected smoke failure")
	}
	for index, secret := range []string{password, secondaryPassword, "smoke@example.invalid", "secondary@example.invalid", "sensitive-response-body"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("expected error not to contain sensitive value case %d", index)
		}
	}
}

// TestRunHonorsTimeout は、応答しないendpointを全体timeoutで中断することを確認する。
// 必要な理由: release検証が外部障害で無期限に停止しないようにするため。
func TestRunHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	started := time.Now()
	err := Run(context.Background(), Config{
		FrontendURL: server.URL, APIURL: server.URL,
		Email: "smoke@example.invalid", Password: "synthetic-secret",
		SecondaryEmail: "secondary@example.invalid", SecondaryPassword: "secondary-secret",
		Timeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout failure")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("expected bounded execution, took %s", elapsed)
	}
}

// TestValidateConfigRejectsUnsafeURLsAndMissingSecrets は、smoke入力をHTTP(S)のoriginと必須値へ制限することを確認する。
// 必要な理由: 認証情報をuser-info付きURLや未対応schemeへ誤送信しないため。
func TestValidateConfigRejectsUnsafeURLsAndMissingSecrets(t *testing.T) {
	valid := Config{
		FrontendURL: "http://localhost:30101", APIURL: "https://api.example.invalid",
		Email: "smoke@example.invalid", Password: "synthetic-secret",
		SecondaryEmail: "secondary@example.invalid", SecondaryPassword: "secondary-secret",
		Timeout: time.Second,
	}
	if err := ValidateConfig(valid); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}

	invalid := []Config{
		// file schemeへ認証情報を送らない。
		{FrontendURL: "file:///tmp/app", APIURL: valid.APIURL, Email: valid.Email, Password: valid.Password, Timeout: valid.Timeout},
		// URL user-infoを拒否する。
		{FrontendURL: valid.FrontendURL, APIURL: "https://user:pass@example.invalid", Email: valid.Email, Password: valid.Password, Timeout: valid.Timeout},
		// password省略を拒否する。
		{FrontendURL: valid.FrontendURL, APIURL: valid.APIURL, Email: valid.Email, Timeout: valid.Timeout},
		// secondary credentials省略を拒否する。
		{FrontendURL: valid.FrontendURL, APIURL: valid.APIURL, Email: valid.Email, Password: valid.Password, Timeout: valid.Timeout},
		// timeout省略を拒否する。
		{FrontendURL: valid.FrontendURL, APIURL: valid.APIURL, Email: valid.Email, Password: valid.Password, SecondaryEmail: valid.SecondaryEmail, SecondaryPassword: valid.SecondaryPassword},
		// remote hostへの平文HTTPでcredentialを送らない。
		{FrontendURL: valid.FrontendURL, APIURL: "http://api.example.invalid", Email: valid.Email, Password: valid.Password, SecondaryEmail: valid.SecondaryEmail, SecondaryPassword: valid.SecondaryPassword, Timeout: valid.Timeout},
	}
	for index, config := range invalid {
		if err := ValidateConfig(config); err == nil {
			t.Fatalf("expected invalid config case %d to fail", index)
		}
	}

	for _, apiURL := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080", "https://api.example.invalid"} {
		config := valid
		config.APIURL = apiURL
		if err := ValidateConfig(config); err != nil {
			t.Errorf("expected API URL %q to be allowed: %v", apiURL, err)
		}
	}
}

// TestRunRejectsIncompleteSuccessPayloads は、HTTP 200でもMVP検証に必要なpayloadが欠ければ失敗することを確認する。
// 必要な理由: endpointの存在だけでなくsynthetic fixtureとresponse contractをsmokeで検出するため。
func TestRunRejectsIncompleteSuccessPayloads(t *testing.T) {
	cases := []struct {
		name string
		mode string
	}{
		// health payloadの意味を検証する。
		{name: "unexpected health payload", mode: "health"},
		// summary wrapperの欠落を検証する。
		{name: "missing summary", mode: "summary"},
		// fixture口座がない環境を検証する。
		{name: "missing accounts", mode: "accounts"},
		// account detail wrapperの欠落を検証する。
		{name: "missing account detail", mode: "detail"},
		// category masterが空の環境を検証する。
		{name: "missing categories", mode: "categories"},
		// 既定取引が空の環境を検証する。
		{name: "missing default transactions", mode: "default-transactions"},
		// 既定の新しい順が崩れた環境を検証する。
		{name: "default transactions are not newest first", mode: "default-order"},
		// 組み合わせfilterとfixtureが一致しない環境を検証する。
		{name: "missing filtered transactions", mode: "filtered-transactions"},
		{name: "filtered account does not match", mode: "filtered-account"},
		{name: "filtered category does not match", mode: "filtered-category"},
		{name: "filtered direction does not match", mode: "filtered-direction"},
		{name: "filtered status does not match", mode: "filtered-status"},
		{name: "filtered date is before UTC range", mode: "filtered-date-before"},
		{name: "filtered date is after UTC range", mode: "filtered-date-after"},
		// 古い順queryが空になる契約回帰を検証する。
		{name: "missing oldest transactions", mode: "oldest-transactions"},
		{name: "oldest transactions are not oldest first", mode: "oldest-order"},
		// 1ページ目のnext cursor欠落を検証する。
		{name: "missing next cursor", mode: "page-one"},
		// cursor付き2ページ目が空になる契約回帰を検証する。
		{name: "missing page two", mode: "page-two"},
		{name: "page two repeats page one", mode: "page-duplicate"},
		// primary userからsecondary userのaccountが見える認可回帰を検証する。
		{name: "other user account is exposed", mode: "ownership-exposed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := newSmokePayloadServer(test.mode)
			defer server.Close()
			err := Run(context.Background(), Config{
				FrontendURL: server.URL, APIURL: server.URL,
				Email: "smoke@example.invalid", Password: "synthetic-secret",
				SecondaryEmail: "secondary@example.invalid", SecondaryPassword: "secondary-secret",
				Timeout: time.Second,
			})
			if err == nil {
				t.Fatalf("expected mode %q to fail", test.mode)
			}
		})
	}
}

func newSmokePayloadServer(mode string) *httptest.Server {
	const (
		accountID          = "00000000-0000-4000-8000-000000000001"
		secondaryAccountID = "00000000-0000-4000-8000-000000000002"
	)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/finance-lab" || r.URL.Path == "/finance-lab/transactions"):
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html>")
		case r.URL.Path == "/healthz":
			status := "ok"
			if mode == "health" {
				status = "unexpected"
			}
			writeSmokeJSON(w, http.StatusOK, map[string]string{"status": status})
		case r.URL.Path == "/api/auth/login":
			var input struct{ Email string }
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input.Email == "secondary@example.invalid" && mode == "secondary-login-error" {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "sensitive-response-body do-not-print-secondary-password")
				break
			}
			session := "primary-session"
			if input.Email == "secondary@example.invalid" {
				session = "secondary-session"
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: session, Path: "/"})
			writeSmokeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{"id": "user"}})
		case r.URL.Path == "/api/auth/logout":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
			writeSmokeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case strings.HasPrefix(r.URL.Path, "/api/") && smokeSession(r) == "":
			writeSmokeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthorized"}})
		case r.URL.Path == "/api/finance/summary":
			var summary any = map[string]any{"accountCount": 2}
			if mode == "summary" {
				summary = nil
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"summary": summary})
		case r.URL.Path == "/api/finance/accounts":
			responseAccountID := accountID
			if smokeSession(r) == "secondary-session" {
				responseAccountID = secondaryAccountID
			}
			accounts := []map[string]string{{"id": responseAccountID, "name": "Synthetic Everyday"}}
			if mode == "accounts" {
				accounts = []map[string]string{}
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
		case r.URL.Path == "/api/finance/accounts/"+accountID:
			var account any = map[string]string{"id": accountID}
			if mode == "detail" {
				account = nil
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"account": account, "recentTransactions": []any{}})
		case r.URL.Path == "/api/finance/accounts/"+secondaryAccountID && smokeSession(r) == "primary-session":
			if mode == "ownership-exposed" {
				writeSmokeJSON(w, http.StatusOK, map[string]any{"account": map[string]string{"id": secondaryAccountID}})
				break
			}
			writeSmokeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "finance_account_not_found"}})
		case r.URL.Path == "/api/finance/categories":
			categories := []map[string]string{{"code": "income"}}
			if mode == "categories" {
				categories = []map[string]string{}
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"categories": categories})
		case r.URL.Path == "/api/finance/transactions":
			transactions := []map[string]any{
				smokeTransactionPayload("newest-1", accountID, "2026-08-10T12:00:00Z"),
				smokeTransactionPayload("newest-2", accountID, "2026-08-09T12:00:00Z"),
			}
			var nextCursor any
			query := r.URL.Query()
			switch {
			case len(query) == 0 && mode == "default-transactions":
				transactions = []map[string]any{}
			case len(query) == 0 && mode == "default-order":
				transactions[0], transactions[1] = transactions[1], transactions[0]
			case query.Get("account_id") != "" && mode == "filtered-transactions":
				transactions = []map[string]any{}
			case query.Get("account_id") != "":
				filtered := smokeTransactionPayload("filtered", accountID, "2026-08-06T12:00:00Z")
				switch mode {
				case "filtered-account":
					filtered["accountId"] = secondaryAccountID
				case "filtered-category":
					filtered["category"] = map[string]string{"code": "groceries"}
				case "filtered-direction":
					filtered["direction"] = "credit"
				case "filtered-status":
					filtered["status"] = "pending"
				case "filtered-date-before":
					filtered["occurredAt"] = "2026-06-30T23:59:59Z"
				case "filtered-date-after":
					filtered["occurredAt"] = "2026-09-01T00:00:00Z"
				}
				transactions = []map[string]any{filtered}
			case query.Get("sort") == "oldest":
				transactions = []map[string]any{
					smokeTransactionPayload("oldest-1", accountID, "2026-07-01T12:00:00Z"),
					smokeTransactionPayload("oldest-2", accountID, "2026-07-02T12:00:00Z"),
				}
				if mode == "oldest-transactions" {
					transactions = []map[string]any{}
				} else if mode == "oldest-order" {
					transactions[0], transactions[1] = transactions[1], transactions[0]
				}
			case query.Get("limit") == "1" && query.Get("cursor") == "":
				transactions = []map[string]any{smokeTransactionPayload("page-1", accountID, "2026-08-10T12:00:00Z")}
				if mode != "page-one" {
					nextCursor = "opaque-next"
				}
			case query.Get("limit") == "1" && query.Get("cursor") != "":
				transactions = []map[string]any{smokeTransactionPayload("page-2", accountID, "2026-08-09T12:00:00Z")}
				if mode == "page-two" {
					transactions = []map[string]any{}
				} else if mode == "page-duplicate" {
					transactions[0]["id"] = "page-1"
				}
			}
			writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": transactions, "nextCursor": nextCursor})
		default:
			http.NotFound(w, r)
		}
	}))
}

func handleSmokeTransactions(t *testing.T, w http.ResponseWriter, r *http.Request, accountID string, mark func(string)) {
	t.Helper()
	query := r.URL.Query()
	switch {
	case len(query) == 0:
		mark("transactions-default")
		writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": []map[string]any{
			smokeTransactionPayload("default-1", accountID, "2026-08-10T12:00:00Z"),
			smokeTransactionPayload("default-2", accountID, "2026-08-09T12:00:00Z"),
		}, "nextCursor": nil})
	case query.Get("account_id") == accountID:
		want := url.Values{
			"account_id": {accountID}, "date_from": {"2026-07-01"}, "date_to": {"2026-08-31"},
			"category": {"income"}, "direction": {"debit"}, "status": {"posted"}, "sort": {"newest"},
		}
		if query.Encode() != want.Encode() {
			t.Errorf("unexpected filter query: %s", query.Encode())
		}
		mark("transactions-filtered")
		writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": []map[string]any{
			smokeTransactionPayload("filtered", accountID, "2026-08-06T12:00:00Z"),
		}, "nextCursor": nil})
	case query.Get("sort") == "oldest":
		mark("transactions-oldest")
		writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": []map[string]any{
			smokeTransactionPayload("oldest-1", accountID, "2026-07-01T12:00:00Z"),
			smokeTransactionPayload("oldest-2", accountID, "2026-07-02T12:00:00Z"),
		}, "nextCursor": nil})
	case query.Get("limit") == "1" && query.Get("cursor") == "":
		mark("transactions-page-1")
		writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": []map[string]any{smokeTransactionPayload("page-1", accountID, "2026-08-10T12:00:00Z")}, "nextCursor": "opaque-next"})
	case query.Get("limit") == "1" && query.Get("cursor") == "opaque-next":
		mark("transactions-page-2")
		writeSmokeJSON(w, http.StatusOK, map[string]any{"transactions": []map[string]any{smokeTransactionPayload("page-2", accountID, "2026-08-09T12:00:00Z")}, "nextCursor": nil})
	default:
		t.Errorf("unexpected transaction query: %s", query.Encode())
		http.Error(w, "unexpected", http.StatusBadRequest)
	}
}

func smokeSession(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func smokeTransactionPayload(id, accountID, occurredAt string) map[string]any {
	return map[string]any{
		"id": id, "accountId": accountID, "occurredAt": occurredAt,
		"direction": "debit", "status": "posted",
		"category": map[string]string{"code": "income"},
	}
}

func writeSmokeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
