# Project ルール

## 信頼できる情報源

- root `AGENTS.md` が現在の project contract を定義する。
- `README.md` は repository level の目的と現在段階を示す。
- `docs/app-service.md` はアプリの目的、初期 MVP、AWS 構成比較の前提を示す。
- `docs/db-design.md` と `docs/db-design.dbml` はデータ設計の参照元にする。
- 実装状況は code、tests、Docker Compose、Terraform、README から得られる evidence を優先する。
- DB 方針のように docs と実装がずれている場合は、作業前に正とする対象を確認または明示する。

## ドキュメント

開発または運用を支える場合だけ docs を作成または更新する。

- architecture overview
- API contract
- ER/data model
- sequence または data-flow diagrams
- AWS 構成比較
- non-functional requirements
- 安定した決定事項の ADR
- release が始まった後の runbook、rollback、incident notes

docs は事実ベースにし、将来の agent が session 中に読める短さに保つ。

## 変更方針

- 変更は依頼された挙動に対象範囲を絞る。
- app structure が必要とするまで、広範な framework、package、workflow automation は導入しない。
- 学習プロジェクトとして、分かったことが残る小さい変更を優先する。
- 未解決の前提は引き継ぎメモに明示する。

## 可読性

- 実装コードは、agent だけでなく人間の開発者が後から読んでも追える状態を優先する。
- exported function、React component、custom hook、Terraform module、複雑な定数群には、必要に応じて docstring または短い説明コメントを付ける。
- comment は「何をしているか」だけでなく、「なぜその実装にしているか」「どの制約を守っているか」を補足する用途で使う。
- 自明な代入や 1 行ごとの言い換えコメントは避ける。
- 外部制約、セキュリティ前提、状態遷移、データ整形、環境依存の判断は、コードだけで読み取りづらい場合にコメントで明示する。
