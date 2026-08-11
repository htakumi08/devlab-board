---
title: AWSコンソールでdevlab-boardをECS Fargateへデプロイする最小構成ハンズオン
status: draft
last_updated: 2026-07-27
audience: AWSコンソールからECS FargateへWebアプリを初めてデプロイする開発者
---

# AWSコンソールでdevlab-boardをECS Fargateへデプロイする最小構成ハンズオン

## 目次

1. [このハンズオンのゴール](#1-このハンズオンのゴール)
2. [実現できるか](#2-実現できるか)
3. [今回作る構成](#3-今回作る構成)
4. [本番構成との違い](#4-本番構成との違い)
5. [費用とセキュリティの注意](#5-費用とセキュリティの注意)
6. [事前準備](#6-事前準備)
7. [AWS Budgetsで通知を設定する](#7-aws-budgetsで通知を設定する)
8. [VPCとSubnetを作成する](#8-vpcとsubnetを作成する)
9. [Security Groupを作成する](#9-security-groupを作成する)
10. [RDS PostgreSQLを作成する](#10-rds-postgresqlを作成する)
11. [AWS用のfrontend imageを準備する](#11-aws用のfrontend-imageを準備する)
12. [ECR repositoryを作成してimageをpushする](#12-ecr-repositoryを作成してimageをpushする)
13. [DATABASE_URLをSecrets Managerへ保存する](#13-database_urlをsecrets-managerへ保存する)
14. [ECS task execution roleを作成する](#14-ecs-task-execution-roleを作成する)
15. [CloudWatch Logsのlog groupを作成する](#15-cloudwatch-logsのlog-groupを作成する)
16. [ECS ClusterとTask Definitionを作成する](#16-ecs-clusterとtask-definitionを作成する)
17. [ECS Serviceを作成する](#17-ecs-serviceを作成する)
18. [デプロイ結果を確認する](#18-デプロイ結果を確認する)
19. [更新とロールバックを試す](#19-更新とロールバックを試す)
20. [トラブルシューティング](#20-トラブルシューティング)
21. [AWSリソースを削除する](#21-awsリソースを削除する)
22. [次の発展](#22-次の発展)
23. [公式リファレンス](#23-公式リファレンス)
24. [更新履歴](#24-更新履歴)
25. [要確認・ヒアリング項目](#25-要確認ヒアリング項目)

## 1. このハンズオンのゴール

このハンズオンでは、AWSマネジメントコンソールを中心に、`devlab-board`をAWSへデプロイする。

最終的に次を確認する。

- React画面をブラウザで表示できる
- 新しい学習用ユーザーを登録できる
- Go APIへCookie付きでアクセスできる
- PostgreSQLへユーザーとsessionが保存される
- ログイン後のダッシュボードを表示できる
- User-Agent LabからGo APIの応答を確認できる
- frontendとbackendのログをCloudWatch Logsで確認できる
- 確認後に作成したAWSリソースを削除できる

このハンズオンは、既存の[`ecs-fargate-minimum-handson.md`](./ecs-fargate-minimum-handson.md)でApacheコンテナを動かした次の段階に位置付ける。

## 2. 実現できるか

実現できる。ただし、2026年7月27日時点の`devlab-board`を、そのまま既存の最小構成へ載せることはできない。

理由は次のとおり。

1. Go backendは起動時に`DATABASE_URL`を必須とし、PostgreSQLへ接続する
2. frontendのDockerfileはVite開発サーバー用で、静的ファイル配信用ではない
3. frontendはCookie sessionを使うため、frontendとbackendを別々のPublic IPで公開すると、CORSとCookieの扱いが複雑になる
4. backendだけをECSへ載せても、`devlab-board`の画面全体は確認できない

そこで今回は、1つのFargate Taskに次の2コンテナを同居させる。

- frontend: NginxでReactのbuild成果物を配信する
- backend: Go HTTP APIを実行する

Nginxは`/api/*`を同じTask内の`localhost:8080`へ転送する。Fargateの`awsvpc` network modeでは、同じTaskに属するコンテナ同士が`localhost`で通信できる。

AWSコンソールだけでローカルのDocker imageをECRへアップロードすることはできないため、imageのbuildとpushに限ってDockerとAWS CLIを使用する。それ以外はAWSコンソールを中心に操作する。

## 3. 今回作る構成

```mermaid
flowchart LR
  User["学習者のブラウザ"]
  Docker["学習者のPC<br/>Docker / AWS CLI"]

  subgraph AWS["AWS: ap-northeast-1"]
    ECRF["ECR<br/>frontend image"]
    ECRB["ECR<br/>backend image"]
    Secrets["Secrets Manager<br/>DATABASE_URL"]
    Logs["CloudWatch Logs"]

    subgraph VPC["VPC 10.20.0.0/16"]
      IGW["Internet Gateway"]

      subgraph Public["Public Subnet<br/>2 AZ"]
        SGTask["ECS Security Group<br/>学習者IP/32 → TCP 80"]

        subgraph Task["ECS Fargate Task<br/>Public IPあり"]
          Frontend["Nginx + React<br/>TCP 80"]
          Backend["Go API<br/>TCP 8080"]
          Frontend -->|"/api/* → localhost:8080"| Backend
        end
      end

      subgraph Private["Private Subnet<br/>2 AZ / NATなし"]
        SGRDS["RDS Security Group<br/>ECS SG → TCP 5432"]
        RDS["RDS PostgreSQL<br/>Single-AZ"]
      end
    end
  end

  User -->|"HTTP :80"| IGW --> SGTask --> Frontend
  Backend -->|"PostgreSQL :5432"| SGRDS --> RDS
  Docker --> ECRF
  Docker --> ECRB
  ECRF --> Task
  ECRB --> Task
  Secrets --> Backend
  Task --> Logs
```

### 3.1 使用するAWSサービス

| サービス | 用途 |
| --- | --- |
| VPC | ECSとRDSを配置するネットワーク |
| Internet Gateway | Public Subnet上のFargate Taskへ接続する |
| Security Group | HTTPとDB接続の許可範囲を制限する |
| ECR | frontendとbackendのcontainer imageを保存する |
| ECS Fargate | frontendとbackendを1つのTaskで実行する |
| RDS for PostgreSQL | usersとsessionsを保存する |
| Secrets Manager | `DATABASE_URL`をTaskへ安全に注入する |
| IAM | ECS agentへimage取得、ログ送信、secret取得を許可する |
| CloudWatch Logs | frontendとbackendのcontainer logを確認する |
| AWS Budgets | 利用料金の通知を受け取る |

### 3.2 Resource name

このハンズオンでは、次の名前へ揃える。

| Resource | Name |
| --- | --- |
| VPC | `devlab-board-handson-vpc` |
| ECS Security Group | `devlab-board-handson-ecs-sg` |
| RDS Security Group | `devlab-board-handson-rds-sg` |
| DB subnet group | `devlab-board-handson-db-subnet-group` |
| RDS DB instance | `devlab-board-handson-db` |
| RDS database name | `devlab` |
| RDS master user | `devlab_admin` |
| frontend ECR | `devlab-board-handson/frontend` |
| backend ECR | `devlab-board-handson/backend` |
| Secret | `devlab-board-handson/database-url` |
| IAM role | `devlab-board-handson-ecs-execution-role` |
| Log group | `/ecs/devlab-board-handson` |
| ECS Cluster | `devlab-board-handson-cluster` |
| Task Definition family | `devlab-board-handson-task` |
| ECS Service | `devlab-board-handson-service` |

## 4. 本番構成との違い

今回の構成は、学習対象を減らすための一時的な最小構成である。

| 今回 | 本番候補 |
| --- | --- |
| Fargate TaskへPublic IPを付与 | Private Subnetへ配置 |
| Public IPへ直接HTTP接続 | ALB + ACMでHTTPS化 |
| frontendとbackendを同じTaskで実行 | 独立した配信・スケール単位に分ける |
| ReactをNginxコンテナから配信 | S3 + CloudFrontから配信 |
| ECS Serviceのdesired countは1 | 複数AZで2以上を検討 |
| RDSはSingle-AZ | Multi-AZ、backup、監視を設計 |
| RDS master userでアプリを起動 | migration用とアプリ用のDB userを分ける |
| HTTPのためSecure cookieを無効化 | HTTPSでSecure cookieを有効化 |
| `sslmode=require` | 証明書検証を含む接続を検討 |

同一Taskの複数コンテナは別々にスケールできない。frontendとbackendで負荷特性が異なる本番環境では、この構成をそのまま採用しない。

## 5. 費用とセキュリティの注意

### 5.1 費用

このハンズオンは無料とは限らない。少なくとも次の料金が発生し得る。

- FargateのvCPU / memory利用時間
- Fargate Taskへ付与するPublic IPv4 address
- RDS PostgreSQLのDB instance、storage、backup
- Secrets Managerのsecret
- ECRのimage storage
- CloudWatch Logsの取り込みと保存
- データ転送

ALBとNAT Gatewayは作らないため、それらの時間課金は発生しない。

料金、無料利用枠、creditの条件はaccount作成時期やRegionによって異なる。作成前にAWS Pricing Calculatorと各サービスの料金ページを確認する。

AWS Budgetsは通知機能であり、利用を強制停止する機能ではない。通知には時間差があるため、削除手順の代わりにはならない。

### 5.2 セキュリティ

- AWS root userでは操作しない
- 個人の学習用account、または管理者が許可したsandbox accountを使う
- Regionは`ap-northeast-1`へ統一する
- ECSのTCP 80は、自分のPublic IPv4 `/32`だけ許可する
- TCP 8080、50051、5432をインターネットへ公開しない
- RDSのPublic accessは`No`にする
- `DATABASE_URL`をTask Definitionの平文Environment variablesへ入力しない
- `.env`、AWS credentials、secret ARN以外のsecret値をGitへcommitしない
- 実在する個人メールアドレスや普段使うpasswordをテストに使わない
- このアプリはCSRF対策が未導入の学習段階であるため、第三者へ一般公開しない
- HTTPで確認するため`SESSION_COOKIE_SECURE=false`を使うが、本番では許容しない

## 6. 事前準備

### 6.1 必要なもの

- `devlab-board` repository
- Docker EngineまたはDocker Desktop
- Docker Buildx
- AWS CLI v2
- AWS Management Consoleへサインインできる学習用role
- ECRへimageをpushできる権限
- VPC、EC2 Security Group、RDS、ECS、IAM、Secrets Manager、CloudWatch Logs、Budgetsを操作できる権限

権限不足の解決として、普段使うaccountへ無条件に`AdministratorAccess`を追加しない。表示された`AccessDenied`のactionを確認し、学習用roleの管理者へ必要な範囲だけ依頼する。

### 6.2 対象accountとRegionを確認する

AWSコンソール右上で次を確認する。

- Account IDまたはaccount alias
- 使用中のrole
- Regionが`Asia Pacific (Tokyo) ap-northeast-1`

CLIでも確認する。

```bash
aws sts get-caller-identity
aws configure get region
```

期待するaccountと異なる場合は、作業を始めない。

### 6.3 ローカル動作を確認する

AWSへ載せる前に、repository rootでローカル環境を確認する。

```bash
docker compose up -d --build
docker compose ps
```

ブラウザで<http://localhost:30101>を開き、登録、ログイン、ダッシュボード表示を確認する。

確認後に停止する。

```bash
docker compose down
```

DBデータも削除する場合だけ、影響を理解したうえで`docker compose down --volumes`を使う。

## 7. AWS Budgetsで通知を設定する

1. AWSコンソールで`Billing and Cost Management`を開く
2. 左メニューから`Budgets`を選ぶ
3. `Create budget`を押す
4. `Use a template (simplified)`を選ぶ
5. 初めての学習accountなら`Zero spend budget`、既に利用中なら`Monthly cost budget`を選ぶ
6. 通知先メールアドレスを入力する
7. 内容を確認して作成する
8. 通知先に確認操作が必要な場合は完了させる

Budgetを作っても、課金が自動停止されるわけではない。

## 8. VPCとSubnetを作成する

1. AWSコンソールで`VPC`を開く
2. `Create VPC`を押す
3. `Resources to create`で`VPC and more`を選ぶ
4. 次の値を設定する

| Item | Value |
| --- | --- |
| Name tag auto-generation | `devlab-board-handson` |
| IPv4 CIDR block | `10.20.0.0/16` |
| IPv6 CIDR block | なし |
| Tenancy | `Default` |
| Number of Availability Zones | `2` |
| Number of public subnets | `2` |
| Number of private subnets | `2` |
| NAT gateways | `None` |
| VPC endpoints | `None` |
| DNS hostnames | 有効 |
| DNS resolution | 有効 |

5. Previewで次が作成対象に含まれることを確認する
   - VPC 1個
   - Public Subnet 2個
   - Private Subnet 2個
   - Public Route Table
   - Private Route Table
   - Internet Gateway
   - NAT Gatewayは0個
6. `Create VPC`を押す
7. 作成後に`View VPC`を押す
8. Resource mapを開き、Public Subnetのroute tableだけがInternet Gatewayへの`0.0.0.0/0` routeを持つことを確認する

Private SubnetはRDS用であり、今回インターネットへ接続しないためNAT Gatewayを作らない。

## 9. Security Groupを作成する

### 9.1 ECS用Security Group

1. VPCコンソールの`Security groups`を開く
2. `Create security group`を押す
3. 次を設定する

| Item | Value |
| --- | --- |
| Security group name | `devlab-board-handson-ecs-sg` |
| Description | `HTTP access to devlab-board handson ECS task` |
| VPC | `devlab-board-handson-vpc` |

4. Inbound rulesへ次を追加する

| Type | Protocol | Port | Source |
| --- | --- | --- | --- |
| HTTP | TCP | 80 | `My IP`で自分のPublic IPv4 `/32` |

5. Outbound rulesは初回ハンズオンでは`All traffic / 0.0.0.0/0`を維持する
6. `Create security group`を押す
7. Security Group IDをメモする

`0.0.0.0/0`からのHTTP inboundは設定しない。VPN、proxy、テザリングを切り替えるとPublic IPが変わる可能性がある。

### 9.2 RDS用Security Group

1. もう一度`Create security group`を押す
2. 次を設定する

| Item | Value |
| --- | --- |
| Security group name | `devlab-board-handson-rds-sg` |
| Description | `PostgreSQL access from devlab-board ECS task` |
| VPC | `devlab-board-handson-vpc` |

3. Inbound rulesへ次を追加する

| Type | Protocol | Port | Source |
| --- | --- | --- | --- |
| PostgreSQL | TCP | 5432 | `devlab-board-handson-ecs-sg` |

4. SourceはCIDRではなくECS用Security Groupを選ぶ
5. `Create security group`を押す
6. Security Group IDをメモする

RDS用Security Groupへ自分のPublic IPや`0.0.0.0/0`を追加しない。

## 10. RDS PostgreSQLを作成する

### 10.1 DB subnet group

1. AWSコンソールで`RDS`を開く
2. 左メニューから`Subnet groups`を選ぶ
3. `Create DB subnet group`を押す
4. 次を設定する

| Item | Value |
| --- | --- |
| Name | `devlab-board-handson-db-subnet-group` |
| Description | `Private subnets for devlab-board handson` |
| VPC | `devlab-board-handson-vpc` |

5. VPC作成時に選ばれた2つのAvailability Zoneを選ぶ
6. 各Availability ZoneからPrivate Subnetだけを1つずつ、合計2つ選ぶ
7. `Create`を押す

RDSのDB subnet groupは、少なくとも2つのAvailability Zoneを含む必要がある。

### 10.2 DB instance

1. RDSコンソールの`Databases`を開く
2. `Create database`を押す
3. `Standard create`を選ぶ
4. 次を設定する

| Section | Item | Value |
| --- | --- | --- |
| Engine options | Engine type | `PostgreSQL` |
| Engine options | Engine version | PostgreSQL 18の最新minor。表示されない場合は利用可能な新しいstandard support版 |
| Templates | Template | `Dev/Test`または最小設定を選べるtemplate |
| Availability | Deployment option | `Single DB instance` |
| Settings | DB instance identifier | `devlab-board-handson-db` |
| Settings | Master username | `devlab_admin` |
| Credentials | Credentials management | 手動管理。ハンズオン用の強いpasswordをpassword managerで作成 |
| Instance | DB instance class | `db.t4g.micro`。表示されない場合は選択可能な最小burstable class |
| Storage | Storage type | `General Purpose SSD (gp3)` |
| Storage | Allocated storage | `20 GiB` |
| Storage | Storage autoscaling | 無効 |
| Connectivity | Compute resource | 接続しない |
| Connectivity | VPC | `devlab-board-handson-vpc` |
| Connectivity | DB subnet group | `devlab-board-handson-db-subnet-group` |
| Connectivity | Public access | `No` |
| Connectivity | VPC security group | Existingを選び、`devlab-board-handson-rds-sg`だけを指定 |
| Authentication | Database authentication | Password authentication |
| Additional configuration | Initial database name | `devlab` |
| Additional configuration | Backup retention | 学習後すぐ削除する場合は`0 days` |
| Additional configuration | Deletion protection | 無効 |
| Monitoring | Enhanced Monitoring | 無効 |

5. storage encryptionは有効のままにする
6. Monthly estimated costsが表示される場合は内容を読む
7. `Create database`を押す
8. Statusが`Available`になるまで待つ
9. DB詳細の`Connectivity & security`から次をメモする
   - Endpoint
   - Port `5432`
   - VPC
   - Subnet group
   - Security Group

master passwordは、この後Secrets Managerへ登録するまでpassword managerで保持する。チャット、docs、source code、shell historyへ貼り付けない。

現在のbackendは起動時にテーブルを作成するため、ハンズオンではmaster userを使用する。本番ではmigration用userと、通常のアプリ実行用userを分ける。

## 11. AWS用のfrontend imageを準備する

現在の`frontend/Dockerfile`はVite開発サーバー用である。AWS用にNginxで静的ファイルを配信するDockerfileと設定を追加する。

### 11.1 `frontend/Dockerfile.aws`

```dockerfile
FROM node:22-alpine AS build

RUN corepack enable && corepack prepare yarn@1.22.22 --activate

WORKDIR /app

COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

COPY . .

# APIは同じoriginの /api を使い、Nginxからbackendへproxyする。
RUN VITE_API_BASE_URL= yarn build

FROM nginx:1.28-alpine

COPY nginx.aws.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
```

### 11.2 `frontend/nginx.aws.conf`

```nginx
server {
    listen 80;
    server_name _;

    client_max_body_size 1m;

    location = /healthz {
        proxy_pass http://127.0.0.1:8080/healthz;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

`proxy_pass`の末尾に`/`を付けず、`/api/*`のpathをそのままGo backendへ渡す。

### 11.3 build前の確認

```bash
cd frontend
yarn install --frozen-lockfile
yarn build
cd ../backend
go test ./...
cd ..
```

## 12. ECR repositoryを作成してimageをpushする

### 12.1 frontend repository

1. AWSコンソールで`Elastic Container Registry`を開く
2. `Private repositories`を選ぶ
3. `Create repository`を押す
4. 次を設定する

| Item | Value |
| --- | --- |
| Repository name | `devlab-board-handson/frontend` |
| Image tag mutability | Mutableでもよいが、push時は固定tagを使う |
| Encryption | AES-256 |
| Image scanning | 利用可能な基本scanを有効にする |

5. `Create repository`を押す

### 12.2 backend repository

同じ手順で`devlab-board-handson/backend`を作成する。

### 12.3 ECRへloginする

作成したrepositoryを開き、`View push commands`を押す。表示されたlogin commandをrepository rootで実行する。

例:

```bash
aws ecr get-login-password --region ap-northeast-1 \
  | docker login \
      --username AWS \
      --password-stdin ACCOUNT_ID.dkr.ecr.ap-northeast-1.amazonaws.com
```

`ACCOUNT_ID`は自分のaccount IDへ置き換える。表示されたpasswordをファイルへ保存しない。

### 12.4 imageをbuildしてpushする

Apple SiliconなどARM64のPCからも同じTask Definitionで実行できるよう、今回は`linux/amd64`へ固定する。

```bash
export ECR_REGISTRY="ACCOUNT_ID.dkr.ecr.ap-northeast-1.amazonaws.com"
export IMAGE_TAG="handson-v1"

docker buildx build \
  --platform linux/amd64 \
  --file frontend/Dockerfile.aws \
  --tag "$ECR_REGISTRY/devlab-board-handson/frontend:$IMAGE_TAG" \
  --push \
  frontend

docker buildx build \
  --platform linux/amd64 \
  --file backend/Dockerfile \
  --tag "$ECR_REGISTRY/devlab-board-handson/backend:$IMAGE_TAG" \
  --push \
  backend
```

push後、各ECR repositoryの`Images`で`handson-v1`が存在することを確認する。ECS Task Definitionでは`latest`ではなく、この固定tagを指定する。

## 13. DATABASE_URLをSecrets Managerへ保存する

### 13.1 接続文字列を組み立てる

形式は次のとおり。

```text
postgres://devlab_admin:URLエンコードしたPASSWORD@RDS_ENDPOINT:5432/devlab?sslmode=require
```

passwordに`@`、`:`、`/`、`?`、`#`、`%`などが含まれる場合はURLエンコードが必要になる。ハンズオンではpassword managerで十分に長い英数字のpasswordを生成すると、接続文字列を安全に組み立てやすい。

この接続文字列をshell commandへ直接書くとhistoryに残る可能性があるため、Secrets Managerコンソール内でだけ組み立てる。

### 13.2 Secretを作成する

1. AWSコンソールで`Secrets Manager`を開く
2. `Store a new secret`を押す
3. `Other type of secret`を選ぶ
4. `Plaintext`タブを選ぶ
5. 前項の`postgres://...`を1行で入力する
6. Encryption keyは`aws/secretsmanager`を選ぶ
7. `Next`を押す
8. Secret nameへ`devlab-board-handson/database-url`を入力する
9. Rotationは無効のままにする
10. 内容を確認して保存する
11. Secret detailからSecret ARNをメモする

Secret valueをスクリーンショットへ含めない。後のTask DefinitionではSecret ARNだけを使う。

## 14. ECS task execution roleを作成する

### 14.1 Roleを作成する

1. AWSコンソールで`IAM`を開く
2. `Roles`から`Create role`を押す
3. Trusted entity typeで`AWS service`を選ぶ
4. Use caseで`Elastic Container Service`、`Elastic Container Service Task`を選ぶ
5. Permission policyとして`AmazonECSTaskExecutionRolePolicy`を追加する
6. Role nameへ`devlab-board-handson-ecs-execution-role`を入力する
7. Roleを作成する

このroleは、ECS/Fargate agentがECRからimageを取得し、CloudWatch Logsへlogを送り、Secrets Managerから設定を取得するために使う。アプリケーションコードがAWS APIを呼ぶtask roleとは別である。

### 14.2 Secret取得権限を追加する

1. 作成したroleを開く
2. `Add permissions`から`Create inline policy`を選ぶ
3. JSON editorへ次を入力する
4. `SECRET_ARN`を実際のSecret ARNへ置き換える

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadDatabaseUrlSecret",
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue"
      ],
      "Resource": "SECRET_ARN"
    }
  ]
}
```

5. Policy nameを`ReadDevlabBoardDatabaseUrl`として保存する
6. Role detailからRole ARNをメモする

defaultのSecrets Manager KMS keyを利用するため、今回は個別の`kms:Decrypt`を追加しない。customer managed KMS keyを使う場合は、そのkeyに対する最小限の権限も必要になる。

## 15. CloudWatch Logsのlog groupを作成する

1. AWSコンソールで`CloudWatch`を開く
2. `Logs`、`Log groups`を開く
3. `Create log group`を押す
4. Log group nameへ`/ecs/devlab-board-handson`を入力する
5. Retentionを`7 days`に設定する
6. `Create`を押す

frontendとbackendは、同じlog group内で別stream prefixを使う。

## 16. ECS ClusterとTask Definitionを作成する

### 16.1 Cluster

1. AWSコンソールで`Elastic Container Service`を開く
2. `Clusters`を選ぶ
3. `Create cluster`を押す
4. Cluster nameへ`devlab-board-handson-cluster`を入力する
5. Infrastructureは`AWS Fargate`を利用できる既定設定にする
6. Container Insightsは最初のハンズオンでは無効でもよい
7. `Create`を押す

### 16.2 Task Definition

コンソールのフォームでも作成できるが、2コンテナ、Secret、log設定を間違えにくくするため、コンソール内のJSON editorを使う。

事前に次を用意する。

| Placeholder | Value |
| --- | --- |
| `EXECUTION_ROLE_ARN` | IAM role ARN |
| `BACKEND_IMAGE_URI` | backend ECR URI + `:handson-v1` |
| `FRONTEND_IMAGE_URI` | frontend ECR URI + `:handson-v1` |
| `DATABASE_URL_SECRET_ARN` | Secrets Manager secret ARN |

1. ECSコンソールで`Task definitions`を選ぶ
2. `Create new task definition`からJSON editorを使う作成方法を選ぶ
3. 次のJSONを貼り付ける
4. 4つのplaceholderを実値へ置き換える

```json
{
  "family": "devlab-board-handson-task",
  "executionRoleArn": "EXECUTION_ROLE_ARN",
  "networkMode": "awsvpc",
  "containerDefinitions": [
    {
      "name": "backend",
      "image": "BACKEND_IMAGE_URI",
      "essential": true,
      "environment": [
        {
          "name": "APP_ENV",
          "value": "handson"
        },
        {
          "name": "PORT",
          "value": "8080"
        },
        {
          "name": "GRPC_PORT",
          "value": "50051"
        },
        {
          "name": "SESSION_STORE",
          "value": "postgres"
        },
        {
          "name": "SESSION_COOKIE_SECURE",
          "value": "false"
        }
      ],
      "secrets": [
        {
          "name": "DATABASE_URL",
          "valueFrom": "DATABASE_URL_SECRET_ARN"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/devlab-board-handson",
          "awslogs-region": "ap-northeast-1",
          "awslogs-stream-prefix": "backend"
        }
      }
    },
    {
      "name": "frontend",
      "image": "FRONTEND_IMAGE_URI",
      "essential": true,
      "portMappings": [
        {
          "containerPort": 80,
          "hostPort": 80,
          "protocol": "tcp"
        }
      ],
      "dependsOn": [
        {
          "containerName": "backend",
          "condition": "START"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/devlab-board-handson",
          "awslogs-region": "ap-northeast-1",
          "awslogs-stream-prefix": "frontend"
        }
      }
    }
  ],
  "requiresCompatibilities": [
    "FARGATE"
  ],
  "cpu": "256",
  "memory": "512",
  "runtimePlatform": {
    "cpuArchitecture": "X86_64",
    "operatingSystemFamily": "LINUX"
  }
}
```

5. JSON validationでerrorがないことを確認する
6. `Create`を押す
7. Task Definition familyが`devlab-board-handson-task`、revisionが`1`であることを確認する

`DATABASE_URL`を通常の`environment`へ入れず、必ず`secrets`として指定する。

## 17. ECS Serviceを作成する

1. ECSコンソールで`devlab-board-handson-cluster`を開く
2. `Services`タブから`Create`を押す
3. 次を設定する

| Section | Item | Value |
| --- | --- | --- |
| Environment | Compute options | Launch type |
| Environment | Launch type | FARGATE |
| Deployment | Application type | Service |
| Deployment | Task definition family | `devlab-board-handson-task` |
| Deployment | Revision | 最新revision |
| Deployment | Service name | `devlab-board-handson-service` |
| Deployment | Desired tasks | `1` |
| Networking | VPC | `devlab-board-handson-vpc` |
| Networking | Subnets | 2つのPublic Subnet |
| Networking | Security Group | Existingから`devlab-board-handson-ecs-sg`だけを選ぶ |
| Networking | Public IP | `Turned on` |
| Load balancing | Load balancer | なし |
| Service Connect | Service Connect | 無効 |
| Service Auto Scaling | Auto Scaling | 無効 |

4. Security Groupの新規自動作成を選ばない
5. Deployment failure detectionでcircuit breakerを選べる場合は有効にし、rollbackも有効にする
6. 内容を確認して`Create`を押す
7. Serviceの`Tasks`タブでTaskが`RUNNING`になるまで待つ

Taskが`STOPPED`を繰り返す場合は、先にService eventとStopped reasonを確認する。闇雲にTask Definition revisionを増やさない。

## 18. デプロイ結果を確認する

### 18.1 Public IP

1. ECS Clusterを開く
2. `devlab-board-handson-service`を開く
3. `Tasks`タブで`RUNNING`のTaskを選ぶ
4. `Networking`セクションからPublic IPを確認する

以降、この値を`TASK_PUBLIC_IP`とする。

Public IPは固定ではない。Taskが置き換わると変わる可能性がある。

### 18.2 health check

```bash
curl --fail --show-error "http://TASK_PUBLIC_IP/healthz"
```

期待するresponse例:

```json
{
  "status": "ok",
  "service": "devlab-board-backend",
  "env": "handson"
}
```

### 18.3 画面確認

ブラウザで次を開く。

```text
http://TASK_PUBLIC_IP
```

次の順に確認する。

1. Login / Register画面が表示される
2. 架空の学習用メールアドレスと、使い回していないpasswordでユーザー登録する
3. 登録後にダッシュボードへ遷移する
4. `Session`が`active`と表示される
5. `Store`が`postgres`と表示される
6. ページを再読み込みしてもログイン状態が維持される
7. `User-Agent Lab`を開き、Go APIから応答が返る
8. Logout後にログイン画面へ戻る
9. 作成したユーザーで再ログインできる

### 18.4 Network確認

ブラウザのDevToolsで`Network`を開き、次を確認する。

- `/api/auth/register`または`/api/auth/login`が`2xx`
- responseに`Set-Cookie`がある
- `/api/auth/me`にCookieが送信される
- frontendとAPIのhostが同じPublic IPである
- APIの接続先が`localhost`になっていない

### 18.5 CloudWatch Logs

1. CloudWatchコンソールで`/ecs/devlab-board-handson`を開く
2. `backend`と`frontend`で始まるlog streamを確認する
3. backendの起動logを確認する
4. frontendのaccess logで`/`、`/api/*`、`/healthz`を確認する

password、Cookie、`DATABASE_URL`がlogへ出ていないことも確認する。

### 18.6 RDS

RDSコンソールで次を確認する。

- DB statusが`Available`
- Publicly accessibleが`No`
- 接続数が増えている
- ECS用Security Group以外からのinboundを許可していない

## 19. 更新とロールバックを試す

### 19.1 新しいimageをpushする

小さな画面文言などを変更し、tagを変えてpushする。

```bash
export ECR_REGISTRY="ACCOUNT_ID.dkr.ecr.ap-northeast-1.amazonaws.com"
export IMAGE_TAG="handson-v2"

docker buildx build \
  --platform linux/amd64 \
  --file frontend/Dockerfile.aws \
  --tag "$ECR_REGISTRY/devlab-board-handson/frontend:$IMAGE_TAG" \
  --push \
  frontend
```

### 19.2 Task Definition revision

1. ECSコンソールで`devlab-board-handson-task`を開く
2. 最新revisionから`Create new revision`を選ぶ
3. frontend image URIだけを`:handson-v2`へ変更する
4. Secretや他の環境変数が維持されていることを確認する
5. 新revisionを作成する

### 19.3 Serviceを更新する

1. ECS Serviceを開く
2. `Update service`を押す
3. Task Definition revisionを新しいrevisionへ変更する
4. Desired tasksは`1`のままにする
5. 更新を開始する
6. 新Taskが`RUNNING`になり、旧Taskが停止することを確認する
7. 新しいPublic IPを取得して画面を確認する

### 19.4 ロールバック

不具合があれば、同じ`Update service`でTask Definition revisionを直前の正常revisionへ戻す。

この構成ではPublic IPが変わり得るため、更新後は必ず現在のTaskからIPを再取得する。

## 20. トラブルシューティング

### 20.1 Taskが`STOPPED`になる

ECSのTask detailで次を確認する。

- Stopped reason
- backend containerのreasonとexit code
- frontend containerのreasonとexit code
- Service events
- CloudWatch Logs

### 20.2 `CannotPullContainerError`

- ECR image URIとtagが存在するか
- imageをpushしたRegionとECSのRegionが同じか
- TaskへPublic IPを付与したか
- Public SubnetがInternet Gatewayへのrouteを持つか
- task execution roleに`AmazonECSTaskExecutionRolePolicy`があるか
- buildしたCPU architectureとTask Definitionの`X86_64`が一致するか

### 20.3 Secretを取得できない

- Task Definitionの`DATABASE_URL_SECRET_ARN`が完全なARNか
- task execution roleに`secretsmanager:GetSecretValue`があるか
- policyのResourceが対象Secret ARNと一致するか
- SecretとECSが同じRegionか
- custom KMS keyを使っている場合、必要な`kms:Decrypt`があるか

### 20.4 backendがDBへ接続できない

- RDS statusが`Available`か
- `DATABASE_URL`のhostがRDS Endpointと一致するか
- database nameが`devlab`か
- userとpasswordが正しいか
- `sslmode=require`が付いているか
- ECSとRDSが同じVPCか
- RDS Security Groupの5432 sourceがECS Security Groupか
- RDS Public accessを有効にして解決しようとしていないか

### 20.5 frontendは表示されるがAPIが`502`

- backend containerが起動しているか
- CloudWatch LogsにDB接続errorがないか
- backendがport 8080で待ち受けているか
- `nginx.aws.conf`のproxy先が`127.0.0.1:8080`か
- frontendとbackendが同じTask Definitionに含まれているか

### 20.6 APIの接続先が`localhost:30102`になる

frontend imageがローカル用設定でbuildされている。

- `frontend/Dockerfile.aws`の`RUN VITE_API_BASE_URL= yarn build`を確認する
- image tagを更新してpushする
- 新しいTask Definition revisionを作る
- Serviceを更新する

### 20.7 login後にsessionが維持されない

- frontendとAPIが同じPublic IPから配信されているか
- `/api`がNginx経由になっているか
- browserのCookieに`devlab_session`があるか
- `/api/auth/me`にCookieが送信されているか
- dashboardの`Store`が`postgres`か
- backend logにsession DB errorがないか

### 20.8 ブラウザから接続できない

- Taskが`RUNNING`か
- 現在のTaskのPublic IPを見ているか
- ECS Security Groupが現在の自分のPublic IPv4 `/32`を許可しているか
- VPN、proxy、テザリングでPublic IPが変わっていないか
- Public SubnetとPublic IP assignmentを選んだか
- 組織networkがTCP 80を遮断していないか

## 21. AWSリソースを削除する

課金停止までがハンズオンである。削除は依存関係を考慮して次の順に行う。

### 21.1 ECS Service

1. ECS Clusterを開く
2. Serviceを選択する
3. `Update service`でDesired tasksを`0`にするか、削除画面の指示に従う
4. Running taskが0になったことを確認する
5. `devlab-board-handson-service`を削除する
6. Cluster内にTaskが残っていないことを確認する

### 21.2 ECS ClusterとTask Definition

1. `devlab-board-handson-cluster`を削除する
2. Task DefinitionのrevisionをすべてDeregisterする

### 21.3 RDS

1. RDSのDatabasesを開く
2. `devlab-board-handson-db`を選択する
3. `Delete`を選ぶ
4. Final snapshotを作成しない
5. Automated backupsを保持しない
6. 削除確認文字列を入力する
7. DBが一覧から消えるまで待つ

学習結果を残す必要がある場合でも、個人データを含むsnapshotを無期限に残さない。

### 21.4 Secrets Manager

1. `devlab-board-handson/database-url`を開く
2. `Delete secret`を選ぶ
3. 復旧期間を確認して削除を予約する

Secret valueを確認画面や削除記録へコピーしない。

### 21.5 ECR

1. `devlab-board-handson/frontend`を選択する
2. repository内のimageを削除する
3. repositoryを削除する
4. `devlab-board-handson/backend`も同様に削除する

### 21.6 CloudWatch Logs

`/ecs/devlab-board-handson` log groupを削除する。

### 21.7 IAM

1. `devlab-board-handson-ecs-execution-role`を開く
2. inline policyとattached policyを確認する
3. roleを削除する

他のTask DefinitionやServiceが利用していないことを先に確認する。

### 21.8 DB subnet groupとSecurity Group

1. RDSの`devlab-board-handson-db-subnet-group`を削除する
2. VPCコンソールで`devlab-board-handson-rds-sg`を削除する
3. `devlab-board-handson-ecs-sg`を削除する

依存するENIが残っている場合は、ECS TaskやRDSの削除完了を待つ。

### 21.9 VPC

1. VPCコンソールで`devlab-board-handson-vpc`を選ぶ
2. Resource mapでFargate ENI、RDS ENI、Security Groupなどが残っていないことを確認する
3. VPCを削除する

AWS Budgetsは今後の学習にも使えるため、不要でなければ残してよい。

### 21.10 削除後確認

次が残っていないことを各コンソールで確認する。

- Running ECS Task
- ECS Service
- RDS DB instance
- RDS snapshot / retained automated backup
- ECR image
- Secrets Manager secret
- CloudWatch log group
- 学習用VPC

翌日以降もBilling and Cost Managementで利用料金を確認する。

## 22. 次の発展

このハンズオンを一度作成・確認・削除できた後、次の順に拡張する。

1. ALBを追加し、Taskを直接公開しない
2. ACM証明書と独自domainでHTTPS化する
3. `SESSION_COOKIE_SECURE=true`にする
4. frontendをS3 + CloudFrontへ分離する
5. backendをPrivate Subnetへ移す
6. NAT GatewayとECR / Logs / Secrets Manager VPC endpointのcostを比較する
7. migration用DB userとアプリ用DB userを分ける
8. RDS backup、Multi-AZ、監視、alarmを追加する
9. ElastiCache Redisへsession storeを移す
10. Terraformで同じ構成を再現する
11. GitHub Actions + OIDCでECR pushとECS deployを自動化する

コンソールで一度関係を確認した後、既存のTerraformハンズオンや`infra/`へ知識を戻す。

## 23. 公式リファレンス

- [Amazon VPC: Create a VPC](https://docs.aws.amazon.com/vpc/latest/userguide/create-vpc.html)
- [Amazon RDS: Creating an Amazon RDS DB instance](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_CreateDBInstance.html)
- [Amazon RDS: Working with a DB instance in a VPC](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_VPC.WorkingWithRDSInstanceinaVPC.html)
- [Amazon ECR: Creating a private repository](https://docs.aws.amazon.com/AmazonECR/latest/userguide/repository-create.html)
- [Amazon ECR: Pushing an image](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-push.html)
- [Amazon ECS: Creating a task definition using the console](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)
- [Amazon ECS: Getting started with Fargate](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/getting-started-fargate.html)
- [Amazon ECS: Fargate task networking](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-task-networking.html)
- [Amazon ECS: Containers in the same task and localhost](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-networking-awsvpc.html)
- [Amazon ECS: Task execution IAM role](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)
- [Amazon ECS: Pass Secrets Manager secrets through environment variables](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-envvar-secrets-manager.html)
- [AWS Secrets Manager: Create a secret](https://docs.aws.amazon.com/secretsmanager/latest/userguide/create_secret.html)
- [Amazon CloudWatch Logs: Working with log groups](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html)
- [AWS Cost Management: Using a budget template](https://docs.aws.amazon.com/cost-management/latest/userguide/budget-templates.html)
- [AWS Fargate Pricing](https://aws.amazon.com/fargate/pricing/)
- [Amazon RDS for PostgreSQL Pricing](https://aws.amazon.com/rds/postgresql/pricing/)
- [Amazon VPC Pricing](https://aws.amazon.com/vpc/pricing/)
- [AWS Secrets Manager Pricing](https://aws.amazon.com/secrets-manager/pricing/)

## 24. 更新履歴

| 日付 | 内容 |
| --- | --- |
| 2026-07-27 | AWSコンソール中心でdevlab-boardをECS Fargate、RDS、ECRへデプロイする初稿を作成 |

## 25. 要確認・ヒアリング項目

- AWS accountは個人学習用か、組織管理accountか
- IAM Identity Centerを使えるか
- `ap-northeast-1`でRDS PostgreSQL 18と`db.t4g.micro`を選択できるか
- AWS用frontend DockerfileとNginx設定を正式にrepositoryへ追加するか
- 最初はこのコンソール構成を試すか、既存Terraform最小構成のApache確認を先に行うか
- ハンズオン後すぐdestroyするか、数日間だけdev環境として残すか
- 次の段階でALB + HTTPSを先に追加するか、S3 + CloudFrontへfrontendを分離するか
