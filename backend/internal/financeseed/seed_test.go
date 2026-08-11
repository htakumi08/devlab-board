package financeseed

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"testing"

	platformpostgres "devlab-board/backend/internal/platform/postgres"
	_ "github.com/lib/pq"
)

// TestDeterministicUUIDIsStableAndCanonical は、同じユーザーとkeyから常に同じcanonical UUIDを生成することを確認する。
// 必要な理由: seedを反復してもfixtureの公開IDと件数を安定させるため。
func TestDeterministicUUIDIsStableAndCanonical(t *testing.T) {
	const userPublicID = "00000000-0000-4000-8000-000000000001"

	first := deterministicUUID(userPublicID, "account-checking")
	second := deterministicUUID(userPublicID, "account-checking")
	different := deterministicUUID(userPublicID, "account-savings")

	if first != second {
		t.Fatalf("expected a stable UUID, got %q and %q", first, second)
	}
	if first == different {
		t.Fatal("expected different fixture keys to produce different UUIDs")
	}
	canonical := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !canonical.MatchString(first) {
		t.Fatalf("expected a canonical version 5 UUID, got %q", first)
	}
}

// TestValidateConfigRestrictsEnvironmentsAndActions は、seedが明示した非本番環境とactionだけで動くことを確認する。
// 必要な理由: 誤設定したseedが本番データや想定外の対象を変更することを防ぐため。
func TestValidateConfigRestrictsEnvironmentsAndActions(t *testing.T) {
	validEnvironments := []string{"local", "development", "test"}
	for _, environment := range validEnvironments {
		// 各許可環境でapplyを実行できることを確認する。
		if err := ValidateConfig(Config{AppEnv: environment, UserEmail: "synthetic@example.invalid", Action: ActionApply}); err != nil {
			t.Fatalf("expected %q to be allowed: %v", environment, err)
		}
	}

	invalid := []Config{
		// productionは明示的に拒否する。
		{AppEnv: "production", UserEmail: "synthetic@example.invalid", Action: ActionApply},
		// 環境の省略を拒否する。
		{UserEmail: "synthetic@example.invalid", Action: ActionApply},
		// 対象ユーザーの省略を拒否する。
		{AppEnv: "local", Action: ActionApply},
		// 未知のactionを拒否する。
		{AppEnv: "local", UserEmail: "synthetic@example.invalid", Action: Action("replace")},
	}
	for index, config := range invalid {
		if err := ValidateConfig(config); err == nil {
			t.Fatalf("expected invalid config case %d to be rejected", index)
		}
	}
}

// TestRunApplyAndCleanAreRepeatableAndOwnerScoped は、apply/cleanの反復性とユーザー分離をPostgreSQLで確認する。
// 必要な理由: fixture再投入や削除が通常データと他ユーザーのFinanceデータを壊さないことを保証するため。
func TestRunApplyAndCleanAreRepeatableAndOwnerScoped(t *testing.T) {
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
	ownerID, ownerEmail := insertSeedTestUser(t, ctx, db, "owner")
	otherID, otherEmail := insertSeedTestUser(t, ctx, db, "other")
	// db.Closeより先に実行し、成功したintegration testもfixtureを残さない。
	defer func() {
		if _, cleanupErr := db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, ownerID, otherID); cleanupErr != nil {
			t.Errorf("clean seed test users: %v", cleanupErr)
		}
	}()
	insertNormalAccount(t, ctx, db, ownerID)

	ownerConfig := Config{AppEnv: "test", UserEmail: ownerEmail, Action: ActionApply}
	otherConfig := Config{AppEnv: "test", UserEmail: otherEmail, Action: ActionApply}
	if result, err := Run(ctx, db, ownerConfig); err != nil || result.Accounts != 2 || result.Transactions < 36 {
		t.Fatalf("first owner apply: result=%#v err=%v", result, err)
	}
	firstIDs := seededPublicIDs(t, ctx, db, ownerID)
	if _, err := Run(ctx, db, ownerConfig); err != nil {
		t.Fatalf("second owner apply: %v", err)
	}
	secondIDs := seededPublicIDs(t, ctx, db, ownerID)
	if firstIDs != secondIDs {
		t.Fatalf("expected repeatable public IDs, got %q then %q", firstIDs, secondIDs)
	}
	assertSeedCoverage(t, ctx, db, ownerID)
	if _, err := Run(ctx, db, otherConfig); err != nil {
		t.Fatalf("other owner apply: %v", err)
	}

	ownerConfig.Action = ActionClean
	if result, err := Run(ctx, db, ownerConfig); err != nil || result.Accounts != 0 || result.Transactions != 0 {
		t.Fatalf("first owner clean: result=%#v err=%v", result, err)
	}
	if _, err := Run(ctx, db, ownerConfig); err != nil {
		t.Fatalf("second owner clean: %v", err)
	}
	if got := countRows(t, ctx, db, `SELECT COUNT(*) FROM finance_accounts WHERE user_id = $1`, ownerID); got != 1 {
		t.Fatalf("expected the normal owner account to remain, got %d accounts", got)
	}
	if got := countRows(t, ctx, db, `SELECT COUNT(*) FROM finance_accounts WHERE user_id = $1`, otherID); got != 2 {
		t.Fatalf("expected the other user's seed accounts to remain, got %d", got)
	}
}

func assertSeedCoverage(t *testing.T, ctx context.Context, db *sql.DB, userID int64) {
	t.Helper()
	var transactions, accounts, directions, statuses, categories, nullMerchants, nullCategories, tiedTimes int
	var maximumAmount string
	err := db.QueryRowContext(ctx, `
		SELECT
			COUNT(*), COUNT(DISTINCT account_id), COUNT(DISTINCT direction), COUNT(DISTINCT status),
			COUNT(DISTINCT category_code), COUNT(*) FILTER (WHERE merchant IS NULL),
			COUNT(*) FILTER (WHERE category_code IS NULL), MAX(amount_minor)::text
		FROM finance_transactions
		WHERE user_id = $1
	`, userID).Scan(
		&transactions, &accounts, &directions, &statuses, &categories,
		&nullMerchants, &nullCategories, &maximumAmount,
	)
	if err != nil {
		t.Fatalf("read fixture coverage: %v", err)
	}
	if transactions != 40 || accounts != 2 || directions != 2 || statuses != 3 || categories != 5 {
		t.Fatalf("fixture dimensions are incomplete: transactions=%d accounts=%d directions=%d statuses=%d categories=%d",
			transactions, accounts, directions, statuses, categories)
	}
	if nullMerchants == 0 || nullCategories == 0 || maximumAmount != "9007199254740993" {
		t.Fatalf("fixture boundary coverage is incomplete")
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT COALESCE(posted_at, authorized_at)
			FROM finance_transactions
			WHERE user_id = $1
			GROUP BY COALESCE(posted_at, authorized_at)
			HAVING COUNT(*) > 1
		) tied
	`, userID).Scan(&tiedTimes); err != nil {
		t.Fatalf("read fixture tie coverage: %v", err)
	}
	if tiedTimes == 0 {
		t.Fatal("expected fixture transactions with the same occurred timestamp")
	}
}

// TestRunRejectsProductionBeforeDatabaseAccess は、production指定をDBへ触れる前に拒否することを確認する。
// 必要な理由: 接続状態にかかわらずproductionでfixture変更を開始させないため。
func TestRunRejectsProductionBeforeDatabaseAccess(t *testing.T) {
	_, err := Run(context.Background(), nil, Config{
		AppEnv: "production", UserEmail: "synthetic@example.invalid", Action: ActionApply,
	})
	if !errors.Is(err, ErrUnsafeEnvironment) {
		t.Fatalf("expected unsafe environment error, got %v", err)
	}
}

func insertSeedTestUser(t *testing.T, ctx context.Context, db *sql.DB, label string) (int64, string) {
	t.Helper()
	publicID := deterministicUUID("finance-seed-integration", label)
	email := label + "-" + publicID + "@example.invalid"
	var userID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (public_id, email, password_hash, name)
		VALUES ($1::uuid, $2, 'test-hash', 'Synthetic Test User')
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, publicID, email).Scan(&userID)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return userID, email
}

func insertNormalAccount(t *testing.T, ctx context.Context, db *sql.DB, userID int64) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO finance_accounts (
			public_id, user_id, name, account_type, mask, currency,
			current_balance_minor, available_balance_minor, status, balance_as_of
		)
		VALUES ($1::uuid, $2, 'Normal account', 'checking', '1111', 'JPY', 1, 1, 'active', '2026-08-11T00:00:00Z')
		ON CONFLICT (public_id) DO NOTHING
	`, deterministicUUID("normal-account", "owner"), userID)
	if err != nil {
		t.Fatalf("insert normal account: %v", err)
	}
}

func seededPublicIDs(t *testing.T, ctx context.Context, db *sql.DB, userID int64) string {
	t.Helper()
	var ids string
	err := db.QueryRowContext(ctx, `
		SELECT string_agg(public_id::text, ',' ORDER BY public_id::text)
		FROM finance_transactions
		WHERE user_id = $1
	`, userID).Scan(&ids)
	if err != nil {
		t.Fatalf("read seeded public IDs: %v", err)
	}
	return ids
}

func countRows(t *testing.T, ctx context.Context, db *sql.DB, statement string, argument any) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, statement, argument).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
