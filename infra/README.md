# Terraform Structure

このディレクトリは、`devlab-board` の AWS インフラ実験を Terraform で管理するための骨格である。

現在は EC2 Auto Scaling 構成を中心にした骨格が残っている。今後、Serverless と ECS Fargate の構成を同じアプリの別デプロイレーンとして追加する。

`codex-workflow` の構成思想に合わせて、永続的な構造はディレクトリで分ける。

## 関連ドキュメント

- `../AGENTS.md`
- `../codex-workflow/README.md`
- `../codex-workflow/AGENTS.md`
- `./AGENTS.md`

## ディレクトリ構成

```text
infra/
  modules/
    network/
    security/
    iam/
    s3/
    cloudfront/
    alb/
    compute/
    rds/
    observability/
  envs/
    dev/
    stg/
    prod/
```

今後の構成案:

```text
infra/
  ec2-asg/
    terraform/
  serverless/
    terraform/
    lambda/
  ecs-fargate/
    terraform/
```

## module の責務

- `network`
  VPC, Subnet, Route Table, IGW, NAT などのネットワーク層
- `security`
  Security Group などの通信制御
- `iam`
  EC2 instance profile, application role などの権限
- `s3`
  静的配信やアップロード保存先となる S3 バケット
- `cloudfront`
  CloudFront distribution, OAC, origin 設定
- `alb`
  ALB, target group, listener
- `compute`
  Launch Template, Auto Scaling Group, EC2 関連
- `rds`
  DB subnet group, parameter group, RDS instance
- `observability`
  CloudWatch logs, alarms, dashboard, SNS など

## env の責務

- `dev`
  日常開発、検証用
- `stg`
  リリース前検証用
- `prod`
  本番用

各環境は、

- `versions.tf`
- `providers.tf`
- `variables.tf`
- `locals.tf`
- `main.tf`
- `outputs.tf`
- `backend.tf`
- `backend.hcl.example`
- `terraform.tfvars.example`

を入口として持つ。

## この段階での方針

今回はプロジェクト名と学習テーマを `devlab-board` に寄せ直す段階なので、module の中身はまだ空の骨格である。

次にやること:

1. EC2 Auto Scaling 構成を `dev` で最小実装する
2. GitHub Actions + OIDC のデプロイ導線を作る
3. Serverless 構成を追加して比較する
4. ECS Fargate 構成を追加して比較する
