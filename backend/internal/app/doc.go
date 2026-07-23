// Package app は、devlab-board バックエンドの HTTP API を構成する機能を提供する。
//
// このパッケージは、実行環境の設定、DB と session store の初期化、
// HTTP routing と認証、ユーザーデータへのアクセスをまとめている。
// cmd/api を起動責務に保ち、HTTP 処理と外部リソースの組み立てを
// 再利用・テストしやすくするために分離している。
package app
