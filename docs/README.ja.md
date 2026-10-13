# floe

[English README](../README.md)

エージェントのためのローカルのページサーバーです。Go の単一バイナリで配布します。
エージェントはページをデータ（YAML）で書き、floe がテンプレートで描きます。HTML のファイルは
リンクしてそのまま表示します。どちらも `http://127.0.0.1:7325` で配信し、ライブリロードと管理 UI を提供します。図は宣言的な定義から
インライン SVG として描画し、描画前にはみ出し・重なり・線の交差を検査します。

> **ステータス:** 開発中です。組み込みの `report` テンプレートでのデータファイルの公開、HTML のリンク、図の描画（`floe diagram render`。AWS アイコンは `floe icons fetch aws` で取得）は使えます。データファイルの検査と自分のテンプレートは予定で、まだありません。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## クイックスタート

```sh
floe publish plan.yaml     # データからページを描いて保存し、URL を出してブラウザで開く
floe publish --link out/index.html   # HTML のファイルをそのまま表示する（リクエストのたびに読む）
floe check plan.yaml       # 公開せずにデータファイルを検査する
floe list                  # ページを新しい順に出す
floe stop                  # 裏で動いているサーバーを止める
```

サーバーは最初の公開で起動し、`floe stop` で止めるまで動き続けます。
管理画面は `http://127.0.0.1:7325` で開けます。

## 多言語化

メッセージは英語と日本語で出せます。コマンドは `FLOE_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG`
の順に見て言語を決め、管理画面はブラウザの言語に従います。既定は英語です。

## ドキュメント

| 内容 | English | 日本語 |
|---|---|---|
| コマンドと環境変数 | [commands.md](commands.md) | [commands.ja.md](commands.ja.md) |
| 変更履歴 | [CHANGELOG.md](../CHANGELOG.md) | [CHANGELOG.ja.md](CHANGELOG.ja.md) |

## ライセンス

[MIT](../LICENSE)
