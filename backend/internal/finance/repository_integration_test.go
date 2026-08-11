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

// TestPostgresRepositoryTransactionsPaginatesWithinOwner は、所有者を分離し同一日時でも2ページに重複・欠落がないことを確認する。
// 必要な理由: keyset paginationが他ユーザー取引を漏らさず、内部IDによる安定順を維持するため。
func TestPostgresRepositoryTransactionsPaginatesWithinOwner(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "transactions-owner")
	otherID := insertTestUser(t, ctx, tx, "transactions-other")
	ownerAccountID, ownerAccountPublicID := insertTestAccount(t, ctx, tx, ownerID, "JPY", 180000, 165000)
	otherAccountID, _ := insertTestAccount(t, ctx, tx, otherID, "JPY", 999999, 999999)
	sameOccurredAt := time.Date(2026, time.August, 11, 1, 5, 0, 0, time.UTC)
	oldestID := insertNullableTestTransactionRecord(t, ctx, tx, ownerID, ownerAccountID, sameOccurredAt.Add(-time.Hour))
	middleID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: ownerAccountID, AuthorizedAt: sameOccurredAt})
	newestID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: ownerID, AccountID: ownerAccountID, AuthorizedAt: sameOccurredAt})
	insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: otherID, AccountID: otherAccountID, AuthorizedAt: sameOccurredAt.Add(time.Hour)})

	repository := NewPostgresRepository(tx)
	first, err := repository.Transactions(ctx, ownerID, transactionPageQuery{Limit: 2})
	if err != nil {
		t.Fatalf("get first transaction page: %v", err)
	}
	if !first.HasMore || len(first.Transactions) != 2 || first.Transactions[0].ID != newestID || first.Transactions[1].ID != middleID {
		t.Fatalf("unexpected first page: %#v", first)
	}
	if first.Transactions[0].AccountID != ownerAccountPublicID || first.Transactions[0].AmountMinor != "4200" {
		t.Fatalf("expected public account ID and decimal amount, got %#v", first.Transactions[0])
	}

	// 1ページ目末尾の公開IDをowner条件で解決して次ページを取得する。
	cursorID := first.Transactions[len(first.Transactions)-1].ID
	second, err := repository.Transactions(ctx, ownerID, transactionPageQuery{CursorPublicID: &cursorID, Limit: 2})
	if err != nil {
		t.Fatalf("get second transaction page: %v", err)
	}
	if second.HasMore || len(second.Transactions) != 1 || second.Transactions[0].ID != oldestID {
		t.Fatalf("unexpected second page: %#v", second)
	}
	if second.Transactions[0].Merchant != nil || second.Transactions[0].Category != nil {
		t.Fatalf("expected nullable merchant and category to remain null, got %#v", second.Transactions[0])
	}
}

// TestPostgresRepositoryTransactionsHidesInvalidCursorOwners は、不存在と他所有者cursorを同じerrorへ正規化することを確認する。
// 必要な理由: cursorから他ユーザー取引の存在を推測できないようrepositoryで所有者境界を強制するため。
func TestPostgresRepositoryTransactionsHidesInvalidCursorOwners(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "transactions-cursor-owner")
	otherID := insertTestUser(t, ctx, tx, "transactions-cursor-other")
	otherAccountID, _ := insertTestAccount(t, ctx, tx, otherID, "JPY", 999999, 999999)
	otherCursorID := insertTestTransactionRecord(t, ctx, tx, transactionRecord{UserID: otherID, AccountID: otherAccountID, AuthorizedAt: time.Now()})

	cases := []struct {
		name     string
		cursorID string
	}{
		// 同じ所有者に存在しない公開IDをresource非開示errorにする。
		{name: "missing", cursorID: testUUID(t)},
		// 他所有者の公開IDもresource非開示errorにする。
		{name: "other owner", cursorID: otherCursorID},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPostgresRepository(tx).Transactions(ctx, ownerID, transactionPageQuery{CursorPublicID: &test.cursorID, Limit: 25})
			if !errors.Is(err, ErrInvalidTransactionQuery) {
				t.Fatalf("expected invalid query for cursor %q, got %v", test.cursorID, err)
			}
		})
	}
}

// TestPostgresRepositoryTransactionsFiltersSortsAndPaginates は、全filterとoldest paginationを所有者境界内で組み合わせられることを確認する。
// 必要な理由: Slice 3Bの各条件が同じoccurredAt定義を使い、filter済みページ間でも重複や欠落を起こさないため。
func TestPostgresRepositoryTransactionsFiltersSortsAndPaginates(t *testing.T) {
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

	ownerID := insertTestUser(t, ctx, tx, "transactions-filter-owner")
	otherID := insertTestUser(t, ctx, tx, "transactions-filter-other")
	accountA := testUUID(t)
	accountAID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: accountA, UserID: ownerID, Name: "Filter Account A",
		Status: "active", Currency: "JPY", Current: 100000, Available: int64Pointer(90000),
	})
	accountBID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: testUUID(t), UserID: ownerID, Name: "Filter Account B",
		Status: "active", Currency: "JPY", Current: 200000, Available: int64Pointer(190000),
	})
	otherAccountPublicID := testUUID(t)
	otherAccountID := insertTestAccountRecord(t, ctx, tx, testAccountRecord{
		PublicID: otherAccountPublicID, UserID: otherID, Name: "Other Filter Account",
		Status: "active", Currency: "JPY", Current: 300000, Available: int64Pointer(290000),
	})
	base := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	oldestGroceriesID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountAID, Name: "Oldest groceries", Category: stringPointer("groceries"),
		Direction: "debit", Status: "posted", AuthorizedAt: base,
	})
	transportationID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountAID, Name: "Transportation income", Category: stringPointer("transportation"),
		Direction: "credit", Status: "pending", AuthorizedAt: base.AddDate(0, 0, 1),
	})
	uncategorizedID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountAID, Name: "Uncategorized reversal",
		Direction: "debit", Status: "reversed", AuthorizedAt: base.AddDate(0, 0, 2),
	})
	firstTieID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountAID, Name: "First tie", Category: stringPointer("groceries"),
		Direction: "debit", Status: "posted", AuthorizedAt: base.AddDate(0, 0, 2),
	})
	secondTieID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountAID, Name: "Second tie", Category: stringPointer("groceries"),
		Direction: "debit", Status: "posted", AuthorizedAt: base.AddDate(0, 0, 2),
	})
	otherAccountGroceriesID := insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: ownerID, AccountID: accountBID, Name: "Other account groceries", Category: stringPointer("groceries"),
		Direction: "debit", Status: "posted", AuthorizedAt: base.AddDate(0, 0, 3),
	})
	insertFilterTransactionRecord(t, ctx, tx, filterTransactionRecord{
		UserID: otherID, AccountID: otherAccountID, Name: "Hidden other owner", Category: stringPointer("groceries"),
		Direction: "debit", Status: "posted", AuthorizedAt: base.AddDate(0, 0, 4),
	})

	repository := NewPostgresRepository(tx)
	dateFrom := base.AddDate(0, 0, 1)
	dateToExclusive := base.AddDate(0, 0, 4)
	groceries := "groceries"
	debit := "debit"
	posted := "posted"
	missingAccount := testUUID(t)
	combined, err := repository.Transactions(ctx, ownerID, transactionPageQuery{
		AccountPublicID: &accountA, DateFrom: &dateFrom, DateToExclusive: &dateToExclusive,
		Category: &groceries, Direction: &debit, Status: &posted, Sort: transactionSortNewest, Limit: 25,
	})
	if err != nil {
		t.Fatalf("get combined filtered transactions: %v", err)
	}
	if got := transactionIDs(combined.Transactions); !equalStrings(got, []string{secondTieID, firstTieID}) {
		t.Fatalf("expected combined owner filters to return tie rows newest-first, got %#v", got)
	}

	cases := []struct {
		name  string
		query transactionPageQuery
		want  []string
	}{
		// 口座filterは他口座と他所有者を除外する。
		{name: "account", query: transactionPageQuery{AccountPublicID: &accountA, Limit: 25}, want: []string{secondTieID, firstTieID, uncategorizedID, transportationID, oldestGroceriesID}},
		// UTC期間は下端inclusive、上端exclusiveで同じoccurredAtを使う。
		{name: "date", query: transactionPageQuery{DateFrom: &dateFrom, DateToExclusive: &dateToExclusive, Limit: 25}, want: []string{otherAccountGroceriesID, secondTieID, firstTieID, uncategorizedID, transportationID}},
		// category master codeで絞り込む。
		{name: "category", query: transactionPageQuery{Category: &groceries, Limit: 25}, want: []string{otherAccountGroceriesID, secondTieID, firstTieID, oldestGroceriesID}},
		// 予約値uncategorizedはNULL categoryだけを返す。
		{name: "uncategorized", query: transactionPageQuery{Category: stringPointer("uncategorized"), Limit: 25}, want: []string{uncategorizedID}},
		// direction filterを単独でも適用できる。
		{name: "direction", query: transactionPageQuery{Direction: stringPointer("credit"), Limit: 25}, want: []string{transportationID}},
		// status filterを単独でも適用できる。
		{name: "status", query: transactionPageQuery{Status: stringPointer("reversed"), Limit: 25}, want: []string{uncategorizedID}},
		// canonicalでも不存在の口座はresource情報を出さず空ページにする。
		{name: "missing account", query: transactionPageQuery{AccountPublicID: &missingAccount, Limit: 25}, want: []string{}},
		// 他所有者の口座も不存在と区別せず空ページにする。
		{name: "unowned account", query: transactionPageQuery{AccountPublicID: &otherAccountPublicID, Limit: 25}, want: []string{}},
		// 形式が有効でもmasterにないcategoryは空ページにする。
		{name: "unknown category", query: transactionPageQuery{Category: stringPointer("future-category"), Limit: 25}, want: []string{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			page, err := repository.Transactions(ctx, ownerID, test.query)
			if err != nil {
				t.Fatalf("get filtered transactions: %v", err)
			}
			if got := transactionIDs(page.Transactions); !equalStrings(got, test.want) {
				t.Fatalf("expected IDs %#v, got %#v", test.want, got)
			}
		})
	}

	// oldestでも同一日時の内部IDを昇順tie-breakerに使い、2ページを重複なく取得する。
	firstPage, err := repository.Transactions(ctx, ownerID, transactionPageQuery{Category: &groceries, Sort: transactionSortOldest, Limit: 2})
	if err != nil {
		t.Fatalf("get oldest first page: %v", err)
	}
	if !firstPage.HasMore || !equalStrings(transactionIDs(firstPage.Transactions), []string{oldestGroceriesID, firstTieID}) {
		t.Fatalf("unexpected oldest first page: %#v", firstPage)
	}
	cursorID := firstPage.Transactions[1].ID
	secondPage, err := repository.Transactions(ctx, ownerID, transactionPageQuery{CursorPublicID: &cursorID, Category: &groceries, Sort: transactionSortOldest, Limit: 2})
	if err != nil {
		t.Fatalf("get oldest second page: %v", err)
	}
	if secondPage.HasMore || !equalStrings(transactionIDs(secondPage.Transactions), []string{secondTieID, otherAccountGroceriesID}) {
		t.Fatalf("unexpected oldest second page: %#v", secondPage)
	}

	// cursor行が現在のfilterに一致しなければ、任意の位置からページを開始せず同じ400用errorにする。
	if _, err := repository.Transactions(ctx, ownerID, transactionPageQuery{CursorPublicID: &transportationID, Category: &groceries, Limit: 25}); !errors.Is(err, ErrInvalidTransactionQuery) {
		t.Fatalf("expected mismatched filter cursor to be invalid, got %v", err)
	}
}

// TestPostgresRepositoryTransactionCategories は、category masterをdisplay_orderとcodeの安定順で返すことを確認する。
// 必要な理由: frontendが選択肢をhard-codeせず、DBを正として毎回同じ順序で表示するため。
func TestPostgresRepositoryTransactionCategories(t *testing.T) {
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

	categories, err := NewPostgresRepository(db).TransactionCategories(ctx)
	if err != nil {
		t.Fatalf("get transaction categories: %v", err)
	}
	wantCodes := []string{"income", "groceries", "transportation", "utilities", "other"}
	gotCodes := make([]string, 0, len(categories))
	for _, category := range categories {
		gotCodes = append(gotCodes, category.Code)
	}
	if !equalStrings(gotCodes, wantCodes) {
		t.Fatalf("expected category order %#v, got %#v", wantCodes, gotCodes)
	}
}

// TestPostgresRepositoryTransactionsUsesRecentIndex は、取引一覧queryが既存のuser recent indexをSortなしで使えることを確認する。
// 必要な理由: cursor paginationでも全取引sortへ退行せず、既存indexと固定順序を一致させるため。
func TestPostgresRepositoryTransactionsUsesRecentIndex(t *testing.T) {
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
	connection, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("get query plan connection: %v", err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disable sequential scan: %v", err)
	}
	if _, err := connection.ExecContext(ctx, `SET enable_bitmapscan = off`); err != nil {
		t.Fatalf("disable bitmap scan: %v", err)
	}
	rows, err := connection.QueryContext(ctx, `
		EXPLAIN (COSTS OFF)
		SELECT t.public_id, a.public_id, a.name
		FROM finance_transactions t
		JOIN finance_accounts a ON a.id = t.account_id AND a.user_id = t.user_id
		WHERE t.user_id = 1
		ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
		LIMIT 26
	`)
	if err != nil {
		t.Fatalf("explain transactions query: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan query plan: %v", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read query plan: %v", err)
	}
	plan := strings.Join(lines, "\n")
	if !strings.Contains(plan, "finance_transactions_user_recent_idx") || strings.Contains(plan, "Sort") {
		t.Fatalf("expected user recent index without Sort, got plan:\n%s", plan)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close newest plan rows: %v", err)
	}

	queries := []struct {
		name       string
		statement  string
		indexNames []string
	}{
		// DESC indexのbackward scanでoldestも追加Sortなしにする。
		{name: "oldest", indexNames: []string{"finance_transactions_user_recent_idx"}, statement: `
			SELECT t.public_id
			FROM finance_transactions t
			WHERE t.user_id = 1
			ORDER BY COALESCE(t.posted_at, t.authorized_at) ASC, t.id ASC
			LIMIT 26`},
		// expression日時のrangeもuser recent index上で評価する。
		{name: "date range", indexNames: []string{"finance_transactions_user_recent_idx"}, statement: `
			SELECT t.public_id
			FROM finance_transactions t
			WHERE t.user_id = 1
			  AND COALESCE(t.posted_at, t.authorized_at) >= TIMESTAMPTZ '2026-08-01 00:00:00Z'
			  AND COALESCE(t.posted_at, t.authorized_at) < TIMESTAMPTZ '2026-09-01 00:00:00Z'
			ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
			LIMIT 26`},
		// account_idとuser_idの選択度に応じたrecent indexで順序を維持する。
		{name: "account", indexNames: []string{"finance_transactions_account_recent_idx", "finance_transactions_user_recent_idx"}, statement: `
			SELECT t.public_id
			FROM finance_transactions t
			WHERE t.user_id = 1 AND t.account_id = 1
			ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
			LIMIT 26`},
	}
	for _, test := range queries {
		t.Run(test.name, func(t *testing.T) {
			planRows, err := connection.QueryContext(ctx, "EXPLAIN (COSTS OFF) "+test.statement)
			if err != nil {
				t.Fatalf("explain query: %v", err)
			}
			defer planRows.Close()
			var planLines []string
			for planRows.Next() {
				var line string
				if err := planRows.Scan(&line); err != nil {
					t.Fatalf("scan query plan: %v", err)
				}
				planLines = append(planLines, line)
			}
			if err := planRows.Err(); err != nil {
				t.Fatalf("read query plan: %v", err)
			}
			plan := strings.Join(planLines, "\n")
			usesExpectedIndex := false
			for _, indexName := range test.indexNames {
				usesExpectedIndex = usesExpectedIndex || strings.Contains(plan, indexName)
			}
			if !usesExpectedIndex || strings.Contains(plan, "Sort") {
				t.Fatalf("expected one of %v without Sort, got plan:\n%s", test.indexNames, plan)
			}
		})
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

type filterTransactionRecord struct {
	UserID       int64
	AccountID    int64
	Name         string
	Category     *string
	Direction    string
	Status       string
	AuthorizedAt time.Time
}

func insertFilterTransactionRecord(t *testing.T, ctx context.Context, tx *sql.Tx, transaction filterTransactionRecord) string {
	t.Helper()
	publicID := testUUID(t)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO finance_transactions (
			public_id, user_id, account_id, name, merchant, amount_minor,
			currency, direction, status, category_code, authorized_at, posted_at
		)
		VALUES ($1, $2, $3, $4, 'Synthetic Filter Merchant', 4200,
			'JPY', $5, $6, $7, $8, $8)
	`, publicID, transaction.UserID, transaction.AccountID, transaction.Name, transaction.Direction, transaction.Status, transaction.Category, transaction.AuthorizedAt)
	if err != nil {
		t.Fatalf("insert filter transaction record: %v", err)
	}
	return publicID
}

func transactionIDs(transactions []Transaction) []string {
	ids := make([]string, 0, len(transactions))
	for _, transaction := range transactions {
		ids = append(ids, transaction.ID)
	}
	return ids
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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

func insertNullableTestTransactionRecord(t *testing.T, ctx context.Context, tx *sql.Tx, userID int64, accountID int64, authorizedAt time.Time) string {
	t.Helper()
	publicID := testUUID(t)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO finance_transactions (
			public_id, user_id, account_id, name, amount_minor,
			currency, direction, status, authorized_at
		)
		VALUES ($1, $2, $3, 'Uncategorized transaction', 500,
			'JPY', 'credit', 'pending', $4)
	`, publicID, userID, accountID, authorizedAt)
	if err != nil {
		t.Fatalf("insert nullable transaction record: %v", err)
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
