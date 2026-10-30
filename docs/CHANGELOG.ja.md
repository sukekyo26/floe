# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- 追加：印刷でも、ページの地を含めてどの地も画面のとおりに出す（塗った地の上の明るい字が白い紙に消えない）。
  CSS の変数 `--face`・`--on-face`。色で自分を塗る面（帯、swiss の危険の注意と灰色の注意、ink の引用、コード）が地と
  字の色を置くと、floe の土台が中のリンク・行の code・フォーカスの輪をその字の色で描く。
- 追加：`floe theme list` と `floe theme show`。`floe part show`・`floe template show`・
  `floe theme show` は、部品・見た目・共有の schema・テンプレート・テーマが読むファイルを全部、
  並べた `files:` の行の後に、1 つずつ `==> 名前 <==` の下に書いたとおりに出す。ファイル名を足すと、
  そのファイルだけを、先頭の BOM を除いて書いたとおりに出す（`floe part show code part.html`）。
- 追加：floe のテンプレート `slides`。1 枚ずつ見せるスライドで、帯の表紙の後に、画面の高さの枚
  （見出しと report と同じ部品）を並べ、スクロールで 1 枚ずつ止めて右下に何枚目かを出す。見出しだけの枚は
  話の区切りになり、印刷すると 1 枚が 1 ページになる。`direction: across` と書くと枚を横に並べ、← → のキーや
  左右のなぞりで送る。
- 追加：floe の見た目 `numbered`。リストの項目に丸い番号を付け、次の項目へ線でつなぐ。部品 `steps`
  が借り、テンプレートは `<ol>` に `data-floe-look="numbered"` で借りる（runbook の手順もこれを借りる）。
- 追加：floe のテンプレート `runbook`。打つコマンドと一緒に残す手順書で、始める前に確かめること、
  注意、番号と線でつないだ手順（行う人の札、写すボタン付きのコマンド、失敗したらすること）、
  別の枠の戻し方を並べる。
- 追加：floe のテンプレート `postmortem`。人を責めずに書く障害の振り返りで、帯の中の要約と
  重さの札（重大・中程度・軽微。赤は重大だけ）、影響、部品 `timeline` で描く時系列、原因、
  うまくいったこと・まずかったこと・運が良かったこと、担当と状態（これから・作業中・済み）の
  付いた対策を並べる。
- 追加：floe の見た目 `band`。アクセントの色で塗った帯で、中の字・リンク・code・フォーカスの輪を、
  塗った面の上の字の色にする。テンプレートは `data-floe-look="band"` で借りる（design の結論と
  handoff・postmortem の頭もこれを借りる）。
- 追加：floe のテンプレート `adr`。1 つの決めたことを紙 1 枚に書く記録で、題の下に状態の判子
  （提案・採用・却下・廃止・置き換え済み。置き換え済みなら置き換えた記録を書く）、背景、決めたこと、
  選ばなかった案と理由、良くなることと引き受けることを並べる。
- 追加：floe のテンプレート `handoff`。決めてほしいことを先に出す、作業の引き継ぎと状況の
  知らせで、日付、渡す人と受け取る人、終えた・進行中・決めてほしい・次にやるの数、Q1・Q2 と
  番号の付いた問いと案と勧め、誰が何をしていて答え待ちか止まっているか、次にやること、気を付けることを書く。
- 追加：floe のテンプレート `design`。結論を先に出す設計書で、必須の背景と目的に、
  やらないこと、判断（問いと部品 `options` の案。`pick` を付けた案がページの一番上に出て、
  無いものは未決と出る）、案を比べずに決めたこと、影響の範囲、report と同じ形の自分の節を続ける。
- 追加：`code`・`terminal`・`diff` の右上に、中身を写すボタン。code は書いたまま、
  terminal はプロンプトと出力を除いたコマンドだけ、diff は書いたままを写し、
  写したことをページの言語で示す。ページがクリップボードに書けない所（VS Code の
  Simple Browser など）でも写す。長い行は、右端までスクロールすると、終わりがボタンの
  左に出る。
- 追加：警告。ページのより良い書き方の勧めで、`floe check`・`floe render`・
  `floe template check`・`floe publish` が、誤りの後に
  `file:line:column: warning: message` で出す。ページは描き、終了コードも
  変えない。最初の警告は、`text` の部品（report の節と、design・handoff・adr・
  postmortem・runbook の `more` の中、runbook の手順の `text`、slides の枚、
  `details`・`tabs` の中も）の囲んだコードのブロックで、ただのコードとして描き、`code` か `terminal` に
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
  部品の `part.html` は、floe の部品が使う関数（`copyButton`・`bars`・`diff`・`tree`・
  `diagram`・`script`）を呼べる。`bars` は、キーと値で書いていない項目と、
  数でないか 0 より小さい値を断る。`part.html` を走らせた text/template の誤り（schema が
  宣言していない鍵など）は、`page.html` と同じくその行と列で言い、関数が値を断る誤り
  （`num`・`bars`）は、その値の位置で言う。
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
