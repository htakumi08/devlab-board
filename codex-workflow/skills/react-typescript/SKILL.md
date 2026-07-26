---
name: react-typescript
description: Vite + React + TypeScript の画面、component、API client を追加・変更するときに使う。
---

# React TypeScript Skill

## 事前確認

- `frontend/AGENTS.md` と `frontend/README.md` を読む。
- target user flow と API boundary を特定する。
- 実ファイルから build tool と package manager を確認する。
- backend が未実装の場合は stub の範囲を明示する。

## 実装方針

- components は focused かつ typed に保つ。
- API response の前提は画面ごとに散らさず boundary へ寄せる。
- local state は `useState`、共有する client state は Jotai の Atom、server state は TanStack Query を使う。
- server state を Atom や `useState` へ重複保存しない。
- cross-route state が必要になるまで Jotai を導入しない。
- API data の取得を実装する段階で TanStack Query を導入する。
- loading、empty、error、offline state を必要な範囲で扱う。
- static build constraints と public env vars を意識する。
- accessibility basics を保つ。

## 検証

- `yarn build` を実行する。
- lint/test scripts が存在する場合は対象変更に合わせて実行する。
- UI が重要な変更では、browser または screenshot で layout を確認する。
