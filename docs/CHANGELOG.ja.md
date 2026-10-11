# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- HTML ページを保存して `http://127.0.0.1:7325` の URL を出す `floe publish <file>` を追加。
  - サーバーが動いていなければ裏で起動し、新しいページはブラウザで開き、本文だけの HTML は組み込みの雛形で包む。
  - 雛形の言語は英語で、`--lang ja` のように別の言語を指定できる。
  - `--link` はファイルをその場所のまま登録し、そのディレクトリの配下も配信する。
  - ほかに `--slug`・`--title`・`--description`・`--lang`・`--force` を使える。
  - 同じ slug で公開し直すと、そのことを stderr に出す。タイトルが変わっていれば警告も出す。
- `floe list`・`floe rm`・`floe serve`・`floe stop` と、ライブリロード付きの管理画面を追加。
  - artifact スキルと同じ形式の保存先を読み書きする。
  - サーバーは `floe stop` で止めるまで動き続ける。
  - 設定は `FLOE_DIR`・`FLOE_PORT`（既定は 7325）で行う。
- メッセージを英語と日本語で出すようにした。
  - コマンドは `FLOE_LANG`、`LC_ALL`、`LC_MESSAGES`、`LANG` の順に見て言語を決める。
  - 管理画面はブラウザの言語に従う。
  - 既定は英語。
- `floe` コマンドと、インストール済みの版を表示する `floe version`
  （`floe --version` でも可）を追加。
- linux と macOS（amd64 / arm64）向けのビルド済みバイナリを GitHub Releases で配布し、
  最新のリリース（または `FLOE_VERSION`）を SHA-256 を検証してから
  `FLOE_INSTALL_DIR`（既定は `~/.local/bin`）に入れる `install.sh` を追加。
