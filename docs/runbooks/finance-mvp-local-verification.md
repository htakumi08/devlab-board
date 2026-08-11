---
title: Finance読み取りMVP ローカル検証Runbook
status: implementation
last_updated: 2026-08-11
requirements: FR-AUTH-002, FR-AUTH-003, FR-AUTH-004, FR-AUTH-005, FR-TXN-004, FR-TXN-005, FR-TXN-006, FR-TXN-007
---

# Finance読み取りMVP ローカル検証Runbook

## 目次

1. [目的](#1-目的)
2. [前提](#2-前提)
3. [環境変数](#3-環境変数)
4. [synthetic fixtureを投入する](#4-synthetic-fixtureを投入する)
5. [共通smoke testを実行する](#5-共通smoke-testを実行する)
6. [fixtureを削除する](#6-fixtureを削除する)
7. [実装検証](#7-実装検証)
8. [手動確認](#8-手動確認)
9. [MVP全体の受け入れマトリクス](#9-mvp全体の受け入れマトリクス)
10. [AWS variantでの再利用](#10-aws-variantでの再利用)
11. [更新履歴](#11-更新履歴)
12. [要確認・ヒアリング項目](#12-要確認ヒアリング項目)

## 1. 目的

Docker Compose上の既存ユーザーへ再現可能なsynthetic Financeデータを投入し、Overview、Accounts、Account Detail、Transactionsの共通smoke testを実行してから、fixture専用データだけを削除する。

この手順はsynthetic data専用である。実在する氏名、メール、口座番号、取引、passwordをfixtureへ使用しない。seed commandは`APP_ENV=local|development|test`だけを許可し、productionではDB接続前に停止する。

## 2. 前提

- repository rootに`.env`があり、`docker compose up -d --build`が成功している。
- migrationが適用済みである。seed commandも実行前にmigrationを確認する。
- primaryとsecondaryの2人のsmoke用ユーザーが既に登録済みである。2人ともこの検証専用とし、実在する利用者を使わない。
- 2人の認証情報をGit、shell history、logへ残さない。
- Slice 3Bの`GET /api/finance/categories`とTransactions filter APIが実装済みである。

## 3. 環境変数

emailはsyntheticな検証用アドレスを指定する。passwordは表示しない入力から環境変数へ設定する。

```bash
export FINANCE_SMOKE_EMAIL='finance-smoke@example.invalid'
export FINANCE_SMOKE_SECONDARY_EMAIL='finance-smoke-secondary@example.invalid'
printf 'Finance smoke primary password: ' >&2
read -rs FINANCE_SMOKE_PASSWORD
printf '\n' >&2
export FINANCE_SMOKE_PASSWORD
printf 'Finance smoke secondary password: ' >&2
read -rs FINANCE_SMOKE_SECONDARY_PASSWORD
printf '\n' >&2
export FINANCE_SMOKE_SECONDARY_PASSWORD
```

完了時は認証情報を現在のshellから削除する。

```bash
unset FINANCE_SMOKE_EMAIL FINANCE_SMOKE_PASSWORD \
  FINANCE_SMOKE_SECONDARY_EMAIL FINANCE_SMOKE_SECONDARY_PASSWORD
```

## 4. synthetic fixtureを投入する

seedはユーザーの公開UUIDと安定keyから決定的な公開UUIDを作る。各ユーザーに2口座と40取引をtransaction内で再作成するため、`apply`を繰り返してもIDと件数は変わらない。

fixtureには25件超の取引、5カテゴリー、debit/credit、pending/posted/reversed、null merchant/category、同一時刻、JavaScript safe integerを超えるminor amountが含まれる。完全な口座番号は保存しない。

```bash
for finance_seed_email in \
  "$FINANCE_SMOKE_EMAIL" \
  "$FINANCE_SMOKE_SECONDARY_EMAIL"
do
  docker compose exec -T \
    -e FINANCE_SEED_USER_EMAIL="$finance_seed_email" \
    -e FINANCE_SEED_ACTION=apply \
    backend go run ./cmd/finance-seed
done
unset finance_seed_email
```

`APP_ENV`と`DATABASE_URL`はbackend serviceに設定済みの値を使う。commandは対象email、取引名、金額をlogへ出さない。

## 5. 共通smoke testを実行する

smoke commandはGo標準ライブラリのHTTP clientとprimary / secondaryで分離したcookie jarを使い、response body、email、password、session cookieを出力しない。全体を既定30秒で停止する。

`FINANCE_SMOKE_API_URL`はHTTPSを指定する。平文HTTPを許可するのは`localhost`、`127.0.0.1`、`::1`だけであり、remote hostへ認証情報を平文送信しない。Docker Composeではbackend container自身の`http://localhost:8080`を使用する。

```bash
docker compose exec -T \
  -e FINANCE_SMOKE_FRONTEND_URL=http://frontend:5173 \
  -e FINANCE_SMOKE_API_URL=http://localhost:8080 \
  -e FINANCE_SMOKE_EMAIL \
  -e FINANCE_SMOKE_PASSWORD \
  -e FINANCE_SMOKE_SECONDARY_EMAIL \
  -e FINANCE_SMOKE_SECONDARY_PASSWORD \
  -e FINANCE_SMOKE_TIMEOUT=30s \
  backend go run ./cmd/finance-smoke
```

自動確認する公開contract:

1. frontend `/`、`/finance-lab`、`/finance-lab/transactions`がHTMLを返す。
2. `/healthz`が`status=ok`を返す。
3. login前のFinance APIが401を返す。
4. primaryとsecondaryが別cookie jarでloginでき、secondary自身の`Synthetic Everyday`口座IDをprimary sessionで取得すると404になる。
5. login後にsummary、accounts、account detail、categoriesを取得できる。
6. Transactionsの既定表示と古い順が時系列どおりである。
7. 組み合わせfilterの全行が、primary口座、income、debit、posted、指定UTC期間を満たす。
8. `limit=1`の2ページが異なる取引を返す。
9. primaryとsecondaryをlogoutし、primaryのFinance APIが401へ戻る。

失敗時はstep名とHTTP statusだけを表示する。認証情報や金融response bodyは表示しない。

## 6. fixtureを削除する

成功・失敗にかかわらず、検証後に2ユーザー分をcleanupする。cleanupは各対象ユーザーの決定的なfixture口座2件だけを削除し、fixture取引は外部キーのcascadeで削除する。他の口座と他ユーザーのデータは変更しない。

```bash
for finance_seed_email in \
  "$FINANCE_SMOKE_EMAIL" \
  "$FINANCE_SMOKE_SECONDARY_EMAIL"
do
  docker compose exec -T \
    -e FINANCE_SEED_USER_EMAIL="$finance_seed_email" \
    -e FINANCE_SEED_ACTION=clean \
    backend go run ./cmd/finance-seed
done
unset finance_seed_email
```

`clean`はfixtureが存在しない状態でも成功するため、再実行できる。

## 7. 実装検証

```bash
docker compose run --rm -e RUN_DB_TESTS=1 backend \
  sh -c 'go test -count=1 ./internal/financeseed ./internal/financesmoke'

docker compose run --rm -e RUN_DB_TESTS=1 backend \
  sh -c 'go test -count=1 ./... && go vet ./...'
```

seed integration testは`apply`2回、`clean`2回、別ユーザー分離、通常口座保持、production拒否を確認する。smoke testは`httptest`で公開journey、2ユーザーownership拒否、filter全条件、sort順序、page非重複、API URL制限、timeout、秘密情報をerrorへ含めないことを確認する。

## 8. 手動確認

smokeだけでは視覚表示と実log内容を保証できないため、次を別に確認する。

- desktopでfilter、sort、pagination、直接URL、再読込、戻る／進むを操作する。
- 390x844で主要filterを操作でき、document全体に横overflowがなく、tableだけが必要に応じて横scrollする。
- keyboardだけでfilter、reset、pagination、navigationを操作し、visible focusとlabelを確認する。
- `docker compose logs backend`を確認し、password、session token、完全な口座番号、取引名、merchant、金額、cursor、SQL詳細が出ていないことを確認する。
- loading、background refreshing、empty、400、401、403、500をcomponent testまたは意図的な検証環境で確認する。

## 9. MVP全体の受け入れマトリクス

| MVP受け入れ条件 | 主な自動確認 | 補完確認 |
| --- | --- | --- |
| Overview、Accounts、Account Detail、Transactionsを移動できる | frontend route/component test、smokeのdirect linkと各API | desktop/mobile browser |
| 総残高、口座数、最近の取引、口座別取引が整合する | Finance repository integration test、seed fixture、smoke | browser表示値 |
| 取引をページングできる | repository pagination test、frontend test、smoke `limit=1` | browser Back/Forward |
| 主要filterがURLと同期する | frontend URL test、API filter integration test、smoke組み合わせfilter | direct URLと再読込 |
| 他ユーザーの口座を取得できない | account ownership test、primary / secondary共通smoke | backend logにresource情報が出ないこと |
| loading、empty、error、permissionを確認できる | frontend component test | background refreshとofflineのbrowser確認 |
| 完全な口座番号、password、tokenがUI/API/logへ出ない | seed/smoke security test、API response test | backend log確認 |
| Finance専用tableが`finance_`プレフィックスに従う | migration integration test、DBML review | PostgreSQL catalog確認 |
| desktop/mobileで主要閲覧操作ができる | semantic component test | desktopと390x844 browser |
| synthetic dataでローカルの一連の動作を再現できる | seed反復integration test、共通smoke | このrunbookのapply→smoke→clean |

## 10. AWS variantでの再利用

共通smokeはDBへ直接接続せず、公開frontend/API contractだけを使う。AWS環境では事前に2人のsynthetic fixtureを安全な手段で用意し、HTTPS URLと4つの認証環境変数をGitHub Environment等のsecretから注入する。productionへseed commandを持ち込んだり、実金融データへsmokeを実行したりしない。

log確認、rollback、fixture準備方法はvariant固有runbookで補足し、共通smoke自体のendpointと期待値は変更しない。

## 11. 更新履歴

| 日付 | 内容 |
| --- | --- |
| 2026-08-11 | deterministic seed、共通smoke、cleanup、MVP受け入れマトリクスの初版を作成 |
| 2026-08-11 | 2ユーザーownership、response semantics、remote API HTTPS制約を共通smokeへ追加 |

## 12. 要確認・ヒアリング項目

- 現時点でこのローカル検証手順を妨げる未決定事項はない。
- AWS variantで使うsynthetic fixtureの事前準備とcredential注入方法は、各variantの設計時に確定する。
