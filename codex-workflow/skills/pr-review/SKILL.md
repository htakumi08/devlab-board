---
name: pr-review
description: PR、ブランチ差分、実装差分、サブエージェント成果物をレビューするときに使う。差分だけでなく関連する呼び出し元、呼び出し先、既存類似処理、docs、API contract、DB schema、Terraform、frontend/backend 境界まで追い、必須レビュー観点をすべて確認し、指摘を重大度「大・中・小」で重み付けして固定フォーマットで findings-first のレビュー結果を出力する。
---

# PR Review Skill

## 目的

devlab-board の変更を、差分だけでなく周辺実装と設計文書まで確認してレビューする。指摘は重大度順に出し、問題がない場合も確認済み観点と残リスクを残す。

## 入力

- 比較元 ref または merge 先 branch
- 比較先 ref または PR branch
- PR タイトルまたは変更目的
- 期待する挙動、関連 Issue、設計メモ
- 未解決 review comment、CI/checks の結果
- 優先レビュー観点があればその指定

入力が不足していても、ローカル差分レビューなら `git status --short` と `git diff HEAD` から開始する。PR 意図がコードから断定できない場合は、不明点として扱う。

## 標準手順

1. root `AGENTS.md`、`codex-workflow/README.md`、`codex-workflow/AGENTS.md`、対象領域の `AGENTS.md` を読む。
2. `codex-workflow/playbooks/code-review.md` と、変更領域に関係する `rules/`、必要な `skills/` を読む。
3. branch 比較なら `scripts/compare_refs.sh <base_ref> <review_ref> [repo_dir]` で差分本体、変更ファイル、コミット範囲を把握する。ローカル差分なら `git status --short`、`git diff HEAD --stat`、`git diff HEAD` で staged と unstaged の差分を読み、`git ls-files --others --exclude-standard` に出た未追跡ファイルの内容も読む。
4. 変更ファイルだけで結論を出さず、関連する呼び出し元、呼び出し先、共通処理、既存類似処理、docs を `scripts/find_related_usages.sh <keyword> [repo_dir]` や `rg` で確認する。
5. frontend、backend、infra、docs、workflow の境界をまたぐ変更では、API contract、DB schema、環境変数、Docker Compose、Terraform、検証手順の整合性を確認する。
6. `references/review-checklist.md` の必須レビュー観点をすべて確認し、各観点の結果を最終出力に残す。
7. 指摘ごとに `references/severity-levels.md` を使って重大度を `大`、`中`、`小` のいずれかに決める。
8. 最終出力は `references/report-template.md` の 6 セクションを使う。指摘事項を先頭に置き、`大`、`中`、`小` の順に並べる。

## 必須レビュー観点

必ず `references/review-checklist.md` を読み、全観点を確認する。

- 仕様・業務ルール整合性
- 不要コード混入
- 既存挙動への副作用
- 責務分離の妥当性
- バリデーションの妥当性
- 認可・セキュリティ
- DB更新の整合性
- クエリと性能
- 例外処理とログ
- 定数・設定値の扱い
- テストの妥当性
- 外部連携影響
- 可読性・保守性
- 堅牢性
- 既存の類似処理との関連性

## 出力ルール

- 事実と推測を分離し、コードや docs から断定できない箇所は `不明` または `要確認` と書く。
- レビュー結果は必ず `1. レビュー指摘` から始める。
- 指摘には `重大度`、`指摘種別`、`対象ファイル`、`指摘内容`、`影響`、`根拠`、`修正方針` を含める。
- 重大度は `大`、`中`、`小` だけを使い、影響、到達可能性、影響範囲、回避・復旧可能性を根拠に決める。
- 確度が低いことを理由に重大度を下げない。前提が未確定なら `仕様確認待ち` または `要確認` と明記し、その前提が成立した場合の影響で重み付けする。
- 好みだけの指摘や、実害を説明できないスタイル上の提案はレビュー指摘に含めない。
- `指摘種別` は `機能不全`、`UI整合性・可視性/誤認リスク`、`性能・運用負荷`、`保守性・設計負債`、`仕様確認待ち` など、実害の性質が分かる分類にする。
- `機能不全` ではない指摘を、ロジック破壊のように誇張しない。実害の種類を具体的に書く。
- 指摘がない場合だけ `指摘事項なし` と明記し、残リスクや未確認範囲を書く。
- `3. 確認した観点` には必須レビュー観点をすべて含める。
- 日本語で簡潔に書く。

## 参照ファイル

- `references/review-checklist.md`: 必須レビュー観点の詳細
- `references/severity-levels.md`: 重大度「大・中・小」の判定基準
- `references/report-template.md`: 固定出力フォーマット
- `scripts/compare_refs.sh`: branch/ref 比較の補助
- `scripts/find_related_usages.sh`: 関連実装探索の補助
