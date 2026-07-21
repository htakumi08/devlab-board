package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

var (
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredential  = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrValidation         = errors.New("validation failed")
	ErrSessionUnavailable = errors.New("session unavailable")
)

type User struct {
	ID           int64
	PublicID     string
	Email        string
	PasswordHash string
	Name         string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserStore interface {
	CreateUser(ctx context.Context, email string, passwordHash string, name string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id int64) (User, error)
}

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

func (s *PostgresUserStore) CreateUser(ctx context.Context, email string, passwordHash string, name string) (User, error) {
	publicID, err := newUUID()
	if err != nil {
		return User{}, err
	}

	var user User
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO users (public_id, email, password_hash, name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, public_id::text, email, password_hash, name, role, created_at, updated_at
	`, publicID, email, passwordHash, name).Scan(
		&user.ID,
		&user.PublicID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}

	return user, nil
}

func (s *PostgresUserStore) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, public_id::text, email, password_hash, name, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.PublicID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *PostgresUserStore) FindByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, public_id::text, email, password_hash, name, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.PublicID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

type MemoryUserStore struct {
	mu      sync.Mutex
	nextID  int64
	byID    map[int64]User
	byEmail map[string]int64
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		nextID:  1,
		byID:    make(map[int64]User),
		byEmail: make(map[string]int64),
	}
}

func (s *MemoryUserStore) CreateUser(_ context.Context, email string, passwordHash string, name string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email = strings.ToLower(email)
	if _, ok := s.byEmail[email]; ok {
		return User{}, ErrEmailExists
	}

	now := time.Now().UTC()
	publicID, err := newUUID()
	if err != nil {
		return User{}, err
	}

	user := User{
		ID:           s.nextID,
		PublicID:     publicID,
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Role:         "user",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextID++
	s.byID[user.ID] = user
	s.byEmail[user.Email] = user.ID

	return user, nil
}

func (s *MemoryUserStore) FindByEmail(_ context.Context, email string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return s.byID[id], nil
}

func (s *MemoryUserStore) FindByID(_ context.Context, id int64) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return strings.Join([]string{
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	}, "-"), nil
}
