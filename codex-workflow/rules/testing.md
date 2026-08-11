# Testing ルール

## 方針

- 変更範囲に対して最小限だが意味のある検証を行う。
- 新機能と不具合修正は、守るべき挙動を表すtestを先に追加し、対象testが期待した理由で失敗するREDを確認してから最小実装でGREENにする。
- REDではcompile errorだけで終わらせず、可能な範囲で未実装の挙動に対応するassertion failureまで進める。実行した対象testと失敗理由を作業結果へ残す。
- GREEN後は重複と責務を見直し、refactor後に対象testと変更領域の全testを再実行する。
- 失敗を再現できる bugfix では、先に失敗するテストまたは再現手順を確認する。
- 大きな E2E より、API handler、component boundary、Terraform validate など近い層から固める。
- 実行できなかった検証は、理由と残リスクを報告する。

## テストケースのコメント

- 各テストケースの直前に、そのケースで「何を確認するか」と「なぜ必要か」を短いコメントで残す。
- コメントは assertion の逐語説明ではなく、守るべき挙動と、そのケースが防ぐ回帰や境界条件を説明する。
- table-driven test や subtest でも、各 case / subtest に対応する位置へコメントを置く。
- テストの目的や条件を変更した場合は、コメントも同時に更新する。

推奨形式:

```go
// テスト内容: GreetingServiceがgRPCサーバーへ登録されていることを確認する。
// 必要な理由: 登録漏れがあると、実装済みのRPCをクライアントから呼び出せないため。
```

## バックエンド

- Go backend は `go test ./...` を基本にする。
- HTTP handler は `httptest` で status、headers、JSON body を確認する。
- DB 導入後は repository / migration / transaction の検証方針を決める。
- PostgreSQL integration testは通常testから分離し、Docker Compose上で `RUN_DB_TESTS=1 go test` として明示実行する。
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
