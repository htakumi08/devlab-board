---
name: code-review
description: 差分レビュー、自己レビュー、サブエージェント成果物の確認に使う。
---

# コードレビュープレイブック

## 見る順番

1. 依頼された挙動と差分が一致しているか。
2. API contract、DB schema、AWS 構成、docs に矛盾がないか。
3. 既存の README、AGENTS、workflow ルールと衝突していないか。
4. security、secret、logging、public exposure の問題がないか。
5. テストまたは検証が変更リスクに見合っているか。
6. 不要な refactor、generated diff、local dependency が混ざっていないか。

## 指摘の書き方

- findings を重大度順に出す。
- file path と line を具体的に示す。
- 挙動への影響を説明する。
- 修正方針は短く具体的に書く。
- 問題がない場合は、残っている test gap または residual risk を明示する。

## サブエージェント成果物の確認

- 担当 path 以外を触っていないか。
- 他の作業者の変更を revert していないか。
- 実行した検証が報告と一致しているか。
- 最終統合時に README、docs、workflow の参照が壊れていないか。
