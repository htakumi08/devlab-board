---
name: frontend-release
description: フロントエンドを S3 + CloudFront へ配信する準備や確認に使う。
---

# フロントエンドリリースプレイブック

1. `frontend/README.md` と `codex-workflow/rules/frontend-react.md` を確認する。
2. API base URL、public config、secret 混入の有無を確認する。
3. `yarn build` を実行する。
4. build output、hashed assets、`index.html` の cache 方針を確認する。
5. S3 sync、CloudFront invalidation、rollback path を手順化する。
6. 公開 route、SPA fallback、API 接続、error state の smoke check を用意する。
7. deploy 後の確認結果と残リスクを報告する。

`terraform apply` や本番向け deploy は、ユーザーの明示的な依頼がある場合だけ実行する。
