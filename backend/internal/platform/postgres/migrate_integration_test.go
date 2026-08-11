package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// TestMigrateAppliesOnceAndDetectsChanges は、migrationの初回適用、再実行、checksum改変拒否を確認する。
// 必要な理由: 複数回起動でDDLを重複実行せず、適用済みSQLの書き換えを見逃さないため。
func TestMigrateAppliesOnceAndDetectsChanges(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}

	ctx := context.Background()
	baseDB, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open base database: %v", err)
	}
	defer baseDB.Close()

	schema := "migration_test_" + randomHex(t, 8)
	if _, err := baseDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer baseDB.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)

	testURL, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	query := testURL.Query()
	query.Set("search_path", schema)
	testURL.RawQuery = query.Encode()

	db, err := sql.Open("postgres", testURL.String())
	if err != nil {
		t.Fatalf("open schema database: %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}

	var migrationCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != 4 {
		t.Fatalf("expected 4 applied migrations, got %d", migrationCount)
	}

	var categoryCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_transaction_categories`).Scan(&categoryCount); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if categoryCount != 5 {
		t.Fatalf("expected 5 category fixtures, got %d", categoryCount)
	}

	// Summary queryとindexの式が一致することを確認する。
	// 必要な理由: LIMIT 5でも式が不一致ならユーザーの全取引をsortする回帰が起きるため。
	var recentIndexDefinition string
	if err := db.QueryRowContext(ctx, `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename = 'finance_transactions'
		  AND indexname = 'finance_transactions_user_recent_idx'
	`).Scan(&recentIndexDefinition); err != nil {
		t.Fatalf("read Finance recent transaction index: %v", err)
	}
	if !strings.Contains(recentIndexDefinition, "COALESCE(posted_at, authorized_at) DESC") ||
		!strings.Contains(recentIndexDefinition, "user_id") ||
		!strings.Contains(recentIndexDefinition, "id DESC") {
		t.Fatalf("expected summary ordering expression in index, got %s", recentIndexDefinition)
	}

	// 口座詳細queryと口座別indexの式が一致し、plannerがSortなしで利用できることを確認する。
	// 必要な理由: 固定5件でも口座の全取引sortが発生する性能回帰を防ぐため。
	var accountRecentIndexDefinition string
	if err := db.QueryRowContext(ctx, `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename = 'finance_transactions'
		  AND indexname = 'finance_transactions_account_recent_idx'
	`).Scan(&accountRecentIndexDefinition); err != nil {
		t.Fatalf("read account recent transaction index: %v", err)
	}
	if !strings.Contains(accountRecentIndexDefinition, "COALESCE(posted_at, authorized_at) DESC") ||
		!strings.Contains(accountRecentIndexDefinition, "account_id") ||
		!strings.Contains(accountRecentIndexDefinition, "id DESC") {
		t.Fatalf("expected account detail ordering expression in index, got %s", accountRecentIndexDefinition)
	}

	planConnection, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("get connection for account recent query plan: %v", err)
	}
	defer planConnection.Close()
	if _, err := planConnection.ExecContext(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disable sequential scan for index plan assertion: %v", err)
	}
	planRows, err := planConnection.QueryContext(ctx, `
		EXPLAIN (COSTS OFF)
		SELECT t.public_id, c.code, c.label
		FROM finance_transactions t
		LEFT JOIN finance_transaction_categories c ON c.code = t.category_code
		WHERE t.account_id = 1 AND t.user_id = 1
		ORDER BY COALESCE(t.posted_at, t.authorized_at) DESC, t.id DESC
		LIMIT 5
	`)
	if err != nil {
		t.Fatalf("explain account recent query: %v", err)
	}
	defer planRows.Close()
	var planLines []string
	for planRows.Next() {
		var line string
		if err := planRows.Scan(&line); err != nil {
			t.Fatalf("scan account recent query plan: %v", err)
		}
		planLines = append(planLines, line)
	}
	plan := strings.Join(planLines, "\n")
	if !strings.Contains(plan, "finance_transactions_account_recent_idx") || strings.Contains(plan, "Sort") {
		t.Fatalf("expected account recent index without Sort, got plan:\n%s", plan)
	}

	if _, err := db.ExecContext(ctx, `UPDATE schema_migrations SET checksum = 'changed' WHERE version = 1`); err != nil {
		t.Fatalf("change checksum for test: %v", err)
	}
	if err := Migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("expected checksum change error, got %v", err)
	}
}

// TestMigrateReportsConnectionAndSchemaErrors は、接続不能と書込先schema不正を明示的なerrorにすることを確認する。
// 必要な理由: migration失敗時にapplication起動を止め、部分的なschemaで処理を続行しないため。
func TestMigrateReportsConnectionAndSchemaErrors(t *testing.T) {
	closedDB, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closedDB.Close()
	if err := Migrate(context.Background(), closedDB); err == nil || !strings.Contains(err.Error(), "get migration connection") {
		t.Fatalf("expected closed connection error, got %v", err)
	}

	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration tests")
	}
	testURL, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	query := testURL.Query()
	query.Set("search_path", "missing_schema_"+randomHex(t, 8))
	testURL.RawQuery = query.Encode()

	db, err := sql.Open("postgres", testURL.String())
	if err != nil {
		t.Fatalf("open invalid-schema database: %v", err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err == nil || !strings.Contains(err.Error(), "create schema_migrations") {
		t.Fatalf("expected schema creation error, got %v", err)
	}
}

func randomHex(t *testing.T, byteCount int) string {
	t.Helper()
	value := make([]byte, byteCount)
	if _, err := rand.Read(value); err != nil {
		t.Fatalf("generate random schema suffix: %v", err)
	}
	return hex.EncodeToString(value)
}
