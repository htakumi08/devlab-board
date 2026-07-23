# Frontend

このディレクトリは、React を一つずつ学びながら育てるための最小フロントエンドである。

## 現在の構成

```text
frontend/
  AGENTS.md
  Dockerfile
  README.md
  index.html
  package.json
  tsconfig.json
  vite.config.ts
  yarn.lock
  src/
    App.tsx
    features/
      user-agent/
        UserAgentLab.tsx
        api.ts
    lib/
      api.ts
    main.tsx
    style.css
```

## package manager

このディレクトリの Node.js 系操作は `yarn` で統一する。

- install: `yarn install`
- dev: `yarn dev`
- build: `yarn build`
- preview: `yarn preview`

## この段階での方針

- `/` には Overview と Session を表示する
- 実験機能は `/user-agent-lab` のように機能ごとの URL と画面を持たせる
- サイドバーは各画面で共通表示し、メイン領域を React Router で切り替える
- component、hook、API client は必要になった時点で追加する
- バックエンドとの接続は、最初の API 契約が決まってから戻す
- S3 + CloudFront 配信では、各 URL への直接アクセスを `index.html` に戻す SPA fallback を設定する

## 状態管理

状態の置き場所は、次の順で判断する。

| 状態 | 手段 | 対象 |
| --- | --- | --- |
| ローカル状態 | `useState` | 入力値や開閉状態など、component 内で完結する状態 |
| グローバル状態 | Jotai の Atom | 複数の component や画面で共有する、サーバー由来ではない状態 |
| サーバー状態 | TanStack Query | API データの取得、キャッシュ、再取得、loading / error |

- まず `useState` を使い、共有が必要になった時点で Atom を検討する
- API データは TanStack Query を正とし、Atom や `useState` へ重複保存しない
- Jotai と TanStack Query は、利用する機能を実装するときに導入する
