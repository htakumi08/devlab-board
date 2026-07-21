# Documentation ルール

## 目的

docs は、将来の人間と Codex が短時間で現在地へ戻るための作業台です。実装の代わりに長い説明を増やす場所ではありません。

## 置き場所

- `docs/app-service.md`: アプリ目的、MVP、サービス方針。
- `docs/db-design.md` / `docs/db-design.dbml`: データ設計。
- `docs/implementation/`: 実装後の方針、追加ファイル、検証結果、割り切り。
- `docs/api/`: API contract や request / response draft。
- `docs/runbooks/`: release、rollback、incident、AWS 操作手順。
- `docs/adr/`: 安定した設計決定。

未作成のディレクトリは、必要になった時点で追加する。

## 作成・更新

- 参照元、目的、読者を先に決める。
- 既存 docs と矛盾する場合は、どちらを更新するかを明示する。
- 長くなる docs には、更新履歴、目次、要確認事項を置く。
- 複雑な処理、状態遷移、AWS 構成、データフローは Mermaid または text 図を検討する。
- 未決定事項は本文に混ぜず、要確認として最後に集める。

## 禁止

- 実コードと違う理想構成を、確定済みのように書かない。
- 一時的な会話ログや個人環境の絶対 path を commit する docs に残さない。
- secrets、tokens、credentials、private URLs を docs に置かない。
