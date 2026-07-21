---
name: terraform-aws
description: Terraform で AWS 構成、module、env、IAM、配信手順を追加・変更するときに使う。
---

# Terraform AWS Skill

## 事前確認

- `infra/AGENTS.md` と `infra/README.md` を読む。
- 対象構成が EC2 Auto Scaling、Serverless、ECS Fargate のどれかを確認する。
- env、module、provider、backend、state、region、account を確認する。
- IAM、network、secret、public exposure、cost impact を確認する。

## 実装方針

- env 固有値と reusable module の境界を分ける。
- variable と output には `description` を付ける。
- IAM は least privilege を優先し、runtime role と deploy role を混ぜない。
- destructive change では理由と rollback path を残す。
- Terraform で作る値を frontend build に渡す場合、public config だけに限定する。

## 検証

- `terraform fmt -check -recursive` を実行する。
- 対象 env で `terraform validate` を実行する。
- AWS 認証と backend 初期化が可能なら `terraform plan` を確認する。
- apply はユーザーが明示した場合だけ実行する。
