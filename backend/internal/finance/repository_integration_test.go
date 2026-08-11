package finance

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	platformpostgres "devlab-board/backend/internal/platform/postgres"
	_ "github.com/lib/pq"
)

// TestPostgresRepositorySummary は、Finance summaryが所有者で分離され通貨別に集計されることを確認する。
// 必要な理由: 金融データの他ユーザー漏えいと、異なる通貨の誤加算をrepository境界で防ぐため。
func TestPostgresRepositorySummary(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := platformpostgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback()

	ownerID := insertTestUser(t, ctx, tx, "summary-owner")
	otherID := insertTestUser(t, ctx, tx, "summary-other")
	ownerAccountID, ownerAccountPublicID := insertTestAccount(t, ctx, tx, ownerID, "JPY", 180000, 165000)
	insertTestAccount(t, ctx, tx, ownerID, "USD", 12345, 12000)
	otherAccountID, _ := insertTestAccount(t, ctx, tx, otherID, "JPY", 999999, 999999)
	insertTestTransaction(t, ctx, tx, ownerID, ownerAccountID, 4200, "JPY", "debit")
	insertTestTransaction(t, ctx, tx, otherID, otherAccountID, 500000, "JPY", "credit")

	// 所有者と口座所有者が一致しない取引を複合外部キーが拒否することを確認する。
	if _, err := tx.ExecContext(ctx, `SAVEPOINT ownership_check`); err != nil {
		t.Fatalf("create ownership savepoint: %v", err)
	}
	_, ownershipErr := tx.ExecContext(ctx, `
		INSERT INTO finance_transactions (
			public_id, user_id, account_id, name, amount_minor,
			currency, direction, status, authorized_at
		)
		VALUES ($1, $2, $3, 'Cross-owner transaction', 100, 'JPY', 'debit', 'posted', NOW())
	`, testUUID(t), ownerID, otherAccountID)
	if ownershipErr == nil {
		t.Fatal("expected cross-owner transaction to violate the ownership foreign key")
	}
	if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT ownership_check`); err != nil {
		t.Fatalf("rollback ownership check: %v", err)
	}

	repository := NewPostgresRepository(tx)
	summary, err := repository.Summary(ctx, ownerID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}

	if summary.AccountCount != 2 {
		t.Fatalf("expected 2 owner accounts, got %d", summary.AccountCount)
	}
	if len(summary.Balances) != 2 {
		t.Fatalf("expected balances to remain separated by currency, got %d", len(summary.Balances))
	}
	if len(summary.RecentTransactions) != 1 {
		t.Fatalf("expected only the owner's transaction, got %d", len(summary.RecentTransactions))
	}
	if summary.RecentTransactions[0].AccountID != ownerAccountPublicID {
		t.Fatalf("expected public account ID %q, got %q", ownerAccountPublicID, summary.RecentTransactions[0].AccountID)
	}
}

// TestPostgresRepositorySummaryEmpty は、口座がないユーザーへ空のsummaryを返すことを確認する。
// 必要な理由: 新規ユーザーをDB errorと誤認せず、UIが明示的なempty stateを表示できるようにするため。
func TestPostgresRepositorySummaryEmpty(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := platformpostgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback()

	userID := insertTestUser(t, ctx, tx, "summary-empty")
	summary, err := NewPostgresRepository(tx).Summary(ctx, userID)
	if err != nil {
		t.Fatalf("get empty summary: %v", err)
	}
	if summary.AccountCount != 0 || len(summary.Balances) != 0 || len(summary.RecentTransactions) != 0 || summary.AsOf != nil {
		t.Fatalf("expected an empty summary, got %#v", summary)
	}
}

// TestPostgresRepositorySummaryPreservesBigintSum は、int64上限を超えるSUM結果を10進文字列で保持することを確認する。
// 必要な理由: PostgreSQLのSUM(bigint)はnumericを返すため、Goのint64へ縮小すると正確な残高を失うため。
func TestPostgresRepositorySummaryPreservesBigintSum(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := platformpostgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback()

	userID := insertTestUser(t, ctx, tx, "summary-bigint-sum")
	insertTestAccount(t, ctx, tx, userID, "JPY", math.MaxInt64, math.MaxInt64)
	insertTestAccount(t, ctx, tx, userID, "JPY", math.MaxInt64, math.MaxInt64)

	summary, err := NewPostgresRepository(tx).Summary(ctx, userID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	const expected MinorAmount = "18446744073709551614"
	if len(summary.Balances) != 1 || summary.Balances[0].CurrentAmountMinor != expected {
		t.Fatalf("expected exact current balance sum %s, got %#v", expected, summary.Balances)
	}
	if summary.Balances[0].AvailableAmountMinor == nil || *summary.Balances[0].AvailableAmountMinor != expected {
		t.Fatalf("expected exact available balance sum %s, got %#v", expected, summary.Balances[0].AvailableAmountMinor)
	}
}

// TestPostgresRepositorySummaryUsesOldestBalanceTimestamp は、複数口座を含む集計のasOfが最古の残高日時になることを確認する。
// 必要な理由: 最新日時を表示すると、合計に含まれるstale口座まで最新残高であると誤認されるため。
func TestPostgresRepositorySummaryUsesOldestBalanceTimestamp(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := platformpostgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback()

	userID := insertTestUser(t, ctx, tx, "summary-as-of")
	oldest := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
	middle := oldest.Add(12 * time.Hour)
	newest := oldest.Add(24 * time.Hour)
	oldAccountID, _ := insertTestAccount(t, ctx, tx, userID, "JPY", 180000, 165000)
	middleAccountID, _ := insertTestAccount(t, ctx, tx, userID, "JPY", 20000, 18000)
	newAccountID, _ := insertTestAccount(t, ctx, tx, userID, "USD", 12345, 12000)
	if _, err := tx.ExecContext(ctx, `
		UPDATE finance_accounts
		SET balance_as_of = CASE id
			WHEN $1 THEN $2::timestamptz
			WHEN $3 THEN $4::timestamptz
			WHEN $5 THEN $6::timestamptz
		END
		WHERE id IN ($1, $3, $5)
	`, oldAccountID, oldest, middleAccountID, middle, newAccountID, newest); err != nil {
		t.Fatalf("set distinct balance timestamps: %v", err)
	}

	summary, err := NewPostgresRepository(tx).Summary(ctx, userID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if summary.AsOf == nil || !summary.AsOf.Equal(oldest) {
		t.Fatalf("expected oldest balance timestamp %s, got %v", oldest, summary.AsOf)
	}
}

func insertTestUser(t *testing.T, ctx context.Context, tx *sql.Tx, prefix string) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO users (public_id, email, password_hash, name)
		VALUES ($1, $2, 'test-hash', 'Finance Test')
		RETURNING id
	`, testUUID(t), fmt.Sprintf("%s-%d@example.test", prefix, time.Now().UnixNano())).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertTestAccount(t *testing.T, ctx context.Context, tx *sql.Tx, userID int64, currency string, current int64, available int64) (int64, string) {
	t.Helper()
	publicID := testUUID(t)
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO finance_accounts (
			public_id, user_id, name, account_type, mask, currency,
			current_balance_minor, available_balance_minor, status, balance_as_of
		)
		VALUES ($1, $2, 'Synthetic Checking', 'checking', '1234', $3, $4, $5, 'active', NOW())
		RETURNING id
	`, publicID, userID, currency, current, available).Scan(&id)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	return id, publicID
}

func insertTestTransaction(t *testing.T, ctx context.Context, tx *sql.Tx, userID int64, accountID int64, amount int64, currency string, direction string) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO finance_transactions (
			public_id, user_id, account_id, name, merchant, amount_minor,
			currency, direction, status, category_code, authorized_at, posted_at
		)
		VALUES ($1, $2, $3, 'Grocery Store', 'Sample Market', $4, $5, $6, 'posted', 'groceries', NOW(), NOW())
	`, testUUID(t), userID, accountID, amount, currency, direction)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
}

func testUUID(t *testing.T) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatalf("generate uuid: %v", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return strings.Join([]string{
		hex.EncodeToString(value[0:4]),
		hex.EncodeToString(value[4:6]),
		hex.EncodeToString(value[6:8]),
		hex.EncodeToString(value[8:10]),
		hex.EncodeToString(value[10:16]),
	}, "-")
}
