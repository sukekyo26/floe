# floe

[English README](../README.md)

エージェントのためのローカルのページサーバーです。Go の単一バイナリで配布します。
エージェントはページをデータ（YAML）で書き、floe がテンプレートで描きます。HTML のファイルは
リンクしてそのまま表示します。どちらも `http://127.0.0.1:7325` で配信します。

> **ステータス:** 作り直し中です。今使えるのは、データファイルを描く `floe check`・`floe render`
> （floe の部品・テーマ・report と、`FLOE_PARTS_DIR`・`FLOE_THEMES_DIR`・`FLOE_TEMPLATES_DIR`、
> 無ければ `~/.config/floe/parts`・`themes`・`templates` の自分のもの）、部品とテンプレートを
> 一覧し、schema を書いたとおりに見せ、いちばん小さい自分のものを作り、自分のものを確かめる
> `floe part list|show|new|check`・`floe template list|show|new|check`、版を出す `floe version`
> です。自分の schema の根には `$id` を書けず、自分のテーマには floe のテーマの名前を付けられません。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## ライセンス

[MIT](../LICENSE)
