# Backend

このディレクトリは、Go を一つずつ学びながら育てるための最小バックエンドである。

## 現在の構成

```text
backend/
  AGENTS.md
  Dockerfile
  README.md
  go.mod
  cmd/
    api/
      main.go
    finance-seed/
      main.go
    finance-smoke/
      main.go
  internal/
    app/
      app.go
      bootstrap.go
      config.go
      doc.go
      user_store.go
      validation.go
    finance/
      account.go
      account_repository.go
      summary.go
      postgres_repository.go
      transaction.go
      transaction_repository.go
    financeseed/
      seed.go
    financesmoke/
      smoke.go
    platform/postgres/
      migrate.go
      migrations/
```

## 起動

```bash
go run ./cmd/api
```

Docker Composeを使う場合は、リポジトリルートで実行する。

```bash
docker compose up -d --build backend
```

## gRPC

backendプロセスは、HTTPの8080番とgRPCの50051番を同時に待ち受ける。
ローカルのDocker Composeでは、gRPCを`127.0.0.1:30104`へ公開する。

Docker環境へ固定バージョンの`grpcurl`を導入しているため、次の順番で確認できる。

```bash
# Reflectionを使って、公開されているサービスを一覧表示する。
docker compose exec backend \
  grpcurl -plaintext 127.0.0.1:50051 list

# GreetingServiceのRPC定義を表示する。
docker compose exec backend \
  grpcurl -plaintext 127.0.0.1:50051 \
  describe grpclab.v1.GreetingService

# Hello RPCへJSON形式のリクエストを送る。
docker compose exec backend \
  grpcurl -plaintext \
  -d '{"name":"Taro"}' \
  127.0.0.1:50051 \
  grpclab.v1.GreetingService/Hello
```

`-plaintext`は、ローカル学習環境でTLSを使わず接続する指定である。本番環境では使用しない。

Goで実装した学習用クライアントからも、同じRPCを確認できる。`backend/`で次を実行する。

```bash
go run ./cmd/grpc-client \
  -target 127.0.0.1:30104 \
  -name Taro \
  -timeout 3s
```

このクライアントはローカルのplaintext接続専用であり、本番向けのTLS設定は含まない。

## エンドポイント

- `GET /`: バックエンドの起動確認
- `GET /healthz`: ヘルスチェック
- `POST /api/auth/register`: email / password で登録し、session を作成する
- `POST /api/auth/login`: email / password でログインし、session を作成する
- `POST /api/auth/logout`: session を破棄する
- `GET /api/auth/me`: ログイン中ユーザを返す
- `GET /api/dashboard`: session 認証済み API の疎通確認
- `GET /api/user-agent`: request の User-Agent を返す
- `GET /api/finance/summary`: 現在ユーザーの通貨別残高と最近の取引を返す
- `GET /api/finance/accounts`: 現在ユーザーの口座一覧を返す
- `GET /api/finance/accounts/{accountId}`: 現在ユーザーの口座詳細と最近の取引5件を返す
- `GET /api/finance/transactions`: 現在ユーザーの取引をfilter・sort対応のcursor paginationで返す
- `GET /api/finance/categories`: 取引filter用のカテゴリーmasterを表示順で返す

取引一覧は、単一値の`account_id`、`date_from`、`date_to`、`category`、`direction`、`status`、`sort`、`cursor`、`limit`を受け付ける。日付はUTC暦日の`YYYY-MM-DD`、`sort`は`newest`または`oldest`とし、filter・sortを変更するときはcursorを破棄して先頭ページから取得する。

`/api/auth/register` と `/api/auth/login` 以外の `/api/` endpoint は session 認証を必要とする。

## Database migration

- 番号付きSQLは `internal/platform/postgres/migrations/` に置く
- `schema_migrations` にversion、file name、checksum、適用日時を保存する
- 適用済みSQLは変更せず、新しい番号のmigrationを追加する
- localでは互換性のためAPI起動時に適用する
- AWS環境ではapplication起動とmigration jobを分離する
- Finance専用テーブルは `finance_` プレフィックスへ統一する

DB定義は [`../docs/db-design.dbml`](../docs/db-design.dbml)、Finance API契約は [`../docs/api/finance-summary.md`](../docs/api/finance-summary.md)、[`../docs/api/finance-accounts.md`](../docs/api/finance-accounts.md)、[`../docs/api/finance-transactions.md`](../docs/api/finance-transactions.md) を参照する。
決定的synthetic fixtureの投入・削除と2ユーザー共通smokeは [`../docs/runbooks/finance-mvp-local-verification.md`](../docs/runbooks/finance-mvp-local-verification.md) を参照する。

## 認証 / セッション

- session 管理は SCS を使う
- local は `SESSION_STORE=postgres` で PostgreSQL の `sessions` テーブルを使う
- ECS Fargate 本番候補では `SESSION_STORE=redis` で Redis store へ切り替える
- password は bcrypt hash として保存する
- email は正規化した小文字で保存し、DB の unique 制約で重複を拒否する
- session lifetime は一旦 SCS の標準設定に従う

## 方針

- まずは標準ライブラリだけで HTTP サーバの基本を理解する
- 学習した単位ごとに、小さい endpoint や package を追加する
- PostgreSQL や AWS 連携は、必要になったタイミングで段階的に戻す
