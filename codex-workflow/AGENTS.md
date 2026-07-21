# codex-workflow 運用ガイド

このディレクトリは、Codex が devlab-board で自立的に作業するための運用レイヤーです。アプリケーションコードや再利用パッケージではなく、判断基準、作業手順、サブエージェント分担、反復作業用 skill を管理します。

## 読み方

1. まず `../AGENTS.md` でプロジェクト全体の制約を確認する。
2. `README.md` で workflow の入口と読み順を確認する。
3. タスクに関係する `rules/` と `playbooks/` だけを読む。
4. サブエージェントを使う場合だけ `roles/` を読む。
5. 反復作業として安定している場合だけ `skills/` を読む。

## メンテナンス方針

- workflow は短く保ち、実際に繰り返し使うルールだけを残す。
- 新しいファイルを増やす前に、既存の rule / playbook / role / skill の更新で足りるか確認する。
- 技術スタックに合わない内容、関係のない generated assets、local dependency folders、個人環境のログは置かない。
- 実コード、README、docs と矛盾が出た場合は、実コードとユーザーの最新要望を優先して workflow を更新する。
- 安定した運用習慣だけを `skills/` に昇格する。単発作業のメモは `docs/implementation/` を使う。

## サブエージェント運用

- 司令塔は、作業全体の目的、影響範囲、検証方針、最終統合を持つ。
- サブエージェントには、触ってよい path と出力形式を明確に渡す。
- 複数エージェントを使う場合、同じファイルを同時に編集させない。
- 返却内容はそのまま採用せず、司令塔が repo 状態と差分を確認して統合する。

## 完了条件

- `README.md` の構成図や読み順が実際のファイル構成と一致している。
- 新しい rule / playbook / role / skill を追加した場合、入口から辿れる。
- docs と workflow のどちらに置くべき内容かが分かれている。
- workflow-only changes では、少なくとも file tree、相対 path、`git status --short` を確認する。
