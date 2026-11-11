---
title: Finance Spending Summary実装
status: implemented
last_updated: 2026-09-24
requirements: UC-06, FR-SUM-006, FR-CAT-002, FR-CAT-003, FR-AUTH-003, FR-AUTH-004
---

# Finance Spending Summary実装

## 1. 目的

Slice 4をレビュー可能な大きさへ分け、最初の変更で所有者境界を持つ支出集計APIを完成させる。

## 2. PR分割

1. Slice 4A: Go API、PostgreSQL集計、unit / integration test、API契約。
2. Slice 4B: React API client、期間選択、chart、数値代替、画面状態、browser確認。
3. 集計queryの実行計画で必要性が確認された場合だけ、index migrationを独立変更として追加する。

BackendとFrontendを同時に変更すると15ファイル以上にまたがり、SQL正確性とUI判断を一度にレビューする必要があるため分割する。

## 3. Slice 4Aの対象範囲

- `GET /api/finance/spending-summary`。
- 必須のUTC期間と最大366日のvalidation。
- session userによるownership。
- `posted + debit`だけの通貨・カテゴリー別集計。
- 未分類、複数通貨、10進文字列の巨大金額、空結果。
- 集計対象取引の最新 `updated_at` を `asOf` とする。
- 未認証、不正query、DB障害の固定response。

対象外:

- frontend表示。
- ユーザーtimezone設定。
- 口座filter。
- schema、migration、seedの変更。

既存seedは2口座、40取引、複数通貨、全status・direction・category、未分類を含むため、追加fixtureは不要と判断した。repository integration testはtransaction内で境界専用データを作成し、終了時にrollbackする。

## 4. 設計判断

- 既存 `/api/finance/summary` へ含めず独立endpointとし、支出集計障害で残高概要まで500になる結合を避ける。
- 日付解析は取引一覧と同じ `YYYY-MM-DD` とUTC境界を再利用する。
- `pending`は未確定、`reversed`は取消、`credit`は入金のため合計しない。
- 通貨換算を行わず、各通貨の合計とカテゴリー内訳を分ける。
- PostgreSQLの `SUM(bigint)` をtextへ変換し、JavaScript安全整数とGo `int64`の上限を越えても精度を失わない。
- category未設定を捨てず `null` として返す。
- schemaは既存のカテゴリーmasterと取引tableで足りるため変更しない。

## 5. TDDと検証

- use case testを先に追加し、未変換のゼロ時刻とvalidation欠落によるREDを確認した。
- HTTP testを先に追加し、未実装handlerの`501`によるREDを確認した。
- PostgreSQL 18のintegration testでownership、status、direction、UTC期間、通貨分離、未分類、`SUM(bigint)`、empty、`asOf`を確認した。
- PostgreSQL統合を含む `go test ./...`、`go vet ./...`、Docker build、認証付きHTTP smokeが成功した。
- 本番相当queryが既存user recent indexで走査可能なことを、index scanを強制した `EXPLAIN` で確認した。小規模なtest dataでは通常plannerがSeq Scanを選ぶため、実データ規模の性能を証明するものではない。
- 現時点では専用indexが必要という根拠がないためmigrationを追加しない。データ量が増えた時点で通常設定の `EXPLAIN (ANALYZE, BUFFERS)` を再確認する。

## 6. 後続作業

- Slice 4Bで期間選択、chart、数値代替、loading / empty / errorを実装する。
- synthetic seedを使うHTTP smokeへ支出集計を追加する。
- ユーザーtimezone設定の正を決めた後にUTC固定を拡張する。

## 7. 更新履歴

| 日付 | 内容 |
| --- | --- |
| 2026-09-24 | PR分割、API境界、集計条件、TDD計画を記録 |
| 2026-09-24 | PostgreSQL統合、実行計画、Docker認証付きsmokeを完了 |
