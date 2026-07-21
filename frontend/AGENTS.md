# devlab-board frontend

このディレクトリは、`devlab-board` の React フロントエンド実装を扱う。

## 作業前の確認順

1. `../AGENTS.md`
2. `../codex-workflow/README.md`
3. `../codex-workflow/AGENTS.md`
4. `README.md`

必要に応じて次も参照する。

- `../docs/app-service.md`
- `../docs/db-design.md`
- `../docs/db-design.dbml`
- `../codex-workflow/rules/frontend-react.md`
- `../codex-workflow/rules/testing.md`
- `../codex-workflow/playbooks/feature.md`
- `../codex-workflow/playbooks/verify.md`

## このディレクトリの既定方針

- 1画面 / 1機能ずつ小さく進める
- 先に UI を作り切るより、API 契約と縦に通す
- Node 系の操作は `yarn` で統一する
- 実験機能ごとの責務は、必要になった時点で `features/` に寄せる
- 共通処理は `lib/` と `components/` に分ける
- ルーティングや画面の入口は `routes/` と `app/` に整理する

## 現在の学習対象

- サイドバーを持つダッシュボード画面
- 小さな実験機能を追加できる画面導線
- API と縦に接続する最小 UI
- S3 + CloudFront 配信を前提にした静的フロント構成

## ガードレール

- UI は学習の邪魔になる過剰な作り込みを避ける
- API の URL やレスポンス形式を画面ごとにバラさせない
- バックエンド未実装なら、仮置きは最小限にする
- `package.json`, `yarn.lock`, `vite.config.ts`, `.env*` などの設定ファイルでは、`rm` などの一括削除をなるべく避け、必要箇所を個別に更新する
- デプロイや環境変数に影響する変更では `codex-workflow/rules/aws-delivery.md` も確認する
- コミットする `AGENTS.md`、README、docs では、参照パスに絶対パスを使わず相対パスを使う
