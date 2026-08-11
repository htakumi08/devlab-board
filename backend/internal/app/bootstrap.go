package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	_ "github.com/lib/pq"

	"devlab-board/backend/internal/finance"
	platformpostgres "devlab-board/backend/internal/platform/postgres"
)

// このファイルは、DB、migration、session store、UserStore、HTTP handler を初期化して Server に組み立てる。
// main.go を起動責務に保ち、外部リソースの初期化と依存関係の結線を一か所に集約するために分離している。

type Server struct {
	Handler http.Handler
	DB      *sql.DB
	Redis   *redis.Pool
}

func NewServer(ctx context.Context, config Config) (*Server, error) {
	if config.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := platformpostgres.Migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	sessionManager, redisPool, err := newSessionManager(config, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	financeRepository := finance.NewPostgresRepository(db)
	financeSummary := finance.NewService(financeRepository)
	financeAccounts := finance.NewAccountService(financeRepository)
	handler := New(config, NewPostgresUserStore(db), sessionManager, financeSummary, financeAccounts).Routes()
	return &Server{Handler: handler, DB: db, Redis: redisPool}, nil
}

func (s *Server) Close() error {
	if s.Redis != nil {
		s.Redis.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func newSessionManager(config Config, db *sql.DB) (*scs.SessionManager, *redis.Pool, error) {
	sessionManager := scs.New()
	sessionManager.Cookie.Name = "devlab_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = config.SessionCookieSecure

	switch config.SessionStore {
	case "", "postgres":
		sessionManager.Store = postgresstore.New(db)
		return sessionManager, nil, nil
	case "redis":
		pool := &redis.Pool{
			MaxIdle:     10,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				conn, err := redis.Dial("tcp", config.RedisAddr)
				if err != nil {
					return nil, err
				}
				if config.RedisPassword != "" {
					if _, err := conn.Do("AUTH", config.RedisPassword); err != nil {
						conn.Close()
						return nil, err
					}
				}
				if config.RedisDB != 0 {
					if _, err := conn.Do("SELECT", strconv.Itoa(config.RedisDB)); err != nil {
						conn.Close()
						return nil, err
					}
				}
				return conn, nil
			},
		}
		sessionManager.Store = redisstore.New(pool)
		return sessionManager, pool, nil
	default:
		return nil, nil, fmt.Errorf("unsupported SESSION_STORE %q", config.SessionStore)
	}
}
