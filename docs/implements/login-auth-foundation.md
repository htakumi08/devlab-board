# Login Auth Foundation

## 1. 目的

`devlab-board` のログイン基盤を、Go `net/http` と SCS による server-side session 管理で実装する。

最初の実装では、ローカル開発は Docker Compose の PostgreSQL を session store として使う。将来 ECS Fargate で本番稼働させる場合は、環境変数で Redis session store へ切り替えられる構成にする。

## 2. 方針

- HTTP framework は追加せず、`net/http` を使う
- session 管理は SCS に委ねる
- session data は server-side store に保存し、ブラウザには session token cookie だけを渡す
- local は `SESSION_STORE=postgres` を使う
- production の ECS Fargate では `SESSION_STORE=redis` を候補にする
- `GET /healthz`、`POST /api/auth/register`、`POST /api/auth/login` は未認証で許可する
- その他の `/api/` は session 認証を必須にする
- session lifetime は一旦 SCS の標準設定に従う

## 3. DB

`users` はアプリケーションのログインユーザ本体を管理する。

```sql
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
```

PostgreSQL session store は SCS postgresstore の標準形に合わせる。

```sql
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  data BYTEA NOT NULL,
  expiry TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry);
```

`sessions` はライブラリ管理テーブルとして扱う。ユーザー別セッション一覧、全端末ログアウト、ログイン監査ログが必要になった場合は、`sessions` を直接拡張せず別テーブルを検討する。

## 4. 環境変数

| Name | local | production candidate | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | required | required | users と PostgreSQL session store |
| `SESSION_STORE` | `postgres` | `redis` | SCS session store selector |
| `REDIS_ADDR` | optional | required for redis | Redis endpoint |
| `REDIS_PASSWORD` | optional | optional | Redis password |
| `REDIS_DB` | optional | optional | Redis logical DB |
| `SESSION_COOKIE_SECURE` | `false` | `true` | Secure cookie flag |
| `FRONTEND_ORIGIN` | `http://localhost:30101` | CloudFront origin | CORS allow origin |

## 5. API

### `POST /api/auth/register`

Request:

```json
{
  "email": "user@example.com",
  "password": "Password1"
}
```

Validation:

- email は `net/mail` でメールアドレスとして parse できること
- email は小文字化して一意に扱う
- password は 8 文字以上
- password は英字と数字の両方を含む
- password は大文字英字を 1 文字以上含む
- 重複 email は `409 Conflict`

Response:

```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "user",
    "role": "user"
  }
}
```

登録成功後は session を作成し、そのままログイン状態にする。

### `POST /api/auth/login`

Request:

```json
{
  "email": "user@example.com",
  "password": "Password1"
}
```

認証成功後、SCS session token を更新し、session data に `user_id` を保存する。

### `POST /api/auth/logout`

session を破棄する。認証必須。

### `GET /api/auth/me`

現在の session から user を返す。認証必須。

### `GET /api/dashboard`

ログイン済み API の疎通確認用。認証必須。

## 6. Frontend

最初の UI は、ログインと登録を同一画面で切り替える。

- Login: email + password
- Register: email + password
- backend と同じ password 条件を画面上でも表示する
- API 呼び出しは `credentials: "include"` で Cookie を送受信する
- 未ログイン時は auth 画面、ログイン後は既存ダッシュボードを表示する

## 7. 検証

- backend: `go test ./...`
- frontend: `yarn build`
- docs: 相対 path と設計方針の整合確認

## 8. 実装メモ

追加した主なファイル:

- `backend/internal/app/app.go`: auth handler、session middleware、CORS、JSON response
- `backend/internal/app/bootstrap.go`: DB 接続、DDL 適用、SCS store 切り替え
- `backend/internal/app/user_store.go`: PostgreSQL user store と test 用 memory store
- `backend/internal/app/validation.go`: email / password validation
- `frontend/src/App.tsx`: login / register / authenticated dashboard
- `frontend/src/style.css`: auth UI と dashboard UI

起動時に `users` と `sessions` の `CREATE TABLE IF NOT EXISTS` を実行する。migration 専用ツールはまだ導入せず、認証基盤の最初の縦切りとしてアプリ起動時 DDL に留める。

## 9. 要確認・ヒアリング項目

- CSRF token をいつ導入するか
- 本番 Cookie domain をどうするか
- Redis を ElastiCache Serverless / node-based のどちらで持つか
- ユーザー別セッション一覧や全端末ログアウトを MVP に含めるか
- ログイン監査ログをどの粒度で持つか
