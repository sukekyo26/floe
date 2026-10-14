# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- データファイルの中にスクリプトを書けるようにした。組み込みの `report` は部品 `script` を、テンプレートは `"format": "script"` の項目を受け取る。`js` に関数を書き、ページはその場所の空の要素と、任意の値の `data` を渡して呼ぶ。`</script` か `<!--` を含む `js` は、その `js` を指して断る。
- データファイルに表・カード・注記を書けるようにした。組み込みの `report` は部品 `table`（`columns` と `rows`。`num` の列は右に揃える。セルは 1 行の Markdown で、数は引用符で囲む。`{ pill, tone }` で状態の札も書ける。各行のセルは列の数だけ書く）・`cards`（`title` と Markdown の `text` の並び）・`note`（Markdown の `text` と、`warn` か `bad` の `tone`）を受け取る。
  - テンプレートの `page.html` から `markdownLine` を使える。1 行の Markdown を段落で包まずに描き、`-` や `1.` のようなセルは文字のまま出す。
- データファイルの中に図を書けるようにした。組み込みの `report` は部品 `diagram` を、テンプレートは `"format": "diagram"` の項目を受け取り、`floe diagram render` に渡すのと同じ spec をその場に書く。
  - 図は `floe diagram render` と同じく検査し（id の重複はページ全体で）、誤りはすべてデータファイルの行と列で出す。
  - ページには図のスタイルとズームが 1 回だけ付く。
  - `floe template check` は、図が色を取る CSS 変数を定義していないテーマを警告する。
- 絶対座標に部品を置いた YAML の spec から図を描き、インライン SVG の HTML 断片にして出す `floe diagram render <spec.yaml>...` を追加。
  - 描く前に、収まらない文字・キャンバスからのはみ出し・重なり・箱やラベルを通る線・線の交差を検査し、誤りをすべて `ファイル:行:列` の形で出す。
  - 断片には図のスタイルと、ページの言語で書いたズームのツールバー（ボタン・Ctrl / ⌘ + ホイール・ピンチ・ドラッグ・矢印キー）が付く。`--no-assets` で外せる。
- アイコン集（`aws`: 公式の Icon package の AWS Architecture Icons）を `FLOE_ICONS_DIR`（既定は `~/.local/share/floe-icons`）に取得する `floe icons fetch <set> [zip-or-url]` と、その名前を探す `floe icons search <word>...` を追加。
  - 図の `icon` 部品と frame の `icon` がこのアイコンを描く。ダーク用の版があれば配色に合わせて切り替わる。
  - 知らない名前には近い名前を示す。
- 組み込みの例の spec（構成図・AWS 構成図・クラス図・ER 図・判定フロー・Git・シーケンス図・状態遷移図・タイムライン・ベン図）を一覧し、1 つを出す `floe diagram example [kind]` を追加。
  - 例は英語版と日本語版があり、メッセージの言語に従う。`--lang` で別の言語を選べる。
- データファイルからページを描いて保存し、`http://127.0.0.1:7325` の URL を出す `floe publish <data.yaml>` を追加。
  - YAML のファイルがテンプレートを指定し、テンプレートがそのデータからページを描く。
  - 組み込みのテンプレート `report` は、見出しと Markdown の文章の節を並べる。Markdown で書けないものは HTML の部品で書ける。
  - 描く前にデータをテンプレートのスキーマで検査し、誤りをすべて行と列付き（`ファイル:行:列`）で出す。
  - データファイルから描いたページのスクリプトは、ページの中身を外へ送れない。CSP で `fetch`・XHR・ほかの場所の画像・フォームを止め、ダウンロードしたものも同じく止める。
  - ページの言語は、データファイルの `lang:`（`en`・`ja` のような BCP 47 の言語タグ）で決まる。どのデータファイルにも必要。
  - サーバーが動いていなければ裏で起動し、新しいページはブラウザで開く。
  - HTML のファイルは保存しない。`--link` でその場所のまま登録し、そのまま表示する。そのディレクトリの配下も配信する。
  - `--link` で登録したデータファイルは、保存せずにページを開くたびに描く。データファイル・ページに出す画像・テンプレートのファイルのどれかを保存するか、図に出すアイコンを取得すると、開いているタブが再読み込みする。誤りがあるあいだは、ページの代わりに誤りの一覧を出す。
  - `--theme` でテンプレートの別のテーマを選べる。ほかに `--slug`・`--title`・`--description`・`--force` を使える。
  - 同じ slug で公開し直すと、そのことを stderr に出す。タイトルが変わっていれば警告も出す。
- 自分のテンプレートを使えるようにした。`FLOE_TEMPLATES_DIR`（既定は `~/.config/floe/templates`）に置いたディレクトリ（symlink でもよい）が、同じ名前の組み込みのテンプレートの代わりに使われる。
  - `page.html`・`schema.json`・テーマのほか、`checks.yaml` で項目どうしの関係を（リストの要素ごとに。`sections.parts` のように途中のリストも辿る）検査し、`computed.yaml` でページに出す値を計算できる。式は [expr](https://expr-lang.org/) で書き、誤りはデータファイルの行と列で、これらのファイルそのものの誤りはそのファイルの行と列で出す。`page.html` は `num` と `bytes` で数を書ける。
  - 画像は data URI としてページに入る。`"format": "image"` の項目にはデータファイルのディレクトリの配下の画像を書き、`page.html` はテンプレートの `assets/` の画像を `asset` で出す。画像は 1 MiB までの png・jpeg・gif・webp・svg で、名前と中身の両方で判定し、データの画像の誤りは行と列で出す。
  - `template.yaml` に、テンプレートが要る floe の版を書く。書き方は [docs/templates.ja.md](templates.ja.md) にある。
  - スキーマは、floe がどのデータファイルからも自分で読む `lang`・`template`・`theme`・`title` を宣言したり必須に挙げたりできない。
- テンプレートを置き場所・テーマ・要る floe の版とともに一覧する `floe template list`、データファイルに書ける項目と見本を出す `floe template show <name>`（データファイルを書く前に読む）、テンプレートを写して自分のテンプレートを作る `floe template new <name> --from <template>`、テンプレートを使う前や配る前に検査する `floe template check <dir>...` を追加。`template check` はファイル（floe が知らない `format` や、`schema.json` で宣言していない `checks.yaml` のパスなど）を検査してから、`example.yaml` を公開のすべての段に通す。
- データファイルを `floe publish` と同じく検査し、誤りをすべて `ファイル:行:列` で出す `floe check <data.yaml>...` を追加。公開もサーバーの起動もしないので、データファイルを置いたリポジトリの CI から使える。
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
