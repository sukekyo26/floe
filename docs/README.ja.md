# floe

[English README](../README.md)

エージェントが書いた HTML ページを配信するローカルサーバーです。Go の単一バイナリで配布します。
エージェントがページ（またはテンプレートで描画する構造化データ）を公開すると、floe が
`http://127.0.0.1:7325` で配信し、ライブリロードと管理 UI を提供します。図は宣言的な定義から
インライン SVG として描画し、描画前にはみ出し・重なり・線の交差を検査します。

> **ステータス:** 開発中です。ページの公開と配信は使えます。テンプレートと図はまだありません。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## 使い方

```sh
floe publish report.html          # ページを保存して URL を出す
floe publish notes.html --lang ja # 本文だけの HTML は英語の雛形で包む。--lang で言語を変えられる
floe publish docs/index.html --link  # ファイルをその場所のまま、ディレクトリごと配信する
floe list                         # ページを新しい順に出す
floe rm report                    # ページを消す（リンク先のファイルは残る）
floe stop                         # 裏で動いているサーバーを止める
```

サーバーは最初の公開で起動し、リクエストが `FLOE_IDLE_MINUTES`（既定は 30）分なければ終了します。
ページは `FLOE_DIR`（既定は `~/.local/share/floe`）に保存し、`FLOE_PORT` でポート（既定は 7325）を変えられます。

## ライセンス

[MIT](../LICENSE)
