---
name: code-review
description: PRレビュー、ブランチ差分レビュー、自己レビュー、サブエージェント成果物の確認に使う。
---

# コードレビュープレイブック

## 見る順番

PR、ブランチ差分、複数領域にまたがる差分、または固定フォーマットのレビュー結果が必要な場合は、先に `.agents/skills/pr-review/SKILL.md` を読む。

PRレビューでは、可能な範囲で PR 目的、base/head、変更ファイル一覧、未解決 review comment、CI/checks、関連 Issue または docs を確認する。

GitHub PR URLとレビュー依頼を受けた場合は、`pr-review` skillのGitHub PRレビュー・投稿モードを使う。対象PRを明示し、reviewed base/head SHAを固定して、既定では `COMMENT` レビューを投稿する。投稿しない指定がある場合はGitHubへ書き込まない。

1. 依頼された挙動と差分が一致しているか。
2. API contract、DB schema、AWS 構成、docs に矛盾がないか。
3. 既存の README、AGENTS、workflow ルールと衝突していないか。
4. security、secret、logging、public exposure の問題がないか。
5. テストまたは検証が変更リスクに見合っているか。
6. 不要な refactor、generated diff、local dependency が混ざっていないか。
7. `pr-review` skill を使う場合は、必須レビュー観点をすべて確認し、`.agents/skills/pr-review/references/severity-levels.md` で重大度を判定して、`.agents/skills/pr-review/references/report-template.md` の6セクションで出力する。

## 指摘の書き方

- findings を重大度 `大`、`中`、`小` の順に出す。
- 重大度は修正工数ではなく実害で判定し、好みだけの提案は指摘にしない。
- file path と line を具体的に示す。
- 挙動への影響を説明する。
- 修正方針は短く具体的に書く。
- 問題がない場合は、残っている test gap または residual risk を明示する。
- PRレビューでは `重大度` / `指摘種別` / `対象ファイル` / `指摘内容` / `影響` / `根拠` / `修正方針` を含む表を先頭に置く。

## サブエージェント成果物の確認

- 担当 path 以外を触っていないか。
- 他の作業者の変更を revert していないか。
- 実行した検証が報告と一致しているか。
- 最終統合時に README、docs、workflow の参照が壊れていないか。
