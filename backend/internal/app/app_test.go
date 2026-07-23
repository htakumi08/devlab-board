package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthFlow は、ユーザー登録から認証済みAPIへ到達するまでの一連の契約を確認する。
// 認証ガード、session cookie、重複登録防止が連携した状態を守るためにテストする。
func TestAuthFlow(t *testing.T) {
	handler := NewTestHandler()

	// ログイン前に保護されたAPIへアクセスできないことを確認する。
	unauthorized := performRequest(handler, http.MethodGet, "/api/dashboard", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized before login, got %d", unauthorized.Code)
	}

	// 正常な登録でユーザーとsessionが作られることを確認する。
	registerBody := `{"email":"User@example.com","password":"Password1"}`
	register := performRequest(handler, http.MethodPost, "/api/auth/register", registerBody)
	if register.Code != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d body=%s", http.StatusCreated, register.Code, register.Body.String())
	}

	cookie := register.Result().Cookies()[0]
	if cookie.Name != "devlab_session" {
		t.Fatalf("expected session cookie, got %q", cookie.Name)
	}

	// 同じメールアドレスの重複登録を許可しないことを確認する。
	duplicate := performRequest(handler, http.MethodPost, "/api/auth/register", registerBody)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("expected duplicate status %d, got %d", http.StatusConflict, duplicate.Code)
	}

	// 発行されたsession cookieで保護されたAPIへアクセスできることを確認する。
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	dashboardRequest.AddCookie(cookie)
	dashboard := httptest.NewRecorder()
	handler.ServeHTTP(dashboard, dashboardRequest)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("expected dashboard status %d, got %d body=%s", http.StatusOK, dashboard.Code, dashboard.Body.String())
	}
}

// TestRegisterRejectsInvalidPassword は、要件を満たさないパスワードを登録APIが拒否することを確認する。
// 弱いパスワードでユーザーが作成されないよう、入力検証の境界を守るためにテストする。
func TestRegisterRejectsInvalidPassword(t *testing.T) {
	handler := NewTestHandler()

	response := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"user@example.com","password":"password"}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected validation status %d, got %d", http.StatusUnprocessableEntity, response.Code)
	}
}

// TestUserAgentEndpoint は、認証状態に応じたアクセス制御とUser-AgentのJSON契約を確認する。
// User-Agent Labが安全に保護され、ブラウザの送信値を変更せず観察できることを守るためにテストする。
func TestUserAgentEndpoint(t *testing.T) {
	handler := NewTestHandler()

	// sessionなしのリクエストを拒否し、認証必須の契約を守ることを確認する。
	t.Run("requires session", func(t *testing.T) {
		response := performRequest(handler, http.MethodGet, "/api/user-agent", "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized status %d, got %d", http.StatusUnauthorized, response.Code)
		}
	})

	// 成功ケースで使用するsession cookieを取得するため、テスト用ユーザーを登録する。
	register := performRequest(
		handler,
		http.MethodPost,
		"/api/auth/register",
		`{"email":"user-agent@example.com","password":"Password1"}`,
	)
	if register.Code != http.StatusCreated {
		t.Fatalf("register user: expected status %d, got %d body=%s", http.StatusCreated, register.Code, register.Body.String())
	}

	// 受信したUser-Agentをapplication/jsonでそのまま返すことを確認する。
	t.Run("returns request user agent as json", func(t *testing.T) {
		const userAgent = "Mozilla/5.0 DevLabTest/1.0"

		request := httptest.NewRequest(http.MethodGet, "/api/user-agent", nil)
		request.Header.Set("User-Agent", userAgent)
		request.AddCookie(register.Result().Cookies()[0])
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
		}
		if got := response.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json content type, got %q", got)
		}

		var payload struct {
			UserAgent string `json:"userAgent"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload.UserAgent != userAgent {
			t.Fatalf("expected user agent %q, got %q", userAgent, payload.UserAgent)
		}
	})
}

// TestValidateEmailAndPassword は、メールアドレスとパスワードの入力ルールを組み合わせて確認する。
// API処理とは独立して、許可する入力と拒否する入力の境界を明確に保つためにテストする。
func TestValidateEmailAndPassword(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		ok       bool
	}{
		// 両方の要件を満たす入力だけが成功する基準ケース。
		{name: "valid", email: "user@example.com", password: "Password1", ok: true},
		// メール形式が不正なら、パスワードが正常でも拒否するケース。
		{name: "invalid email", email: "not-email", password: "Password1", ok: false},
		// 8文字未満のパスワードを拒否する境界ケース。
		{name: "short password", email: "user@example.com", password: "Pass1", ok: false},
		// 数字を含まないパスワードを拒否するケース。
		{name: "no number", email: "user@example.com", password: "Password", ok: false},
		// 大文字英字を含まないパスワードを拒否するケース。
		{name: "no upper", email: "user@example.com", password: "password1", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateEmail(tt.email) && validatePassword(tt.password)
			if got != tt.ok {
				t.Fatalf("expected %v, got %v", tt.ok, got)
			}
		})
	}
}

// performRequest は、HTTPリクエスト作成とhandler実行を共通化して各テストの意図を読みやすくする。
func performRequest(handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == "" {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader([]byte(body))
	}
	request := httptest.NewRequest(method, path, requestBody)
	if body != "" && json.Valid([]byte(body)) && strings.HasPrefix(body, "{") {
		request.Header.Set("Content-Type", "application/json")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
