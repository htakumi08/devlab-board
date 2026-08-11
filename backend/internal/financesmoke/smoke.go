// Package financesmoke verifies the public Finance read-only MVP contract over HTTP.
package financesmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

var ErrInvalidConfig = errors.New("invalid finance smoke configuration")

type Config struct {
	FrontendURL       string
	APIURL            string
	Email             string
	Password          string
	SecondaryEmail    string
	SecondaryPassword string
	Timeout           time.Duration
}

type runner struct {
	client       *http.Client
	frontendBase *url.URL
	apiBase      *url.URL
	email        string
	password     string
}

type accountsResponse struct {
	Accounts []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"accounts"`
}

type categoriesResponse struct {
	Categories []struct {
		Code string `json:"code"`
	} `json:"categories"`
}

type smokeTransaction struct {
	ID         string `json:"id"`
	AccountID  string `json:"accountId"`
	Direction  string `json:"direction"`
	Status     string `json:"status"`
	OccurredAt string `json:"occurredAt"`
	Category   *struct {
		Code string `json:"code"`
	} `json:"category"`
}

type transactionsResponse struct {
	Transactions []smokeTransaction `json:"transactions"`
	NextCursor   *string            `json:"nextCursor"`
}

func ValidateConfig(config Config) error {
	if _, err := parseOrigin(config.FrontendURL); err != nil {
		return err
	}
	if _, err := parseAPIOrigin(config.APIURL); err != nil {
		return err
	}
	if strings.TrimSpace(config.Email) == "" || config.Password == "" ||
		strings.TrimSpace(config.SecondaryEmail) == "" || config.SecondaryPassword == "" {
		return fmt.Errorf("%w: credentials are required", ErrInvalidConfig)
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be positive", ErrInvalidConfig)
	}
	return nil
}

// Run executes the environment-neutral Finance MVP smoke journey without returning response bodies.
func Run(ctx context.Context, config Config) error {
	if err := ValidateConfig(config); err != nil {
		return err
	}
	frontendBase, _ := parseOrigin(config.FrontendURL)
	apiBase, _ := parseAPIOrigin(config.APIURL)
	primary, err := newRunner(frontendBase, apiBase, config.Email, config.Password, config.Timeout)
	if err != nil {
		return err
	}
	secondary, err := newRunner(frontendBase, apiBase, config.SecondaryEmail, config.SecondaryPassword, config.Timeout)
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	return primary.run(runCtx, secondary)
}

func newRunner(frontendBase, apiBase *url.URL, email, password string, timeout time.Duration) (runner, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return runner{}, errors.New("create finance smoke cookie jar")
	}
	return runner{
		client: &http.Client{
			Jar:     jar,
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		frontendBase: frontendBase,
		apiBase:      apiBase,
		email:        strings.TrimSpace(email),
		password:     password,
	}, nil
}

func (r runner) run(ctx context.Context, secondary runner) error {
	for _, step := range []struct {
		name string
		path string
	}{
		{name: "frontend root", path: "/"},
		{name: "finance direct link", path: "/finance-lab"},
		{name: "transactions direct link", path: "/finance-lab/transactions"},
	} {
		if err := r.checkHTML(ctx, step.name, step.path); err != nil {
			return err
		}
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := r.requestJSON(ctx, "health check", http.MethodGet, "/healthz", nil, http.StatusOK, &health); err != nil {
		return err
	}
	if health.Status != "ok" {
		return errors.New("health check returned an unexpected payload")
	}
	if err := r.requestJSON(ctx, "unauthenticated finance check", http.MethodGet, "/api/finance/summary", nil, http.StatusUnauthorized, nil); err != nil {
		return err
	}

	if err := r.login(ctx, "primary login"); err != nil {
		return err
	}

	var summary struct {
		Summary json.RawMessage `json:"summary"`
	}
	if err := r.requestJSON(ctx, "finance summary", http.MethodGet, "/api/finance/summary", nil, http.StatusOK, &summary); err != nil {
		return err
	}
	if len(summary.Summary) == 0 || string(summary.Summary) == "null" {
		return errors.New("finance summary returned an unexpected payload")
	}

	var accounts accountsResponse
	if err := r.requestJSON(ctx, "finance accounts", http.MethodGet, "/api/finance/accounts", nil, http.StatusOK, &accounts); err != nil {
		return err
	}
	accountID := ""
	for _, account := range accounts.Accounts {
		if account.Name == "Synthetic Everyday" {
			accountID = account.ID
			break
		}
	}
	if accountID == "" {
		return errors.New("finance accounts returned no smoke fixture")
	}
	var detail struct {
		Account json.RawMessage `json:"account"`
	}
	if err := r.requestJSON(ctx, "finance account detail", http.MethodGet, "/api/finance/accounts/"+url.PathEscape(accountID), nil, http.StatusOK, &detail); err != nil {
		return err
	}
	if len(detail.Account) == 0 || string(detail.Account) == "null" {
		return errors.New("finance account detail returned an unexpected payload")
	}
	if err := r.verifyOwnershipIsolation(ctx, secondary); err != nil {
		return err
	}

	var categories categoriesResponse
	if err := r.requestJSON(ctx, "finance categories", http.MethodGet, "/api/finance/categories", nil, http.StatusOK, &categories); err != nil {
		return err
	}
	categoryCode := ""
	for _, category := range categories.Categories {
		if category.Code == "income" {
			categoryCode = category.Code
			break
		}
	}
	if categoryCode == "" {
		return errors.New("finance categories returned no options")
	}

	var defaultPage transactionsResponse
	if err := r.requestJSON(ctx, "finance transactions", http.MethodGet, "/api/finance/transactions", nil, http.StatusOK, &defaultPage); err != nil {
		return err
	}
	if len(defaultPage.Transactions) == 0 {
		return errors.New("finance transactions returned no smoke fixture")
	}
	if err := validateTransactionOrder(defaultPage.Transactions, true); err != nil {
		return fmt.Errorf("finance transactions: %w", err)
	}

	filterQuery := url.Values{
		"account_id": {accountID},
		"date_from":  {"2026-07-01"},
		"date_to":    {"2026-08-31"},
		"category":   {categoryCode},
		"direction":  {"debit"},
		"status":     {"posted"},
		"sort":       {"newest"},
	}
	var filteredPage transactionsResponse
	if err := r.requestJSON(ctx, "filtered finance transactions", http.MethodGet, "/api/finance/transactions?"+filterQuery.Encode(), nil, http.StatusOK, &filteredPage); err != nil {
		return err
	}
	if len(filteredPage.Transactions) == 0 {
		return errors.New("filtered finance transactions returned no smoke fixture")
	}
	if err := validateFilteredTransactions(filteredPage.Transactions, accountID); err != nil {
		return err
	}

	var oldestPage transactionsResponse
	if err := r.requestJSON(ctx, "oldest finance transactions", http.MethodGet, "/api/finance/transactions?sort=oldest", nil, http.StatusOK, &oldestPage); err != nil {
		return err
	}
	if len(oldestPage.Transactions) == 0 {
		return errors.New("oldest finance transactions returned no smoke fixture")
	}
	if err := validateTransactionOrder(oldestPage.Transactions, false); err != nil {
		return fmt.Errorf("oldest finance transactions: %w", err)
	}

	var firstPage transactionsResponse
	if err := r.requestJSON(ctx, "finance transaction page one", http.MethodGet, "/api/finance/transactions?limit=1", nil, http.StatusOK, &firstPage); err != nil {
		return err
	}
	if len(firstPage.Transactions) != 1 || firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		return errors.New("finance transaction page one returned an unexpected payload")
	}
	secondQuery := url.Values{"cursor": {*firstPage.NextCursor}, "limit": {"1"}}
	var secondPage transactionsResponse
	if err := r.requestJSON(ctx, "finance transaction page two", http.MethodGet, "/api/finance/transactions?"+secondQuery.Encode(), nil, http.StatusOK, &secondPage); err != nil {
		return err
	}
	if len(secondPage.Transactions) != 1 {
		return errors.New("finance transaction page two returned an unexpected payload")
	}
	if firstPage.Transactions[0].ID == "" || secondPage.Transactions[0].ID == "" ||
		firstPage.Transactions[0].ID == secondPage.Transactions[0].ID {
		return errors.New("finance transaction pages returned overlapping rows")
	}

	if err := r.logout(ctx, "primary logout"); err != nil {
		return err
	}
	if err := r.requestJSON(ctx, "post-logout finance check", http.MethodGet, "/api/finance/summary", nil, http.StatusUnauthorized, nil); err != nil {
		return err
	}
	return nil
}

func (r runner) login(ctx context.Context, step string) error {
	loginBody := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: r.email, Password: r.password}
	var login struct {
		User json.RawMessage `json:"user"`
	}
	if err := r.requestJSON(ctx, step, http.MethodPost, "/api/auth/login", loginBody, http.StatusOK, &login); err != nil {
		return err
	}
	if len(login.User) == 0 || string(login.User) == "null" {
		return fmt.Errorf("%s returned an unexpected payload", step)
	}
	return nil
}

func (r runner) logout(ctx context.Context, step string) error {
	var logout struct {
		Status string `json:"status"`
	}
	if err := r.requestJSON(ctx, step, http.MethodPost, "/api/auth/logout", struct{}{}, http.StatusOK, &logout); err != nil {
		return err
	}
	if logout.Status != "ok" {
		return fmt.Errorf("%s returned an unexpected payload", step)
	}
	return nil
}

func (r runner) verifyOwnershipIsolation(ctx context.Context, secondary runner) error {
	if err := secondary.login(ctx, "secondary login"); err != nil {
		return err
	}
	var accounts accountsResponse
	accountErr := secondary.requestJSON(ctx, "secondary finance accounts", http.MethodGet, "/api/finance/accounts", nil, http.StatusOK, &accounts)
	secondaryAccountID := fixtureAccountID(accounts)
	logoutErr := secondary.logout(ctx, "secondary logout")
	if accountErr != nil {
		return accountErr
	}
	if secondaryAccountID == "" {
		return errors.New("secondary finance accounts returned no smoke fixture")
	}
	if logoutErr != nil {
		return logoutErr
	}
	return r.requestJSON(
		ctx,
		"cross-user finance account check",
		http.MethodGet,
		"/api/finance/accounts/"+url.PathEscape(secondaryAccountID),
		nil,
		http.StatusNotFound,
		nil,
	)
}

func fixtureAccountID(accounts accountsResponse) string {
	for _, account := range accounts.Accounts {
		if account.Name == "Synthetic Everyday" {
			return account.ID
		}
	}
	return ""
}

func validateFilteredTransactions(transactions []smokeTransaction, accountID string) error {
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	toExclusive := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	for _, transaction := range transactions {
		occurredAt, err := time.Parse(time.RFC3339, transaction.OccurredAt)
		if err != nil {
			return errors.New("filtered finance transactions returned an invalid timestamp")
		}
		if transaction.AccountID != accountID || transaction.Category == nil || transaction.Category.Code != "income" ||
			transaction.Direction != "debit" || transaction.Status != "posted" ||
			occurredAt.Before(from) || !occurredAt.Before(toExclusive) {
			return errors.New("filtered finance transactions did not satisfy all criteria")
		}
	}
	return nil
}

func validateTransactionOrder(transactions []smokeTransaction, newest bool) error {
	if len(transactions) < 2 {
		return errors.New("returned too few rows to verify ordering")
	}
	previous, err := time.Parse(time.RFC3339, transactions[0].OccurredAt)
	if err != nil {
		return errors.New("returned an invalid timestamp")
	}
	for _, transaction := range transactions[1:] {
		current, err := time.Parse(time.RFC3339, transaction.OccurredAt)
		if err != nil {
			return errors.New("returned an invalid timestamp")
		}
		if (newest && current.After(previous)) || (!newest && current.Before(previous)) {
			return errors.New("returned rows in an unexpected order")
		}
		previous = current
	}
	return nil
}

func (r runner) checkHTML(ctx context.Context, step, path string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvePath(r.frontendBase, path), nil)
	if err != nil {
		return fmt.Errorf("%s: create request", step)
	}
	request.Header.Set("Accept", "text/html")
	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("%s: request failed", step)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: expected HTTP 200, got %d", step, response.StatusCode)
	}
	if mediaType := response.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(mediaType), "text/html") {
		return fmt.Errorf("%s: expected an HTML response", step)
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes)); err != nil {
		return fmt.Errorf("%s: read response", step)
	}
	return nil
}

func (r runner) requestJSON(ctx context.Context, step, method, path string, body any, expectedStatus int, output any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: encode request", step)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, resolvePath(r.apiBase, path), requestBody)
	if err != nil {
		return fmt.Errorf("%s: create request", step)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("%s: request failed", step)
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		return fmt.Errorf("%s: expected HTTP %d, got %d", step, expectedStatus, response.StatusCode)
	}
	if output == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		if err != nil {
			return fmt.Errorf("%s: read response", step)
		}
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("%s: decode response", step)
	}
	return nil
}

func parseOrigin(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%w: URL must be an HTTP(S) origin", ErrInvalidConfig)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("%w: URL must not contain user-info, path, query, or fragment", ErrInvalidConfig)
	}
	parsed.Path = ""
	return parsed, nil
}

func parseAPIOrigin(raw string) (*url.URL, error) {
	parsed, err := parseOrigin(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "https" {
		return parsed, nil
	}
	hostname := parsed.Hostname()
	ip := net.ParseIP(hostname)
	if !strings.EqualFold(hostname, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("%w: API URL must use HTTPS unless it targets localhost", ErrInvalidConfig)
	}
	return parsed, nil
}

func resolvePath(base *url.URL, path string) string {
	resolved := *base
	resolved.Path = path
	resolved.RawPath = ""
	resolved.RawQuery = ""
	if before, query, ok := strings.Cut(path, "?"); ok {
		resolved.Path = before
		resolved.RawQuery = query
	}
	return resolved.String()
}
