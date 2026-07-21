# Testing ルール

## 方針

- 変更範囲に対して最小限だが意味のある検証を行う。
- 失敗を再現できる bugfix では、先に失敗するテストまたは再現手順を確認する。
- 大きな E2E より、API handler、component boundary、Terraform validate など近い層から固める。
- 実行できなかった検証は、理由と残リスクを報告する。

## バックエンド

- Go backend は `go test ./...` を基本にする。
- HTTP handler は `httptest` で status、headers、JSON body を確認する。
- DB 導入後は repository / migration / transaction の検証方針を決める。
- external service 連携は interface や small wrapper で差し替え可能にする。

## フロントエンド

- 現状の基本確認は `yarn build`。
- test / lint script が追加されたら、対象変更に合わせて実行する。
- API boundary、loading、empty、error state は component または integration test の対象にする。
- browser 上の見た目や interaction が重要な変更では、スクリーンショットまたは手動確認結果を残す。

## インフラ

- Terraform 変更では `terraform fmt -check -recursive`。
- 対象 env で `terraform validate`。
- AWS 認証と backend 初期化が可能なら `terraform plan`。
- destructive diff、cost impact、public exposure は review 対象にする。
