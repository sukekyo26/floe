# floe

[English README](../README.md)

エージェントが書いた HTML ページを配信するローカルサーバーです。Go の単一バイナリで配布します。
エージェントがページ（またはテンプレートで描画する構造化データ）を公開すると、floe が
`http://127.0.0.1:7325` で配信し、ライブリロードと管理 UI を提供します。図は宣言的な定義から
インライン SVG として描画し、描画前にはみ出し・重なり・線の交差を検査します。

> **ステータス:** 開発中です。現時点で使えるのは `floe version` だけです。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## ライセンス

[MIT](../LICENSE)
