# App Service Design

## 1. 目的

`devlab-board` は、小さな実装アイデアや AWS 構成を試すための開発実験ダッシュボードである。

アプリ本体は、サイドバーから実験機能を切り替える 1 つのダッシュボードとして育てる。各機能は大きく作り込まず、React / Go / PostgreSQL / Docker / AWS の学習単位として追加する。

## 2. サービス方針

- 1 機能ずつ小さく実装する
- フロントエンドとバックエンドを縦に通して学ぶ
- PostgreSQL は必要になった機能から使う
- 同じアプリを EC2 Auto Scaling / Serverless / ECS Fargate に載せ替えて比較する
- GitHub Actions による CI/CD も構成ごとに比較する

## 3. 初期 MVP

初期 MVP は、アプリの土台だけに絞る。

- サイドバー付きのダッシュボード画面
- 実験機能を 1 つ表示するメイン領域
- Go backend の `GET /healthz`
- Go backend の簡単な JSON API
- Docker Compose によるローカル起動

## 4. 実験機能の候補

- API 疎通確認パネル
- メモや学習ログの CRUD
- PostgreSQL 接続確認
- ファイルアップロード検証
- 非同期ジョブ検証
- CloudWatch logs / metrics 確認
- 認証とセッション検証

## 5. AWS 構成比較

同じアプリを題材に、次の 3 パターンを比較する。

- EC2 Auto Scaling: ALB, EC2, ASG, RDS を中心にした構成
- Serverless: API Gateway, Lambda, managed database / storage を中心にした構成
- ECS Fargate: ALB, ECS service, task definition, container image を中心にした構成

比較観点:

- デプロイ手順
- ネットワーク構成
- IAM / OIDC
- ログと監視
- スケール方式
- ロールバック
- コスト
- 運用負荷

## 6. 設計メモ

古いアプリ設計は引き継がない。必要な機能だけを、学習テーマに合わせて小さく戻す。
