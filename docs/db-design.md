# DB Design

## 1. 目的

この文書は、`devlab-board` で PostgreSQL を使う機能を追加するときの DB 設計メモである。

現時点では旧アプリの DB 設計を引き継がず、必要になった実験機能からテーブルを追加する。

対応する DBML は [db-design.dbml](./db-design.dbml) に置く。

## 2. 初期方針

- DB は PostgreSQL を前提にする
- 実環境では RDS for PostgreSQL を候補にする
- 最初から広いスキーマを作らない
- 1 実験機能 / 1 use case 単位でテーブルを追加する
- API に公開する ID は、必要になった時点で `public_id` を検討する
- Finance Dashboard専用テーブルは `finance_` プレフィックスへ統一する
- DBMLを論理・物理テーブル設計の正とし、migrationと同じ変更単位で更新する

## 3. 設計対象テーブル

DB 方針は PostgreSQL に統一する。ローカル開発では Docker Compose の PostgreSQL 18 系 service を使い、実環境では RDS for PostgreSQL を候補にする。

実装済み共有テーブル:

- `users`
- `sessions`
- `schema_migrations`

実装済みFinance Dashboard専用テーブル:

- `finance_accounts`
- `finance_transactions`
- `finance_transaction_categories`

DBML上の将来候補（migration未実装）:

- `experiments`
- `experiment_notes`

### 3.1 認証とセッション

- `users` はログインユーザ本体を管理する
- `sessions` はセッションライブラリ用の server-side session store として使う
- セッション Cookie にはライブラリが発行するセッショントークンを入れる
- ログイン中ユーザIDやロール判定に必要な最小情報は、ライブラリの session data に保存する
- `sessions` には `users.id` への外部キーを置かず、SCS PostgreSQL store の標準形に合わせる
- logout、session token renewal、期限切れ cleanup はセッションライブラリの責務に寄せる

この方針では、`sessions` は業務テーブルではなくライブラリ管理テーブルとして扱う。ユーザー別のセッション一覧、全端末ログアウト、ログイン履歴、監査ログが必要になった場合は、`sessions` を直接拡張せず、別テーブルまたは custom session store を検討する。

### 3.2 Finance残高概要

- `finance_accounts` は共有 `users` を所有者として参照し、Finance専用の口座表示情報と残高snapshotを保持する
- `finance_transactions` は `user_id` と `account_id` の複合外部キーにより、取引所有者と口座所有者の不一致をDBでも拒否する
- `finance_transaction_categories` はFinance内の表示用カテゴリーmasterとする
- 金額はすべてminor unitの `bigint` とし、浮動小数点型を使わない
- Finance APIでは `bigint` と `SUM(bigint)` の精度をJavaScript境界まで維持するため、minor unitを10進文字列として返す
- 通貨はISO 4217の大文字3文字を保存する
- 口座番号は完全値を保持せず、画面表示用の末尾最大4文字だけを `mask` に保存する
- API公開IDにはUUIDの `public_id` を使い、内部bigint IDを公開しない
- summary集計は有効口座を通貨別に集計し、異なる通貨を単純加算しない

### 3.3 Migration

- migrationは `backend/internal/platform/postgres/migrations/` の番号付きSQLを正とする
- 適用済みversionとchecksumは共有基盤テーブル `schema_migrations` で管理する
- 既存ローカル環境へは `CREATE TABLE IF NOT EXISTS` を含む初期migrationを安全に適用する
- 現段階ではローカル学習環境との互換性のためAPI起動時に適用するが、AWS環境ではapplication起動とmigration jobを分離する
- 適用済みmigrationの内容は変更せず、schema変更は新しいversionを追加する

## 4. Index方針

- `finance_accounts(user_id, status)` は所有者別の有効口座集計に使う
- `finance_transactions(user_id, COALESCE(posted_at, authorized_at) DESC, id DESC)` は、確定日時を優先するSummaryの最近の取引取得に使う
- `finance_transactions(account_id, authorized_at DESC, id DESC)` は口座詳細で使う
- index追加は実queryと `EXPLAIN` を確認し、未使用の先回りを避ける

## 5. 保留事項

- ファイルアップロード
- 非同期ジョブ
- 検索
- synthetic fixtureの投入方法
- account / transactionの保持・削除
- provider connectionと同期
- 複数通貨の換算
- ユーザー別セッション管理
- ログイン履歴と監査ログ
- remember me
- CSRF / CORS / Cookie 属性の確定

これらは必要になった時点で、アプリ設計と AWS 構成への影響を見ながら追加する。

## 6. 更新履歴

| 日付 | 内容 |
| --- | --- |
| 2026-08-11 | 認証・sessionの初期設計を記載 |
| 2026-08-11 | Finance残高概要の3テーブル、命名規則、versioned migration方針を追加 |
| 2026-08-11 | Finance金額API契約とSummary query用expression indexを更新 |
