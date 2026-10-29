# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- 追加：floe の見た目 `band`。アクセントの色で塗った帯で、中の字・リンク・code・フォーカスの輪を、
  塗った面の上の字の色にする。テンプレートは `data-floe-look="band"` で借りる（design の結論と
  handoff の頭もこれを借りる）。
- 追加：floe のテンプレート `adr`。1 つの決めたことを紙 1 枚に書く記録で、題の下に状態の判子
  （提案・採用・却下・廃止・置き換え済み。置き換え済みなら置き換えた記録を書く）、背景、決めたこと、
  選ばなかった案と理由、良くなることと引き受けることを並べる。
- 追加：floe のテンプレート `handoff`。決めてほしいことを先に出す、作業の引き継ぎと状況の
  知らせで、日付、渡す人と受け取る人、終えた・進行中・決めてほしい・次にやるの数、Q1・Q2 と
  番号の付いた問いと案と勧め、誰が何をしていて答え待ちか止まっているか、次にやること、気を付けることを書く。
- 追加：floe のテンプレート `design`。結論を先に出す設計書で、必須の背景と目的に、
  やらないこと、判断（問いと部品 `options` の案。`pick` を付けた案がページの一番上に出て、
  無いものは未決と出る）、案を比べずに決めたこと、影響の範囲、report と同じ形の自分の節を続ける。
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
