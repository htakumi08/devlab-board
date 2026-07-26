#frontendについて

状態管理の戦略ってどこかに記載されていますか？

ローカル状態管理: Frontend React ルール

## 構成

- frontend は Vite + React + TypeScript を使う。
- Node 系操作は `frontend/README.md` に合わせて `yarn` で統一する。
- app が成長したら feature-oriented folder を優先する。
- API client、type、data transformation は画面ごとに重複させず境界へ寄せる。
- component、hook、route、lib は、必要になった時点で分ける。

## 静的配信

- build output は S3 + CloudFront static hosting で安全に配信できる状態にする。
- `VITE_` で公開してよい値だけを bundle する。
- secret、private endpoint、raw credential を frontend に入れない。
- 長い cache lifetime には hashed assets を使う。
- `index.html` と runtime config の cache behavior を明確にする。
- direct deep link のための SPA fallback behavior を計画する。

## 実装

- shape が不確かな API data は boundary で扱い、画面内で前提を散らさない。
- loading、empty、error、offline、permission state を必要な範囲で用意する。
- 複数画面で繰り返し必要になるまで、広範な global state は避ける。
- label、keyboard operation、focus、semantic controls など accessibility basics を保つ。
- UI は学習の邪魔になる過剰な装飾を避け、ダッシュボードとして情報を読み取りやすくする。

## 状態管理

- component 内で完結する状態は `useState` を使う。
- 複数の component や画面で共有する client state は Jotai の Atom を使う。
- API 由来の server state は TanStack Query を使う。
- server state を Atom や `useState` へ重複保存しない。
- 実装前に各状態を分類し、必要になるまで Jotai と TanStack Query を追加しない。

## コメント / Docstring

- component、custom hook、utility の exported API は、人間が責務を追いやすいよう必要に応じて docstring を付ける。
- 画面状態の分岐、非同期の競合回避、UI 制約、API response を画面向けへ整形する箇所は、意図を短いコメントで残す。
- JSX の見た目を言い換えるだけのコメントや、単純な state 更新の説明は避ける。
