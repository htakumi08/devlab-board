---
name: go-http-api
description: Go 標準ライブラリ中心の HTTP API endpoint を追加・変更するときに使う。
---

# Go HTTP API Skill

## 事前確認

- `backend/AGENTS.md` と `backend/README.md` を読む。
- route、handler、request、response、status code、error shape を確認する。
- frontend が期待する API contract を確認する。
- DB、logging、AWS 連携のどれに触れる変更か確認する。

## 実装方針

- `cmd/` は起動責務に保つ。
- handler は薄くし、validation、business rules、DB access を分ける。
- request context を external call や DB call へ渡せる形にする。
- response writer への書き込みは helper で揃える。
- 失敗時の log と client response は分ける。
- endpoint には `httptest` の unit test を追加する。

## 検証

- `go test ./...` を実行する。
- 必要に応じて `go test ./cmd/api -run TestName` で対象テストを確認する。
- API contract が安定したら `docs/api/` または `docs/implementation/` に記録する。
