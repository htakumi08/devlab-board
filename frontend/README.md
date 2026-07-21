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

- まずは `App.tsx` に 1 画面だけ置く
- component、hook、API client は必要になった時点で追加する
- バックエンドとの接続は、最初の API 契約が決まってから戻す
