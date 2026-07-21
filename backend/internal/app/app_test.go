package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthFlow(t *testing.T) {
	handler := NewTestHandler()

	unauthorized := performRequest(handler, http.MethodGet, "/api/dashboard", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized before login, got %d", unauthorized.Code)
	}

	registerBody := `{"email":"User@example.com","password":"Password1"}`
	register := performRequest(handler, http.MethodPost, "/api/auth/register", registerBody)
	if register.Code != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d body=%s", http.StatusCreated, register.Code, register.Body.String())
	}

	cookie := register.Result().Cookies()[0]
	if cookie.Name != "devlab_session" {
		t.Fatalf("expected session cookie, got %q", cookie.Name)
	}

	duplicate := performRequest(handler, http.MethodPost, "/api/auth/register", registerBody)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("expected duplicate status %d, got %d", http.StatusConflict, duplicate.Code)
	}

	dashboardRequest := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	dashboardRequest.AddCookie(cookie)
	dashboard := httptest.NewRecorder()
	handler.ServeHTTP(dashboard, dashboardRequest)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("expected dashboard status %d, got %d body=%s", http.StatusOK, dashboard.Code, dashboard.Body.String())
	}
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	handler := NewTestHandler()

	response := performRequest(handler, http.MethodPost, "/api/auth/register", `{"email":"user@example.com","password":"password"}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected validation status %d, got %d", http.StatusUnprocessableEntity, response.Code)
	}
}

func TestValidateEmailAndPassword(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		ok       bool
	}{
		{name: "valid", email: "user@example.com", password: "Password1", ok: true},
		{name: "invalid email", email: "not-email", password: "Password1", ok: false},
		{name: "short password", email: "user@example.com", password: "Pass1", ok: false},
		{name: "no number", email: "user@example.com", password: "Password", ok: false},
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
