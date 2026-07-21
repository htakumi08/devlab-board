---
name: infra-change
description: Terraform と AWS 構成を追加・変更・比較するときに使う。
---

# インフラ変更プレイブック

1. 対象構成が EC2 Auto Scaling、Serverless、ECS Fargate のどれかを確認する。
2. 変更対象 env、module、state、backend、provider を確認する。
3. IAM、network、public exposure、secret、cost、rollback の影響を整理する。
4. Terraform module の責務と env 固有値の境界を確認する。
5. `terraform fmt -check -recursive` と対象 env の `terraform validate` を実行する。
6. AWS 認証と backend 初期化が可能なら `terraform plan` を確認する。
7. docs、diagram、runbook に影響がある場合は同じ変更で更新する。

## apply 前の確認

- ユーザーが apply/deploy を明示的に依頼している。
- plan の destroy / replace / public exposure / IAM expansion を確認済み。
- rollback path と smoke check がある。
- state lock と対象 account / region / env を確認済み。
