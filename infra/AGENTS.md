# devlab-board infra

このディレクトリは、`devlab-board` の Terraform 実装と AWS インフラ管理を扱う。

## 作業前の確認順

1. `../AGENTS.md`
2. `../codex-workflow/README.md`
3. `../codex-workflow/AGENTS.md`
4. `README.md`

必要に応じて次も参照する。

- `../codex-workflow/rules/terraform.md`
- `../codex-workflow/rules/aws-delivery.md`
- `../codex-workflow/playbooks/infra-change.md`
- `../codex-workflow/roles/infra-reviewer.md`
- `../docs/`

## このディレクトリの既定方針

- まず `dev` 相当の小さい検証環境から実装する
- EC2 Auto Scaling / Serverless / ECS Fargate の構成を、比較できる単位で分ける
- 共通化は早すぎる抽象化を避け、各構成の違いを理解してから行う
- Terraform state、IAM、OIDC、ログ、ロールバック手順を構成ごとに明示する

## 現在の学習対象

- EC2 + ALB + Auto Scaling
- Serverless architecture
- ECS Fargate
- Terraform による AWS インフラの再現可能な管理
- GitHub Actions + OIDC を前提にした CI/CD
- CloudWatch を使った基本監視

## ガードレール

- 破壊的差分は理由を残す
- secret を直接 commit しない
- `prod` 前提の変更は project doc と整合を取る
- 構成図や設計 doc に影響がある変更は同じ変更で追従する
- コミットする `AGENTS.md`、README、docs では、参照パスに絶対パスを使わず相対パスを使う
