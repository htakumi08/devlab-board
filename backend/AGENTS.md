# devlab-board backend

このディレクトリは、`devlab-board` の Go バックエンド実装を扱う。

## 作業前の確認順

1. `../AGENTS.md`
2. `../codex-workflow/README.md`
3. `../codex-workflow/AGENTS.md`
4. `README.md`

必要に応じて次も参照する。

- `../docs/app-service.md`
- `../docs/db-design.md`
- `../docs/db-design.dbml`
- `../infra/README.md`
- `../infra/AGENTS.md`
- `../codex-workflow/rules/backend-go.md`
- `../codex-workflow/rules/testing.md`
- `../codex-workflow/playbooks/feature.md`
- `../codex-workflow/playbooks/verify.md`

## このディレクトリの既定方針

- 1 endpoint / 1 use case 単位で小さく進める
- `cmd/` は起動責務だけに寄せる
- 業務ロジックは、必要になった時点で `internal/` の機能単位へ分ける
- DB, logging, S3 などの基盤処理は、必要になった時点で `internal/platform/` に寄せる
- API 実装は `docs/app-service.md` と `docs/db-design.md` の事実を優先する

## 現在の学習対象

- Go による HTTP API の基本実装
- ダッシュボード機能ごとの小さな API 設計
- PostgreSQL を使った機能単位のデータ管理
- EC2 / Serverless / ECS Fargate へ載せ替えやすい境界設計
- ALB ヘルスチェックと readiness の扱い

## ガードレール

- ORM を先に増やすより、まずは DB 契約を理解できる実装を優先する
- 認証やセッションは必要になってから追加する
- 実験機能は小さく閉じ、共通基盤を過剰に先回りしない
- 運用やデプロイに影響する変更では `codex-workflow/rules/aws-delivery.md` と `infra` 側も確認する
- コミットする `AGENTS.md`、README、docs では、参照パスに絶対パスを使わず相対パスを使う
