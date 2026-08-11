package finance

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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

// TestPostgresRepositoryAccounts は、口座一覧が所有者で分離されactive優先の安定順になることを確認する。
// 必要な理由: 一覧query自身で所有者境界と表示順を保証し、他ユーザー口座の漏えいを防ぐため。
func TestPostgresRepositoryAccounts(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "accounts-owner")
	otherID := insertTestUser(t, ctx, tx, "accounts-other")
	insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000003", UserID: ownerID, Name: "Zulu Closed",
		Status: "closed", Currency: "JPY", Current: math.MaxInt64, Available: nil,
	})
	insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000002", UserID: ownerID, Name: "Alpha Savings",
		Status: "active", Currency: "USD", Current: 12345, Available: int64Pointer(12000),
	})
	insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000001", UserID: ownerID, Name: "Alpha Savings",
		Status: "active", Currency: "JPY", Current: 180000, Available: int64Pointer(165000),
	})
	insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000004", UserID: otherID, Name: "Other Owner",
		Status: "active", Currency: "JPY", Current: 999999, Available: int64Pointer(999999),
	})

	accounts, err := NewPostgresRepository(tx).Accounts(ctx, ownerID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	wantIDs := []string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003",
	}
	if len(accounts) != len(wantIDs) {
		t.Fatalf("expected %d owner accounts, got %#v", len(wantIDs), accounts)
	}
	for index, wantID := range wantIDs {
		if accounts[index].ID != wantID {
			t.Fatalf("expected account %d to be %q, got %#v", index, wantID, accounts[index])
		}
	}
	if accounts[2].CurrentAmountMinor != "9223372036854775807" || accounts[2].AvailableAmountMinor != nil {
		t.Fatalf("expected exact bigint and null available balance, got %#v", accounts[2])
	}
}

// TestPostgresRepositoryAccountDetail は、所有口座とその最新5取引だけを返すことを確認する。
// 必要な理由: 口座詳細で他口座・他ユーザーの取引を混在させず、確定日時優先の順序を守るため。
func TestPostgresRepositoryAccountDetail(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "account-detail-owner")
	otherID := insertTestUser(t, ctx, tx, "account-detail-other")
	targetAccountID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000010", UserID: ownerID, Name: "Detail Account",
		Status: "active", Currency: "JPY", Current: 180000, Available: int64Pointer(165000),
	})
	otherAccountID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000020", UserID: ownerID, Name: "Other Account",
		Status: "active", Currency: "JPY", Current: 30000, Available: int64Pointer(30000),
	})
	otherOwnerAccountID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000030", UserID: otherID, Name: "Other Owner",
		Status: "active", Currency: "JPY", Current: 999999, Available: int64Pointer(999999),
	})

	base := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	oldestID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(time.Hour)})
	postedLatestID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(2 * time.Hour), PostedAt: timePointer(base.Add(10 * time.Hour))})
	thirdID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(3 * time.Hour)})
	fourthID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(4 * time.Hour)})
	fifthID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(5 * time.Hour)})
	sixthID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: targetAccountID, AuthorizedAt: base.Add(6 * time.Hour)})
	insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: otherAccountID, AuthorizedAt: base.Add(20 * time.Hour)})
	insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: otherID, AccountID: otherOwnerAccountID, AuthorizedAt: base.Add(30 * time.Hour)})

	detail, err := NewPostgresRepository(tx).Account(ctx, ownerID, "00000000-0000-4000-8000-000000000010")
	if err != nil {
		t.Fatalf("get account detail: %v", err)
	}
	wantTransactionIDs := []string{postedLatestID, sixthID, fifthID, fourthID, thirdID}
	if detail.ID != "00000000-0000-4000-8000-000000000010" || len(detail.RecentTransactions) != 5 {
		t.Fatalf("unexpected account detail: %#v", detail)
	}
	for index, wantID := range wantTransactionIDs {
		if detail.RecentTransactions[index].ID != wantID {
			t.Fatalf("expected transaction %d to be %q, got %#v", index, wantID, detail.RecentTransactions[index])
		}
		if detail.RecentTransactions[index].AccountID != detail.ID {
			t.Fatalf("expected public account ID only, got %#v", detail.RecentTransactions[index])
		}
	}
	for _, transaction := range detail.RecentTransactions {
		if transaction.ID == oldestID {
			t.Fatalf("expected oldest transaction outside fixed recent limit, got %#v", detail.RecentTransactions)
		}
	}
}

// TestPostgresRepositoryAccountDetailHidesOtherOwners は、未存在と他所有者の公開UUIDを同じerrorにすることを確認する。
// 必要な理由: repository queryの所有者条件を回避して口座の存在を推測されないようにするため。
func TestPostgresRepositoryAccountDetailHidesOtherOwners(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "account-hidden-owner")
	otherID := insertTestUser(t, ctx, tx, "account-hidden-other")
	insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: "00000000-0000-4000-8000-000000000099", UserID: otherID, Name: "Hidden Account",
		Status: "active", Currency: "JPY", Current: 100, Available: int64Pointer(100),
	})

	for _, publicID := range []string{
		"00000000-0000-4000-8000-000000000098",
		"00000000-0000-4000-8000-000000000099",
	} {
		_, err := NewPostgresRepository(tx).Account(ctx, ownerID, publicID)
		if !errors.Is(err, ErrAccountNotFound) {
			t.Fatalf("expected account not found for %q, got %v", publicID, err)
		}
	}
}

type testAccountRecord struct {
	PublicID  string
	UserID    int64
	Name      string
	Status    string
	Currency  string
	Current   int64
	Available *int64
}

func insertTestAccountRecord(t *testing.T, ctx context.Context, tx *sql.Tx, account testAccountRecord) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO finance_accounts (
			public_id, user_id, name, account_type, mask, currency,
			current_balance_minor, available_balance_minor, status, balance_as_of
		)
		VALUES ($1, $2, $3, 'checking', '1234', $4, $5, $6, $7, NOW())
		RETURNING id
	`, account.PublicID, account.UserID, account.Name, account.Currency, account.Current, account.Available, account.Status).Scan(&id)
	if err != nil {
		t.Fatalf("insert account record: %v", err)
	}
	return id
}

type transactionRecord struct {
	UserID       int64
	AccountID    int64
	AuthorizedAt time.Time
	PostedAt     *time.Time
}

func insertTestTransactionRecord(t *testing.T, ctx context.Context, tx *sql.Tx, transaction transactionRecord) string {
	t.Helper()
	publicID := testUUID(t)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO finance_transactions (
			public_id, user_id, account_id, name, merchant, amount_minor,
			currency, direction, status, category_code, authorized_at, posted_at
		)
		VALUES ($1, $2, $3, 'Account transaction', 'Synthetic Merchant', 4200,
			'JPY', 'debit', 'posted', 'groceries', $4, $5)
	`, publicID, transaction.UserID, transaction.AccountID, transaction.AuthorizedAt, transaction.PostedAt)
	if err != nil {
		t.Fatalf("insert transaction record: %v", err)
	}
	return publicID
}

func int64Pointer(value int64) *int64 {
	return &value
}

func timePointer(value time.Time) *time.Time {
	return &value
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
