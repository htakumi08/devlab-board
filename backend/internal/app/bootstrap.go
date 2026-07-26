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

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	sessionManager, redisPool, err := newSessionManager(config, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	handler := New(config, NewPostgresUserStore(db), sessionManager).Routes()
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

func migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			public_id UUID NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			expiry TIMESTAMPTZ NOT NULL
		);

		CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry);
	`)
	return err
}
