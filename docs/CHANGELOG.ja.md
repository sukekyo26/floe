# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- 追加：`code`・`terminal`・`diff` の右上に、中身を写すボタン。code は書いたまま、
  terminal はプロンプトと出力を除いたコマンドだけ、diff は書いたままを写し、
  写したことを示す。長い行は、右端までスクロールすると、終わりがボタンの左に出る。
- 追加：警告。ページのより良い書き方の勧めで、`floe check`・`floe render`・
  `floe template check`・`floe publish` が、誤りの後に
  `file:line:column: warning: message` で出す。ページは描き、終了コードも
  変えない。最初の警告は、`text` の部品（節の中、`details`・`tabs` の中も）の
  囲んだコードのブロックで、ただのコードとして描き、`code` か `terminal` に
  書くよう勧める。`text` は部品の 1 つになったので、自分の部品に `text` という
  名前があると断る。
- 追加：図の部品 `tree`。木の節と子だけを書けば、floe が各節を親の下に置き、
  線を引く。あわせて図の例 `org`（`floe diagram example org`）。ある会社の
  部署を `tree` で描く。
- 追加：図の例 `gantt`（`floe diagram example gantt`）。作業の期間を棒で、
  待つつながりを矢印で描く図で、あるアプリの 1 つの版の 6 週間の予定を置く。
- 追加：図の例 `quadrant`（`floe diagram example quadrant`）。2 つの軸で項目を
  4 つの領域に分ける図で、ノートのアプリの機能の候補を、価値と手間で置く。
- 追加：図の例 `swimlane`（`floe diagram example swimlane`）。流れを、する人ごとの
  レーンに分けた図で、ユーザー・アーキテクト・エンジニアの 1 つの PR の流れを描く。
- 追加：`code`・`terminal`・`diff` の右上に、中身をコピーするボタン。写すのはコード、
  プロンプトと出力を除いたコマンド、書いたままの diff。押すとページの言語で「コピーしました」
  と言い、ページがクリップボードに書けない所（VS Code の Simple Browser など）でも写す。
- 追加：アーカイブ。`floe archive <slug>...`（`--undo` で戻す）、管理ページの
  「アーカイブ」（1 枚でも、選んだものまとめてでも）、`PATCH /api/artifacts/{slug}` の
  `archived` で、ページを消さずに `floe list` と管理ページの一覧から外す。URL では今までどおり
  開ける。管理ページは棚の「アーカイブ」で、`floe list --archived` は 5 列目を付けて、アーカイブも
  出す。アーカイブしたページを publish し直すと、一覧に戻る。
- 追加：`floe publish`。データファイル（YAML）のページを、そこに書いたテンプレートで描き、
  描いたとおりに保存して、ローカルのサーバー（`http://127.0.0.1:7325`）の URL を出す。
  サーバーが動いていなければ裏で起こす。`--link` は保存せず、ファイルをその場で登録する。
  HTML のファイルはそのまま見せ、データファイルはページを開くたびに描き、保存すると開いた
  タブを読み直す。
- 追加：`floe check` と `floe render`。データファイルを書かずに確かめ、ページを floe 無しで
  開ける HTML のファイルに書く（CI 向けに `--check`）。誤りは
  `file:line:column: message` で出し、確かめの 1 つの段の誤りは全部まとめて出す。
- 追加：`floe rebuild`。保存したページを、今のテンプレート・部品・テーマで描き直す。描けなく
  なったページは前のままにする。
- 追加：`floe list`・`floe rm`・`floe serve`・`floe stop` と、ブラウザの言語で英語か日本語に
  なる管理ページ（ページの一覧・編集・表示・ダウンロード）。左の棚で、すべて・お気に入り
  （すべての一覧では先頭にまとめる）・アーカイブ・タグ・種類・プロジェクトを選ぶ（狭い画面
  では 1 つの選ぶ欄にたたむ）。開いたページの上の帯で、お気に入りとアーカイブができる（ボタンはアイコンだけで、オンの間は塗る）。帯の題を押すと、開いた一覧のページが並び、探して選べる。一覧の並べ替えは、管理ページの形のメニューで、マウスでもキーでも選べる。
  タグは、1 枚の編集の画面、選んだものまとめて、`PATCH /api/artifacts/{slug}` の `tags` で
  付け、ページの題の下に出て、検索でも当たり、publish し直しても残る。キーで来たボタンや欄には
  細い灰色の輪を 1 つ出し、マウスで押したボタンには出さない。
- 追加：floe のテンプレート `report`（見出しの下に節を並べ、節ごとに見出しと部品を置く）と、
  テンプレートが `floe:///parts/<name>.json` で指す 32 の部品（`floe part list` が並べる）。
  そのうち 30 は report の節が Markdown の `text`・`html` と並べて取るもの（`code`・
  `terminal`・`table`・`image`（画面の写しなど、データファイルのディレクトリの画像）から
  `findings`・`decision`・`risks`・`timeline`・`tabs`・`diagram`・`script` まで）、`cell` と `stack` はほかの部品が中に持つもの。
- 追加：データファイルが `theme:` で選ぶ 9 つのテーマ（`default`・`soft`・`editorial`・
  `ink`・`brutal`・`blueprint`・`notebook`・`swiss`・`washi`）。どれも CSS の変数のパレットと
  部品の形を持つ。`default` と、その色を使うテーマ（`soft`・`editorial`）のページの地は、管理
  ページと同じ暖かい灰色で、字の色も管理ページと同じ、線も同じ暖かさ。terminal のプロンプト、
  code の言語の札、diff の hunk の頭は黄緑。
- 追加：`FLOE_TEMPLATES_DIR` の自分のテンプレート（`page.html`・`schema.json`・
  `example.yaml`、要るなら `page.css`・`checks.yaml`・`computed.yaml`・`words.yaml`・
  `assets/`）と、`floe template list`・`show`・`new`（埋めて使う最小のテンプレート）・`check`。
- 追加：`FLOE_PARTS_DIR`・`FLOE_THEMES_DIR` の自分の部品・見た目・共有の schema・テーマ
  （floe のものと並べて置き、floe のものと同じ名前は付けない）と、`floe part list`・`show`・
  `new`（埋めて使う最小の部品）・`check`。部品の schema は、テンプレートと同じく floe の
  format（`html`・`markdown`・`image`）と、それを持つ共有の schema を取れる。
  共有の schema は、どれかの schema が名指すかによらず、その根から確かめる。
  自分の部品・共有の schema・テーマ・テンプレートに誤りがあると、直すまでどのページも描かない。
  部品・テーマ・テンプレートの置き場所が同じか、一方がもう一方の中にあれば、その設定を断る。
- 追加：キャンバスに部品（frame・box・table・icon・名前でつなぐ edge・dot・bar・axis・
  circle・label・line）を置く spec から inline SVG で描く図。文は 1 つの言語でも、
  いくつかの言語でも（`{en: …, ja: …}`）書け、描く前に、どの言語でも字のはみ出し・重なり・
  箱を通る線・交差を確かめる。データファイルの `diagram` の部品か、`floe diagram render`・
  `inspect`・`example` で使う。
- 追加：`floe icons fetch` と `floe icons search`。図の `icon` の部品が使う AWS
  Architecture Icons を手元に置く。
- 追加：データファイルから描いたページのスクリプトが、ページの中身を外へ送れないようにする
  CSP。サーバーが返すページにも、`floe render` が書くページにも付ける。
