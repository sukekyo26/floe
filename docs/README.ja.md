# floe

[English README](../README.md)

エージェントのためのローカルのページサーバーです。Go の単一バイナリで配布します。
エージェントはページをデータ（YAML）で書き、floe がテンプレートで描きます。HTML のファイルは
リンクしてそのまま表示します。どちらも `http://127.0.0.1:7325` で配信し、ライブリロードと管理 UI を提供します。図は宣言的な定義から
インライン SVG として描画し、描画前にはみ出し・重なり・線の交差を検査します。

> **ステータス:** 開発中です。組み込みの `report` テンプレートでのデータファイルの公開、HTML のリンク、図の描画（`floe diagram render`。AWS アイコンは `floe icons fetch aws` で取得）、データファイルの検査（`floe check`）、自分のテンプレートは使えます。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## クイックスタート

```sh
floe publish plan.yaml     # データを保存し、ページの URL を出してブラウザで開く
floe publish --link out/index.html   # HTML のファイルをそのまま表示する（リクエストのたびに読む）
floe check plan.yaml       # 公開せずにデータファイルを検査する
floe list                  # ページを新しい順に出す
floe stop                  # 裏で動いているサーバーを止める
```

サーバーは最初の公開で起動し、`floe stop` で止めるまで動き続けます。
管理画面は `http://127.0.0.1:7325` で開けます。

## 多言語化

コマンドの出力は、環境の言語によらず英語です（読むのはエージェントなので）。ブラウザに出る画面
（管理画面と、データファイルの誤りのページ）は、ブラウザの言語に従って英語か日本語で出ます。

## ドキュメント

| 内容 | English | 日本語 |
|---|---|---|
| コマンドと環境変数 | [commands.md](commands.md) | [commands.ja.md](commands.ja.md) |
| テンプレートの書き方 | [templates.md](templates.md) | [templates.ja.md](templates.ja.md) |
| 変更履歴 | [CHANGELOG.md](../CHANGELOG.md) | [CHANGELOG.ja.md](CHANGELOG.ja.md) |

## ライセンス

[MIT](../LICENSE)
