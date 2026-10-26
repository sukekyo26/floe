# floe

[English README](../README.md)

エージェントのためのローカルのページサーバーです。Go の単一バイナリで配布します。
エージェントはページをデータ（YAML）で書き、floe がテンプレートで描きます。HTML のファイルは
リンクしてそのまま表示します。どちらも `http://127.0.0.1:7325` で配信します。

> **ステータス:** 作り直し中です。今使えるのは、データファイルを描く `floe check`・`floe render`
> （floe の部品・テーマ・report と、`FLOE_PARTS_DIR`・`FLOE_THEMES_DIR`・`FLOE_TEMPLATES_DIR`、
> 無ければ `~/.config/floe/parts`・`themes`・`templates` の自分のもの）、自分のものを確かめる
> `floe part check`・`floe template check`、版を出す `floe version` です。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## ライセンス

[MIT](../LICENSE)
