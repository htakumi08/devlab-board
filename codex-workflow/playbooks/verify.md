---
name: verify
description: 実装、docs、workflow 変更後の確認に使う。
---

# 検証プレイブック

1. 変更した領域を backend、frontend、infra、docs、workflow に分ける。
2. 変更領域ごとの最小検証を選ぶ。
3. 実行したコマンド、結果、失敗時の理由を記録する。
4. 実行できなかった検証は、理由と残リスクを明示する。
5. `git status --short` で想定外の差分がないか確認する。

## 既定コマンド

- Backend: `go test ./...`
- Frontend: `yarn build`
- Terraform: `terraform fmt -check -recursive`、対象 env の `terraform validate`
- Docs/workflow: file tree、相対 path、リンク、見出し、参照先の存在確認

## 報告に含めるもの

- 実行したコマンド
- 成功/失敗の結果
- スキップした確認と理由
- 残っているリスク
