# floe

[English README](../README.md)

エージェントのためのローカルのページサーバーです。Go の単一バイナリで配布します。
エージェントはページをデータ（YAML）で書き、floe がテンプレートで描きます。HTML のファイルは
リンクしてそのまま表示します。どちらも `http://127.0.0.1:7325` で、自動で再読み込みし、
管理ページとともに配信します。図は spec から inline SVG で描き、描く前に、字のはみ出し・重なり・
線の交差を確かめます。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

`FLOE_VERSION` でリリースを固定し、`FLOE_INSTALL_DIR` でインストール先（既定は
`~/.local/bin`）を変えられます。バイナリはリリースの `SHA256SUMS` で検証します。

## はじめる

```sh
floe template list          # floe のテンプレート（report・design・handoff・adr）と自分のもの
floe template show report   # report のデータファイルに書けるものと、見本
floe check plan.yaml        # データファイルを確かめる（誤りを出すか、何も出さない）
floe publish plan.yaml      # ページを保存し、URL を出して開く
floe publish --link out/index.html   # HTML のファイルをそのまま見せる
floe list                   # ページの一覧（新しい順）
floe archive old-plan       # ページを消さずに一覧から外す
floe stop                   # 裏で動くサーバーを止める
```

サーバーは最初の publish で起動し、`floe stop` まで動き続けます。
`http://127.0.0.1:7325` を開くと管理ページです。

## ドキュメント

| したいこと | 読む | English |
|---|---|---|
| ページを作る（データファイルを書く） | [pages.ja.md](pages.ja.md) | [pages.md](pages.md) |
| 自分のテンプレートを作る | [templates.ja.md](templates.ja.md) | [templates.md](templates.md) |
| 自分の部品・テーマを作る | [parts.ja.md](parts.ja.md) | [parts.md](parts.md) |
| コマンド・設定を調べる | [commands.ja.md](commands.ja.md) | [commands.md](commands.md) |
| 変わったことを見る | [CHANGELOG.ja.md](CHANGELOG.ja.md) | [CHANGELOG.md](../CHANGELOG.md) |

## 言語

コマンドラインは、環境の言語によらず英語で話します（エージェントが読むため）。
ブラウザが見せるもの（管理ページと、データファイルの誤りのページ）は、ブラウザの言語で英語か日本語になります。

## 開発

`just ci` で CI と同じものを走らせます。ブラウザのテストは Chrome を `FLOE_CHROME`、無ければ `PATH` で探し、見つからなければ飛ばします。

## ライセンス

[MIT](../LICENSE)
