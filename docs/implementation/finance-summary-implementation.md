---
title: Finance Summary実装
status: implemented
last_updated: 2026-08-11
requirements: UC-02, FR-AUTH-002, FR-AUTH-003, FR-SUM-001, FR-SUM-002, FR-SUM-003, FR-SUM-004, FR-SUM-005
---

# Finance Summary実装

## 1. 目的

Finance DashboardのSlice 1を、画面入口だけでなくPostgreSQLの現在ユーザーの残高概要まで縦に接続する。

## 2. 実装方針

- DBMLをschema設計の正とし、Finance専用テーブルを `finance_` プレフィックスへ統一した
- 既存の起動時DDLを、checksum付き番号migrationへ移行した
- `internal/finance` にuse caseとPostgreSQL repositoryを分離した
- repositoryの全queryをsession由来の内部 `user_id` で制限した
- 取引の `user_id` と口座所有者は複合外部キーでも一致を強制した
- 金額はminor unit整数を表す10進文字列、残高は通貨別に返し、異なる通貨を加算しない
- Summaryの単一 `asOf` は集計対象口座の最古の残高日時とし、stale口座を最新に見せない
- 最近の取引queryと一致する `COALESCE(posted_at, authorized_at)` expression indexをversion 3で追加した
- available balanceが一部欠落する場合は部分合計を返さず `null` とする
- Frontendはloading、empty、error + retry、permission denied、successを区別し、Summary取得時の `401` では認証状態を破棄する
- server state libraryは1 endpoint段階では追加せず、Finance component内のlocal request stateに限定した

## 3. 主なファイル

- `docs/db-design.dbml`
- `docs/db-design.md`
- `docs/api/finance-summary.md`
- `backend/internal/platform/postgres/migrate.go`
- `backend/internal/platform/postgres/migrations/0001_auth_foundation.sql`
- `backend/internal/platform/postgres/migrations/0002_finance_summary.sql`
- `backend/internal/platform/postgres/migrations/0003_finance_recent_transaction_index.sql`
- `backend/internal/finance/summary.go`
- `backend/internal/finance/postgres_repository.go`
- `backend/internal/app/app.go`
- `frontend/src/features/finance/financeApi.ts`
- `frontend/src/features/finance/FinanceOverview.tsx`

## 4. Security境界

- clientからuser IDを受け取らない
- 未認証APIは既存middlewareで `401` とする
- APIにはUUID public IDだけを返す
- 完全な口座番号をschema、API、UI、logへ保持・表示しない
- DB error詳細はclient responseへ返さない
- 読み取りGETのみのため、このSliceでは新しいCSRF対象operationを追加しない

## 5. 検証結果

| 検証 | 結果 |
| --- | --- |
| Backend unit / handler test | 成功 |
| PostgreSQL integration test | migration初回・再適用・checksum、所有者分離、通貨別集計、empty、最古asOf、Summary index定義を確認 |
| Backend Finance package coverage | 84.4% |
| Frontend test | 15 tests passed |
| Frontend Finance coverage | statements 98.45%、branches 90.27%、functions 100%、lines 98.45% |
| Frontend production build | 成功 |
| Docker migration | version 1、2、3の適用を確認 |
| Browser | 認証済みempty state、console error 0件、横overflowなし |

## 6. MVP上の割り切り

- synthetic account / transactionを投入するseed commandはまだ実装しない
- 現在の既存ユーザーには口座がないためempty stateを表示する
- partial data、background refresh、offlineは後続のsummary改善で扱う
- migrationはlocalでは起動時適用を維持し、AWS環境では専用jobへ分離する

## 7. 次の候補

1. 開発用synthetic fixture command
2. Finance Accounts一覧と詳細
3. TanStack Query導入判断

## 8. 要確認・ヒアリング項目

- synthetic fixtureをユーザー登録時に自動作成するか、明示的な開発用commandで投入するか
- JPY以外の表示をMVPへ含めるか

## 9. 更新履歴

| 日付 | 内容 |
| --- | --- |
| 2026-08-11 | Finance SummaryのDB・API・UI実装と検証結果を記録 |
| 2026-08-11 | レビュー対応として金額文字列、最古asOf、version 3 index、401 / 403状態遷移を反映 |
