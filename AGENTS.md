# devlab-board エージェントガイド

## プロジェクト

`devlab-board` は、Go と React を一つずつ学びながら、小さな実験機能と AWS 構成比較を育てる開発実験ダッシュボードです。

予定している技術スタック:

- フロントエンド: Vite + React + TypeScript
- バックエンド: Go HTTP API
- ローカル開発: Docker Compose
- インフラ: Terraform + AWS
- AWS 構成比較: EC2 Auto Scaling、Serverless、ECS Fargate
- ドキュメント: `docs/` と `codex-workflow/`

DB 方針は PostgreSQL を正とします。ローカル開発の `docker-compose.yml` も PostgreSQL service に揃えます。

## 作業前の確認順

1. `AGENTS.md`
2. `codex-workflow/README.md`
3. `codex-workflow/AGENTS.md`
4. 変更対象ディレクトリの `AGENTS.md`
5. 関連する `docs/`
6. 関連する `codex-workflow/rules/` と `codex-workflow/playbooks/`

## 運用ルール

- 構造を変更する前に、既存ファイル、README、docs、実コードを確認する。
- workflow ファイルは短く、devlab-board 固有に保つ。一般論や使わない技術ルールを増やさない。
- 繰り返し発生する判断、検証手順、docs 整備方針は、安定した段階で `codex-workflow/` へ反映する。
- 大きな先回りより、1 機能 / 1 endpoint / 1 画面の小さな縦切りを優先する。
- ユーザー向け挙動、API contract、DB schema、AWS 構成、CI/CD、運用手順は設計決定として扱い、安定したら docs に残す。
- docs を作成・更新するときは `codex-workflow/playbooks/documentation.md` と `codex-workflow/rules/documentation.md` を使う。
- 生成物、local dependency、credentials、Terraform state、個人環境のログは commit しない。
- サブエージェントを使う場合は、`codex-workflow/roles/` で責務と触ってよい範囲を明確にする。

## 想定構成

- `backend/`: Go HTTP API
- `frontend/`: Vite + React + TypeScript app
- `infra/`: Terraform modules and environment definitions
- `docs/`: application design、DB design、architecture、runbooks、implementation notes
- `codex-workflow/`: Codex rules、playbooks、roles、compact skills

## バックエンド制約

- 現段階では Go 標準ライブラリ中心で進める。
- `cmd/` は起動責務に寄せ、handler、use case、DB、外部連携は必要になった時点で `internal/` に分ける。
- endpoint を増やすときは request、response、status code、error shape を小さく定義する。
- DB、logging、S3、queue などの基盤処理は、直接 handler に散らさず `internal/platform/` などへ寄せる。
- secrets、raw credentials、不要な個人情報、sensitive request payload を log に出さない。

## フロントエンド制約

- Node 系の操作は `frontend/README.md` に合わせて `yarn` を使う。
- API base URL と response shape は、画面ごとに散らさず境界をまとめる。
- loading、empty、error、offline、permission の状態を必要な範囲で扱う。
- S3 + CloudFront 配信を想定し、build-time に secret を bundle しない。
- 学習段階の UI は過剰に作り込まず、繰り返し使う要素だけ component 化する。

## インフラ制約

- Terraform 変更では、環境差分、state、IAM、cost、rollback を確認する。
- `terraform apply` は明示的に依頼された deploy/release 作業として扱う。
- EC2 Auto Scaling、Serverless、ECS Fargate は比較できる単位で分け、早すぎる共通化を避ける。
- GitHub Actions から AWS へ接続する場合は OIDC を優先し、長期 access key を置かない。

## 検証

作業を返す前に、関係する最小限の検証ループを回す。

- バックエンド: `go test ./...`、必要に応じて `gofmt` 確認。
- フロントエンド: script が存在する場合は `yarn build`。lint/test が追加されたら対象 script も実行する。
- インフラ: Terraform 変更では `terraform fmt -check -recursive`、対象 env の `terraform validate`、可能なら `terraform plan`。
- docs/workflow-only changes: links、相対 path、`git status --short` を確認する。

軽量 workflow map は `codex-workflow/README.md` を参照してください。
