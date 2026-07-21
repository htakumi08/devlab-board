# Terraform ルール

## 構成

- Terraform は `infra/` 配下に置く。
- 環境は `infra/envs/{dev,stg,prod}/` に分ける。
- 共通部品は `infra/modules/` に置く。
- EC2 Auto Scaling、Serverless、ECS Fargate は、比較しやすい単位で分ける。
- module 共通化は、構成差分を理解してから行う。

## State / Secrets

- `terraform.tfstate`、秘密値を含む `*.tfvars`、AWS credentials は commit しない。
- state は remote backend と lock を使う方針にする。
- GitHub Actions は OIDC で AWS Role を引き受ける。長期 access key を secrets に置かない。
- provider、backend、workspace/env directory の前提は README か runbook に残す。

## 検証

- Terraform 変更では `terraform fmt -check -recursive` を基本確認にする。
- 対象 env では `terraform init` 後に `terraform validate` を確認する。
- AWS 認証と backend 初期化が可能な環境では `terraform plan` まで確認する。
- `terraform apply` は deploy / release 作業として扱い、意図が明示されている場合だけ実行する。

## 設計判断

- IAM は service、deployment、runtime の責務で分け、least privilege を優先する。
- module を増やす前に、リソースの境界と再利用価値があるか確認する。
- EC2、Serverless、ECS の比較観点は docs に残す。
- destroy を伴う差分は、理由、影響、rollback path を明示する。

## コメント / Docstring

- module、variable、output には `description` を積極的に付け、人間が用途をすぐ判断できる状態にする。
- IAM policy、assume role policy、CloudFront cache behavior、ALB health check、autoscaling policy など、意図を読み違えやすい箇所は HCL comment で補足する。
- Terraform の式をそのまま言い換えるだけの comment は書かない。
