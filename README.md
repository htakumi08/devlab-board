# devlab-board

`devlab-board` は、Go と React で小さな機能を試すための開発実験ダッシュボードである。

アプリ本体は、サイドバーから実験機能を切り替えるダッシュボードとして育てる。並行して、同じアプリを題材に AWS の複数構成を比較する。

ローカル環境ユーザ
- test@test.com, Password01

## 現在の段階

- `backend/`: Go 標準ライブラリだけの最小 HTTP API
- `frontend/`: Vite + React の最小画面
- `infra/`: AWS / Terraform 学習用の置き場所
- `codex-workflow/`: Codex が自立的に計画、実装、検証、docs 整備を行うための運用ルール

## 学習テーマ

- React のダッシュボード UI
- Go の HTTP API
- PostgreSQL を使う機能実装
- Docker Compose によるローカル開発
- EC2 Auto Scaling 構成
- Serverless 構成
- ECS Fargate 構成
- GitHub Actions による CI/CD

旧アプリ実装は一度削除し、ここから小さい単位で作り直す。

## Docker Compose 起動後のアクセス先

`.env.example` をもとに `.env` を用意し、`docker compose up` で起動した場合のローカル接続先は次のとおり。

| Service | 接続先 | 用途 |
| --- | --- | --- |
| Frontend | <http://localhost:30101> | ブラウザで開く画面 |
| Backend API | <http://localhost:30102> | Go HTTP API |
| PostgreSQL | `localhost:30103` | ホスト側のDBクライアントから接続する場合 |

通常、アプリを確認するときは <http://localhost:30101> をブラウザで開く。

## Codex workflow

Codex で作業するときは、まず `AGENTS.md` と `codex-workflow/README.md` を確認する。

`codex-workflow/` には、Go backend、React frontend、Terraform/AWS、docs 整備、サブエージェント分担のための rule、playbook、role、skill を置く。
