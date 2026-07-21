# AWS 配信ルール

## 構成比較

devlab-board では、同じアプリを次の 3 つの AWS 構成で比較できるようにする。

- EC2 Auto Scaling: ALB、EC2、ASG、RDS を中心にした構成。
- Serverless: API Gateway、Lambda、managed database / storage を中心にした構成。
- ECS Fargate: ALB、ECS service、task definition、container image を中心にした構成。

構成比較では、deploy 手順、network、IAM/OIDC、logs、monitoring、scale、rollback、cost、運用負荷を確認する。

## フロントエンド

- S3 は build 済み static assets のみをホストする。
- CloudFront を公開入口にする。
- deployment credentials は least privilege にする。
- 環境は bucket/distribution、または明確に文書化した prefix で分離する。
- deploy 後は必要な path だけを invalidate する。それ以外は immutable な hashed assets を優先する。

## バックエンド

- Go backend は、構成ごとの runtime 差分を意識して entry point を薄く保つ。
- EC2/ECS では health check、graceful shutdown、port、environment variables を明示する。
- Serverless では adapter、cold start、timeout、IAM、logs、request/response mapping を明示する。
- DB 接続、migration、secret source は構成ごとに文書化する。

## 運用

- deploy command は自動化する前に文書化する。
- AWS credentials は絶対に commit しない。
- release が始まったら、CloudFront/S3/API/DB errors 用の短い runbook を用意する。
- rollback path と smoke check を release 手順に含める。
