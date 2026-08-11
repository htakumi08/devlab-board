# GitHub COMMENTレビュー形式

## Review body

review bodyは `report-template.md` の `## 1. レビュー指摘` から `## 6. 変更概要` までのセクション本体を埋めて使う。テンプレート文書のタイトル、説明、プレースホルダーは投稿しない。

- `## 1. レビュー指摘` から始め、6セクションの見出し名と順序を変えない。
- 指摘表は `根拠` を含む7列を使う。
- `## 3. 確認した観点` は15項目を省略・統合しない。
- repositoryとPR番号、base/head refとSHA、changed files、CI/checksは `## 4. 確認した関連箇所` に記載する。
- 未取得patch、未実行test、確認できない外部依存、残リスクは `## 5. 未確定事項・追加で欲しい情報` に記載する。
- inline commentへ載せた指摘も `## 1. レビュー指摘` の表と指摘要約に含め、GitHubだけを読んでも全体像が分かる本文にする。
- `Codex PRレビュー`、`確認内容`、`未確認範囲・残リスク` など、別形式の見出しを追加しない。

6セクションの後に、次の識別マーカーを追加する。

```html
<!-- codex-pr-review:v1 repo=OWNER/REPO pr=NUMBER base=FULL_SHA head=FULL_SHA -->
```

## Inline comment

```markdown
**重大度: 大 / 中 / 小 — 指摘種別**

問題と、その条件で起きる実害を簡潔に記載する。

根拠: この行と関連処理・仕様との関係。

修正方針: 最小限の直し方。
```

## Inlineにしない指摘

次はreview bodyだけに置く。

- 差分外の既存コードが主対象
- 複数ファイルの組み合わせで発生する設計問題
- test、CI、docs、運用手順全体の不足
- patch上のline/side/positionを確定できない
- 仕様確認待ちで、特定行だけが原因とは断定できない
