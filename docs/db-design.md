# DB Design

## 1. 目的

この文書は、`devlab-board` で PostgreSQL を使う機能を追加するときの DB 設計メモである。

現時点では旧アプリの DB 設計を引き継がず、必要になった実験機能からテーブルを追加する。

対応する DBML は [db-design.dbml](./db-design.dbml) に置く。

## 2. 初期方針

- DB は PostgreSQL を前提にする
- 実環境では RDS for PostgreSQL を候補にする
- 最初から広いスキーマを作らない
- 1 実験機能 / 1 use case 単位でテーブルを追加する
- API に公開する ID は、必要になった時点で `public_id` を検討する

## 3. 最初に追加する候補

DB 方針は PostgreSQL に統一する。ローカル開発では Docker Compose の PostgreSQL 18 系 service を使い、実環境では RDS for PostgreSQL を候補にする。

次に追加する設計対象は、ログイン機能のための認証基盤とする。認証基盤は `net/http` と Go のセッションライブラリを組み合わせ、セッションの発行、保存、期限切れ、破棄はライブラリに寄せる。

候補テーブル:

- `users`
- `sessions`
- `experiments`
- `experiment_notes`

### 3.1 認証とセッション

- `users` はログインユーザ本体を管理する
- `sessions` はセッションライブラリ用の server-side session store として使う
- セッション Cookie にはライブラリが発行するセッショントークンを入れる
- ログイン中ユーザIDやロール判定に必要な最小情報は、ライブラリの session data に保存する
- `sessions` には `users.id` への外部キーを置かず、SCS PostgreSQL store の標準形に合わせる
- logout、session token renewal、期限切れ cleanup はセッションライブラリの責務に寄せる

この方針では、`sessions` は業務テーブルではなくライブラリ管理テーブルとして扱う。ユーザー別のセッション一覧、全端末ログアウト、ログイン履歴、監査ログが必要になった場合は、`sessions` を直接拡張せず、別テーブルまたは custom session store を検討する。

## 4. 保留事項

- ファイルアップロード
- 非同期ジョブ
- 検索
- ダッシュボード集計
- ユーザー別セッション管理
- ログイン履歴と監査ログ
- remember me
- CSRF / CORS / Cookie 属性の確定

これらは必要になった時点で、アプリ設計と AWS 構成への影響を見ながら追加する。
