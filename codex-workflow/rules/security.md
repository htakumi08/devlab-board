# Security ルール

## 必須確認

- hardcoded secrets、tokens、credentials、private URLs がないこと。
- frontend boundary と backend API boundary の両方で input を扱うこと。
- protected backend actions を追加する場合は authorization 方針を決めること。
- CORS、CSRF/session strategy、auth token storage が必要になったら明示すること。
- logs に raw personal data、credentials、sensitive request payloads を出さないこと。
- error responses が stack trace や internal details を漏らさないこと。

## データ取り扱い

- feature に必要なものだけを保存する。
- anonymize されていない sensitive sample data を tests や docs にコピーしない。
- export/download behavior を追加する場合は、権限と監査観点を確認する。
- DB 接続情報、AWS secret、OAuth secret は secret manager または環境ごとの安全な注入方法を使う。

## レビュー起動条件

次の変更では `roles/security-reviewer.md` または security review を使う。

- 認証、認可、session、token storage
- file upload / download
- DB write、migration、権限境界
- AWS IAM、public endpoint、CORS
- secret、環境変数、logging
