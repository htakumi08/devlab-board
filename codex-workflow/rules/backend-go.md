# Backend Go ルール

## 構成

- backend は Go HTTP API とする。
- 現段階では標準ライブラリ中心で進め、必要になるまで framework を増やさない。
- `cmd/` は起動責務だけに寄せる。
- handler、use case、repository、platform integration は、必要になった時点で `internal/` に分ける。
- DB、logging、storage、queue、AWS SDK 連携は handler へ直接散らさない。

## API

- endpoint ごとに request、response、status code、error shape を明確にする。
- JSON response は content type と status code を先に決める。
- frontend と共有する API contract は、安定したら `docs/` に request / response example として残す。
- health check は ALB、ECS、Lambda adapter など将来の実行環境差分を意識して軽く保つ。

## 実装

- request context を捨てず、DB や外部連携へ渡せる形にする。
- error は握りつぶさず、log と client response の境界を分ける。
- secrets、raw credentials、不要な personal data を log に出さない。
- handler の unit test は `httptest` を基本にする。
- DB を導入するときは migration、接続設定、test data の置き場を docs と合わせる。

## コメント / Docstring

- exported function、type、package には、人間が責務を追いやすいよう必要に応じてコメントを付ける。
- 認可、状態遷移、idempotency、transaction、context timeout など、読み違えやすい箇所は意図を短く残す。
- Go の文法学習メモは、長くなる場合はコードコメントではなく docs または学習メモへ移す。
- 自明な代入、単純な mux 登録、1 行ごとの言い換えコメントは避ける。
