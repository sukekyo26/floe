# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- 絶対座標に部品を置いた YAML の spec から図を描き、インライン SVG の HTML 断片にして出す `floe diagram render <spec.yaml>...` を追加。
  - 描く前に、収まらない文字・キャンバスからのはみ出し・重なり・箱やラベルを通る線・線の交差を検査し、誤りをすべて `ファイル:行:列` の形で出す。
  - 断片には図のスタイルと、ページの言語で書いたズームのツールバー（ボタン・Ctrl / ⌘ + ホイール・ピンチ・ドラッグ・矢印キー）が付く。`--no-assets` で外せる。
- 組み込みの例の spec（構成図・クラス図・ER 図・判定フロー・Git・シーケンス図・状態遷移図・タイムライン・ベン図）を一覧し、1 つを出す `floe diagram example [kind]` を追加。
  - 例は英語版と日本語版があり、メッセージの言語に従う。`--lang` で別の言語を選べる。
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
