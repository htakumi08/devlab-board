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
      finance/
        FinanceLayout.tsx
        FinanceOverview.tsx
        FinanceAccounts.tsx
        FinanceAccountDetail.tsx
        FinanceTransactions.tsx
        financeApi.ts
      user-agent/
        UserAgentLab.tsx
        api.ts
    lib/
      api.ts
      queryClient.ts
    main.tsx
    style.css
```

## package manager

このディレクトリの Node.js 系操作は `yarn` で統一する。

- install: `yarn install`
- dev: `yarn dev`
- build: `yarn build`
- preview: `yarn preview`
- test: `yarn test`
- coverage: `yarn test:coverage`

## この段階での方針

- `/` には Overview と Session を表示する
- 実験機能は `/user-agent-lab` のように機能ごとの URL と画面を持たせる
- `/finance-lab`、`/finance-lab/accounts`、`/finance-lab/accounts/:accountId`、`/finance-lab/transactions` はFinance専用layoutを使う
- サイドバーは各画面で共通表示し、メイン領域を React Router で切り替える
- サイドバーの開閉状態は `useState` で管理し、ページ再読み込み時は開いた状態に戻す
- component、hook、API client は必要になった時点で追加する
- バックエンドとの接続は、最初の API 契約が決まってから戻す
- S3 + CloudFront 配信では、各 URL への直接アクセスを `index.html` に戻す SPA fallback を設定する
- localのViteは、Browserの `localhost` に加えてDocker内smokeから使うservice名 `frontend` だけを許可する

## 状態管理

状態の置き場所は、次の順で判断する。

| 状態 | 手段 | 対象 |
| --- | --- | --- |
| ローカル状態 | `useState` | 入力値や開閉状態など、component 内で完結する状態 |
| グローバル状態 | Jotai の Atom | 複数の component や画面で共有する、サーバー由来ではない状態 |
| サーバー状態 | TanStack Query | API データの取得、キャッシュ、再取得、loading / error |

- まず `useState` を使い、共有が必要になった時点で Atom を検討する
- API データは TanStack Query を正とし、Atom や `useState` へ重複保存しない
- Financeのserver stateはTanStack Queryを使い、query keyをFinance API boundaryへ集約する
- 取引履歴のcursorはURL queryを正とし、次ページ移動、直接URL、ブラウザの戻る・進むで同じページを復元する
- 取引履歴の口座、期間、カテゴリー、入出金、状態、並び順もURL queryを正とし、条件変更時はcursorだけを破棄する
- 取引履歴の直接URLに未知key、重複key、空値がある場合はAPIへ縮約して送らず、不正条件のリセット状態を表示する
- 取引カテゴリー候補はDB masterを正として`GET /api/finance/categories`から取得し、frontendへ固定値を重複させない
- logoutまたはFinance APIの401では、進行中のFinance queryを中断し、Finance prefixのcacheを破棄する
- Jotai は共有client stateが必要になるまで導入しない
