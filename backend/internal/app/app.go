package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"

	"devlab-board/backend/internal/finance"
	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

// このファイルは、HTTP route、session 認証、各 handler、JSON response の境界をまとめる。
// リクエスト処理の入口と出口を一か所で追えるようにし、起動処理やデータ保存処理から分離している。

const sessionUserIDKey = "user_id"

type App struct {
	config              Config
	users               UserStore
	sessions            *scs.SessionManager
	financeSummary      FinanceSummaryReader
	financeAccounts     FinanceAccountReader
	financeTransactions FinanceTransactionReader
}

// FinanceSummaryReader is the use-case boundary consumed by the HTTP adapter.
type FinanceSummaryReader interface {
	Summary(ctx context.Context, userID int64) (finance.Summary, error)
}

// FinanceAccountReader is the account-specific use-case boundary consumed by the HTTP adapter.
type FinanceAccountReader interface {
	Accounts(ctx context.Context, userID int64) ([]finance.Account, error)
	Account(ctx context.Context, userID int64, publicAccountID string) (finance.AccountDetail, error)
}

// FinanceTransactionReader is the transaction-list use-case boundary consumed by the HTTP adapter.
type FinanceTransactionReader interface {
	Transactions(ctx context.Context, userID int64, request finance.TransactionListRequest) (finance.TransactionPage, error)
	Categories(ctx context.Context) ([]finance.Category, error)
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Env     string `json:"env"`
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type authResponse struct {
	User userResponse `json:"user"`
}

type userAgentResponse struct {
	UserAgent string `json:"userAgent"`
}

func New(
	config Config,
	users UserStore,
	sessions *scs.SessionManager,
	financeSummary FinanceSummaryReader,
	financeAccounts FinanceAccountReader,
	financeTransactions FinanceTransactionReader,
) *App {
	return &App{
		config:              config,
		users:               users,
		sessions:            sessions,
		financeSummary:      financeSummary,
		financeAccounts:     financeAccounts,
		financeTransactions: financeTransactions,
	}
}

func NewTestHandler() http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(
		Config{AppEnv: "test"},
		NewMemoryUserStore(),
		sessionManager,
		emptyFinanceSummaryReader{},
		emptyFinanceAccountReader{},
		emptyFinanceTransactionReader{},
	).Routes()
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleRoot)
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("POST /api/auth/register", a.handleRegister)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.Handle("POST /api/auth/logout", a.requireSession(http.HandlerFunc(a.handleLogout)))
	mux.Handle("GET /api/auth/me", a.requireSession(http.HandlerFunc(a.handleMe)))
	mux.Handle("GET /api/dashboard", a.requireSession(http.HandlerFunc(a.handleDashboard)))
	mux.Handle("GET /api/user-agent", a.requireSession(http.HandlerFunc(a.handleUserAgent)))
	mux.Handle("GET /api/finance/summary", a.requireSession(http.HandlerFunc(a.handleFinanceSummary)))
	mux.Handle("GET /api/finance/accounts", a.requireSession(http.HandlerFunc(a.handleFinanceAccounts)))
	mux.Handle("GET /api/finance/accounts/{accountId}", a.requireSession(http.HandlerFunc(a.handleFinanceAccount)))
	mux.Handle("GET /api/finance/transactions", a.requireSession(http.HandlerFunc(a.handleFinanceTransactions)))
	mux.Handle("GET /api/finance/categories", a.requireSession(http.HandlerFunc(a.handleFinanceCategories)))

	return a.cors(a.sessions.LoadAndSave(mux))
}

func (a *App) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "devlab-board-backend",
		"message": "backend is running",
	})
}

func (a *App) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "devlab-board-backend",
		Env:     a.config.AppEnv,
	})
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}

	email := normalizeEmail(input.Email)
	validationErrors := validateAuthInput(email, input.Password)
	if len(validationErrors) > 0 {
		writeValidationError(w, validationErrors)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "ユーザー登録に失敗しました")
		return
	}

	user, err := a.users.CreateUser(r.Context(), email, string(passwordHash), defaultNameFromEmail(email))
	if errors.Is(err, ErrEmailExists) {
		writeError(w, http.StatusConflict, "email_exists", "このメールアドレスは既に登録されています")
		return
	}
	if err != nil {
		log.Printf("create user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "ユーザー登録に失敗しました")
		return
	}

	if err := a.startSession(r, user.ID); err != nil {
		log.Printf("start session after register: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "セッション作成に失敗しました")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{User: toUserResponse(user)})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}

	email := normalizeEmail(input.Email)
	if !validateEmail(email) || input.Password == "" {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "メールアドレスまたはパスワードが正しくありません")
		return
	}

	user, err := a.users.FindByEmail(r.Context(), email)
	if errors.Is(err, ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "メールアドレスまたはパスワードが正しくありません")
		return
	}
	if err != nil {
		log.Printf("find user by email: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "ログインに失敗しました")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "メールアドレスまたはパスワードが正しくありません")
		return
	}

	if err := a.startSession(r, user.ID); err != nil {
		log.Printf("start session after login: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "セッション作成に失敗しました")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{User: toUserResponse(user)})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := a.sessions.Destroy(r.Context()); err != nil {
		log.Printf("destroy session: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "ログアウトに失敗しました")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: toUserResponse(user)})
}

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "authenticated api is available",
		"user":    toUserResponse(user),
		"cards": []map[string]string{
			{"label": "Session", "value": "active"},
			{"label": "Store", "value": a.config.SessionStore},
			{"label": "Role", "value": user.Role},
		},
	})
}

func (a *App) handleUserAgent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userAgentResponse{UserAgent: r.UserAgent()})
}

func (a *App) handleFinanceSummary(w http.ResponseWriter, r *http.Request) {
	userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
	summary, err := a.financeSummary.Summary(r.Context(), userID)
	if err != nil {
		log.Printf("finance summary unavailable")
		writeError(w, http.StatusInternalServerError, "finance_summary_unavailable", "Finance概要を取得できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, map[string]finance.Summary{"summary": summary})
}

func (a *App) handleFinanceAccounts(w http.ResponseWriter, r *http.Request) {
	userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
	accounts, err := a.financeAccounts.Accounts(r.Context(), userID)
	if err != nil {
		log.Printf("finance accounts unavailable")
		writeError(w, http.StatusInternalServerError, "finance_accounts_unavailable", "Finance口座一覧を取得できませんでした")
		return
	}
	if accounts == nil {
		accounts = make([]finance.Account, 0)
	}
	writeJSON(w, http.StatusOK, map[string][]finance.Account{"accounts": accounts})
}

func (a *App) handleFinanceAccount(w http.ResponseWriter, r *http.Request) {
	userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
	detail, err := a.financeAccounts.Account(r.Context(), userID, r.PathValue("accountId"))
	if errors.Is(err, finance.ErrAccountNotFound) {
		writeError(w, http.StatusNotFound, "finance_account_not_found", "口座が見つかりません")
		return
	}
	if err != nil {
		log.Printf("finance account unavailable")
		writeError(w, http.StatusInternalServerError, "finance_account_unavailable", "Finance口座を取得できませんでした")
		return
	}
	if detail.RecentTransactions == nil {
		detail.RecentTransactions = make([]finance.Transaction, 0)
	}
	writeJSON(w, http.StatusOK, struct {
		Account            finance.Account       `json:"account"`
		RecentTransactions []finance.Transaction `json:"recentTransactions"`
	}{Account: detail.Account, RecentTransactions: detail.RecentTransactions})
}

func (a *App) handleFinanceTransactions(w http.ResponseWriter, r *http.Request) {
	request, ok := parseFinanceTransactionListRequest(r.URL.RawQuery)
	if !ok {
		writeError(w, http.StatusBadRequest, "finance_transactions_invalid_query", "取引一覧の指定が正しくありません")
		return
	}

	userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
	page, err := a.financeTransactions.Transactions(r.Context(), userID, request)
	if errors.Is(err, finance.ErrInvalidTransactionQuery) {
		writeError(w, http.StatusBadRequest, "finance_transactions_invalid_query", "取引一覧の指定が正しくありません")
		return
	}
	if err != nil {
		log.Printf("finance transactions unavailable")
		writeError(w, http.StatusInternalServerError, "finance_transactions_unavailable", "Finance取引一覧を取得できませんでした")
		return
	}
	if page.Transactions == nil {
		page.Transactions = make([]finance.Transaction, 0)
	}
	writeJSON(w, http.StatusOK, page)
}

func parseFinanceTransactionListRequest(rawQuery string) (finance.TransactionListRequest, bool) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return finance.TransactionListRequest{}, false
	}
	for key, entries := range values {
		if !isFinanceTransactionQueryKey(key) || len(entries) != 1 || entries[0] == "" {
			return finance.TransactionListRequest{}, false
		}
	}

	return finance.TransactionListRequest{
		AccountID: queryValue(values, "account_id"),
		DateFrom:  queryValue(values, "date_from"),
		DateTo:    queryValue(values, "date_to"),
		Category:  queryValue(values, "category"),
		Direction: queryValue(values, "direction"),
		Status:    queryValue(values, "status"),
		Sort:      queryValue(values, "sort"),
		Cursor:    queryValue(values, "cursor"),
		Limit:     queryValue(values, "limit"),
	}, true
}

func isFinanceTransactionQueryKey(key string) bool {
	switch key {
	case "account_id", "date_from", "date_to", "category", "direction", "status", "sort", "cursor", "limit":
		return true
	default:
		return false
	}
}

func queryValue(values url.Values, key string) *string {
	entries, ok := values[key]
	if !ok {
		return nil
	}
	value := entries[0]
	return &value
}

func (a *App) handleFinanceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.financeTransactions.Categories(r.Context())
	if err != nil {
		log.Printf("finance categories unavailable")
		writeError(w, http.StatusInternalServerError, "finance_categories_unavailable", "Financeカテゴリーを取得できませんでした")
		return
	}
	if categories == nil {
		categories = make([]finance.Category, 0)
	}
	writeJSON(w, http.StatusOK, map[string][]finance.Category{"categories": categories})
}

type emptyFinanceSummaryReader struct{}

func (emptyFinanceSummaryReader) Summary(_ context.Context, _ int64) (finance.Summary, error) {
	return finance.Summary{
		Balances:           make([]finance.Balance, 0),
		RecentTransactions: make([]finance.Transaction, 0),
	}, nil
}

type emptyFinanceAccountReader struct{}

func (emptyFinanceAccountReader) Accounts(_ context.Context, _ int64) ([]finance.Account, error) {
	return make([]finance.Account, 0), nil
}

func (emptyFinanceAccountReader) Account(_ context.Context, _ int64, _ string) (finance.AccountDetail, error) {
	return finance.AccountDetail{}, finance.ErrAccountNotFound
}

type emptyFinanceTransactionReader struct{}

func (emptyFinanceTransactionReader) Transactions(_ context.Context, _ int64, _ finance.TransactionListRequest) (finance.TransactionPage, error) {
	return finance.TransactionPage{Transactions: make([]finance.Transaction, 0)}, nil
}

func (emptyFinanceTransactionReader) Categories(_ context.Context) ([]finance.Category, error) {
	return make([]finance.Category, 0), nil
}

func (a *App) startSession(r *http.Request, userID int64) error {
	if err := a.sessions.RenewToken(r.Context()); err != nil {
		return err
	}
	a.sessions.Put(r.Context(), sessionUserIDKey, userID)
	return nil
}

func (a *App) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
		if userID == 0 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) currentUser(r *http.Request) (User, error) {
	userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
	if userID == 0 {
		return User{}, ErrSessionUnavailable
	}
	return a.users.FindByID(r.Context(), userID)
}

func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.config.FrontendOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", a.config.FrontendOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request) (authRequest, bool) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var input authRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "リクエスト形式が正しくありません")
		return authRequest{}, false
	}
	return input, true
}

func validateAuthInput(email string, password string) []fieldError {
	var errors []fieldError
	if !validateEmail(email) {
		errors = append(errors, fieldError{Field: "email", Message: "メールアドレスの形式で入力してください"})
	}
	if !validatePassword(password) {
		errors = append(errors, fieldError{Field: "password", Message: "パスワードは8文字以上、英字、数字、大文字英字を含めてください"})
	}
	return errors
}

func toUserResponse(user User) userResponse {
	return userResponse{
		ID:    user.PublicID,
		Email: user.Email,
		Name:  user.Name,
		Role:  user.Role,
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}

func writeValidationError(w http.ResponseWriter, fieldErrors []fieldError) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":    "validation_error",
			"message": "入力内容を確認してください",
			"fields":  fieldErrors,
		},
	})
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
