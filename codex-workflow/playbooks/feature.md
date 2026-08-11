---
name: feature
description: Go API と React UI を縦に通す境界の明確な機能を実装するときに使う。
---

# 機能実装プレイブック

1. ユーザーワークフローと実験機能の目的を確認する。
2. API boundary、request、response、error shape、DB の要否を定義する。
3. Go backend の handler、use case、test を実装する。
4. React の状態を local / global / server に分類し、API client、UI、loading/error/empty state を実装する。
5. Docker Compose、環境変数、AWS 構成に影響があるか確認する。
6. 実装完了前に、コメントと docstring が運用ルールに沿っているか見直す。
7. 契約や決定事項が安定したら docs を更新する。
8. backend と frontend の対象範囲に絞って検証する。
9. コミットする場合はstage済み差分を確認し、`機能: <要約>` 等の日本語種別と、実装内容が簡潔に分かる日本語の要約を付ける。

フロントエンドとバックエンドの変更は、それぞれ単独でも理解できる状態に保つ。どちらか一方が stub の場合は、引き継ぎで明示する。

## コメント / Docstring 見直しチェック

- exported function、component、custom hook、module に、必要な docstring が付いているか。
- 「何をしているか」の言い換えではなく、「なぜこの実装にしているか」「どの制約を守っているか」が残っているか。
- 状態遷移、認可、DB transaction、非同期競合、`null` / `undefined` の扱いなど、読み違えやすい箇所に補足があるか。
- accessibility や運用上の理由で意味のある UI 制約がある場合、その意図が分かるか。
- Terraform の module、variable、output に `description` があり、IAM や network の意図が読み取れるか。
- 各テストケースに、「何をテストするか」と「なぜそのケースが必要か」が短いコメントで残り、現在の assertion と一致しているか。
- 自明な代入、単純な JSX の見た目説明、1 行ごとの言い換えコメントが残っていないか。
