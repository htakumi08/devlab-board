package app

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

// このファイルは、HTTP route、session 認証、各 handler、JSON response の境界をまとめる。
// リクエスト処理の入口と出口を一か所で追えるようにし、起動処理やデータ保存処理から分離している。

const sessionUserIDKey = "user_id"

type App struct {
	config   Config
	users    UserStore
	sessions *scs.SessionManager
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

func New(config Config, users UserStore, sessions *scs.SessionManager) *App {
	return &App{
		config:   config,
		users:    users,
		sessions: sessions,
	}
}

func NewTestHandler() http.Handler {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	return New(Config{AppEnv: "test"}, NewMemoryUserStore(), sessionManager).Routes()
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
