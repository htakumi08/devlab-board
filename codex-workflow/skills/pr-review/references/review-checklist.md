# PRレビュー観点

差分だけでなく、関連実装、docs、設定、検証手順まで読んだうえで確認する。

| 観点 | 何を見るか | 典型的な見落とし | 主な確認先 |
| --- | --- | --- | --- |
| 仕様・業務ルール整合性 | 変更目的、分岐条件、状態遷移、表示条件、API contract が一致するか | UI だけ合っていて保存条件や API response がずれる | README、docs、API handler、frontend state、Terraform variables |
| 不要コード混入 | 一時ログ、デバッグコード、未使用 import、生成物、不要 refactor が混ざっていないか | `dist`、local logs、暫定コメント、未使用 helper の混入 | 差分全体、`.gitignore`、frontend dist、backend tmp |
| 既存挙動への副作用 | 共通処理変更が他画面、他 API、infra、docs に波及しないか | shared helper や env 名変更で別 service が動かない | `rg`、呼び出し元、Docker Compose、README、docs |
| 責務分離の妥当性 | `cmd/`、handler、use case、repository、platform、component の責務が保たれているか | `cmd/` に業務ロジック、handler に DB 処理、component に API 詳細が散る | backend、frontend、codex-workflow/rules |
| バリデーションの妥当性 | request、form、env、DB input、Terraform variable の検証が足りるか | 空値や境界値、型不一致、未定義 env の見落とし | handler、Form/UI、config、variables.tf |
| 認可・セキュリティ | secret 混入、認証/認可、CSRF/cookie、CORS、XSS、public exposure が安全か | `.env` や logs の混入、token storage、広すぎる IAM | security rule、backend、frontend、infra、Docker Compose |
| DB更新の整合性 | migration、schema、transaction、失敗時の整合性、docs/DBML との一致 | schema だけ変えて migration なし、途中失敗で片側だけ更新 | docs/db-design、migrations、repository、service |
| クエリと性能 | N+1、全件取得、index、pagination、Terraform cost impact が妥当か | 小さい実装でも一覧/API で無制限取得する | repository、SQL、DBML indexes、infra |
| 例外処理とログ | error response と server log の境界、個人情報や secret の出力有無 | stack trace 露出、password/token logging、失敗握りつぶし | handler、service、logger、frontend error state |
| 定数・設定値の扱い | magic number/string、env、config、Terraform variable が整理されているか | URL や port の重複、環境名直書き、設定 docs との不一致 | `.env.example`、Compose、frontend config、Terraform |
| テストの妥当性 | 変更リスクに見合う backend/frontend/infra/docs 検証があるか | 正常系だけ、契約変更のテストなし、build 未確認 | tests、package scripts、go test、terraform validate |
| 外部連携影響 | AWS、RDS、S3、CloudFront、GitHub Actions、メール/queue などへの影響 | local だけ動き、AWS 構成や CI/CD が追従しない | infra、docs、workflow、env |
| 可読性・保守性 | 命名、重複、分岐、コメント、既存パターンとの一貫性 | 学習メモがコードコメントに残りすぎる、過剰抽象化 | 差分全体、既存類似処理 |
| 堅牢性 | 想定外入力、再実行、競合、部分失敗、offline/error state に耐えるか | 二重送信、null/empty 未考慮、起動順依存 | handler、service、frontend state、Compose healthcheck |
| 既存の類似処理との関連性 | 既存 endpoint、component、module、docs と同じ方針か | 類似処理があるのに別方式を増やす | `rg`、既存 skills、playbooks、rules |

## 補助方針

- 共通部品、環境変数、DB schema、Terraform module に触る変更は利用箇所探索を優先する。
- 仕様が不明な場合は、指摘を `仕様確認待ち` として分離する。
- `問題なし` と書く場合も、何を確認したかを根拠付きで残す。
