[English](templates.md) | [日本語](templates.ja.md)

# テンプレート

テンプレートは、データファイルからページを描きます（`floe publish <data.yaml>`）。floe には `report` が組み込まれています。自分のテンプレートは 1 つのディレクトリで、同じ名前の組み込みのテンプレートの代わりに使われます。

## テンプレートの置き場所

- 自分のテンプレートは `$FLOE_TEMPLATES_DIR/<name>/` のディレクトリです。ディレクトリ名がテンプレートの名前になります。`FLOE_TEMPLATES_DIR` の既定は `$XDG_CONFIG_HOME/floe/templates`、無ければ `~/.config/floe/templates` です。
- ディレクトリは symlink でも構いません（リポジトリの clone を指す、など）。中のファイルが symlink でディレクトリの外を指していれば断ります。
- テンプレートの名前は、英小文字・数字・`-`・`_` で、英小文字か数字で始めます。ファイルや、ほかの名前のディレクトリはテンプレートではありません。
- 自分のテンプレートが壊れていれば、公開はエラーで止まります。同じ名前の組み込みのテンプレートには切り替えません。

## ファイル

```text
my-report/
├── template.yaml    # 必要な floe の版（任意）
├── page.html        # ページ。Go の html/template
├── page.css         # ページの CSS。部品の CSS の下の層に入る（任意）
├── schema.json      # データファイルに書ける項目（JSON Schema）
├── example.yaml     # 書き始めに使うデータファイル
├── checks.yaml      # 項目どうしの関係（任意）
├── computed.yaml    # データから計算する値（任意）
├── assets/          # page.html が asset で出す画像（任意）
└── themes/
    ├── default.css  # テンプレートの既定のテーマ。ページの見た目（必須）
    └── print.css    # ほかのテーマ（任意）
```

作り始めるときは、`floe template new my-report --from report` で別のテンプレートを写します。`floe template show <name>` で、テンプレートのデータファイルに書けるものを出せます。`floe template check <dir>` は、使う前や配る前にテンプレートを検査します。ファイルを検査してから、`example.yaml` を公開のすべての段に通します（[commands.ja.md](commands.ja.md#floe-template-check-dir)）。

## template.yaml

```yaml
floe: 1.2
```

このテンプレートには、floe 1.2.0 以降で、メジャーが 1 のものが要ります。メジャーが 0 のあいだはマイナーの版でテンプレートの書き方が変わりうるので、`floe: 0.2` は 0.2.0 以降の 0.2 の floe を要ります。合わない floe では、データを検査する前に公開が止まり、テンプレートが要る floe の版を示します。`template.yaml` の無いテンプレートは、どの floe でも描けます。`floe template check` はこれを警告します。

## page.html

`page.html` は Go の [html/template](https://pkg.go.dev/html/template) で、差し込む値をエスケープします。使えるものは次のとおりです。

| 名前 | 中身 |
|---|---|
| `.Title` | ページのタイトル。`--title`、管理画面で固定したタイトル、データファイルの `title:`、slug の順に決まる |
| `.Lang` | ページの言語（データファイルの `lang:`） |
| `.Data` | データファイルの項目（floe が自分で読む `lang`・`parts-theme`・`template`・`theme`・`title` を除く。これらは `.Lang`・`.Title` で使い、テーマは floe が入れる。どれかを宣言する、または必須に挙げるスキーマは、どこで使っても断る）。スキーマで宣言していてデータに無い項目は nil なので、`{{with .Data.lead}}` と書ける。スキーマで宣言していない名前は描画を止める。`"format": "html"` の項目は HTML としてそのまま、`"format": "markdown"` の項目はその HTML として入る。図はその SVG として、スクリプトは置き場所の要素と関数の呼び出しとして入る |
| `.Calc` | `computed.yaml` の値（名前で引く） |
| `markdown` | Markdown の文字列を HTML にする（`{{markdown .text}}`）。中の生の HTML は除き、表は表の部品の見た目にする（`<div data-floe-part="table">`） |
| `markdownLine` | 表のセルのような 1 行の Markdown を、段落で包まずに HTML にする（`<td>{{markdownLine .}}</td>`）。コード・強調・リンクを書け、`-`・`1.`・`# x` のような行は文字のまま出る。複数の行は、字下げや空行があっても 1 つにつなぐ。中の生の HTML は除く |
| `num` | 数を桁区切りで書く（`{{.Calc.total \| num}}` は `1,234,567`）。有効数字で丸めることもできる（`{{.Calc.rate \| num 3}}` は `0.00648`） |
| `bytes` | バイト数を二進の単位で書く（`{{.Calc.size \| bytes}}` は `1.5 KiB`） |
| `asset` | テンプレートの `assets/` の画像を data URI にする（`<img src="{{asset "logo.svg"}}">`） |

`page.html` には `<style>` を書けません。書くと floe がその行を示して断ります。ページの CSS は floe が、`page.css` とデータファイルが選んだテーマから組み立てます（[部品](#部品)）。これらはページに閉じた範囲（`@scope`）に入り、その根を `:scope` が指します。ページの変数は `:scope` に書きます。そこでは `:root` と `html` は何も指さないので、書くとその行を示して断ります。Google Fonts のようなスタイルシートへの `<link>` は書けます。floe の層の外にあり、層のどれよりも勝ちます。スタイルシートはこの `<link>` で読みます。`page.css` とテーマの `@import` は、層の中ではブラウザが読まないので、floe がその行を示して断ります。どのブロックも閉じない `}` も、floe の層を閉じてしまうので断ります。

データファイルから描いたページは、出すものをすべて中に持ち、そのスクリプトはページの中身を外へ送れません。Content-Security-Policy により、読み込めるのは data URI（と `blob:`）の画像・フォント・音声と動画、インラインのスタイルとスクリプト、Google Fonts、cdnjs・jsDelivr・unpkg のライブラリ（スクリプト・スタイル・フォント。`eval` と WebAssembly は不可）だけです。`fetch`・XHR・ほかの場所の画像・フォームは止まり、管理画面の API にも届きません。ダウンロードしたファイルも、同じ制限を `<meta>` で持ちます。Google Fonts と CDN には、そこから読み込むものの要求が出ます。スクリプトによるページの移動・ウィンドウを開くこと・WebRTC・DNS の先読みは CSP では止められないので、これはうっかりした通信を防ぐもので、悪意のあるテンプレートを閉じ込めるものではありません。信頼できるテンプレートだけを使ってください。

## schema.json

データファイルの項目の [JSON Schema](https://json-schema.org/)（2020-12）です。キーの打ち間違いを断るため、`"additionalProperties": false` を書きます。`template`・`theme`・`parts-theme`・`title` は floe が読むので、宣言しません。

`format` には、floe が検査する JSON Schema の format（`date`・`email`・`uri` など）か、文字列の読み方を決める floe の format（`html`・`image`・`markdown`）を使います。それ以外の format は、この floe ではどんな値も通してしまうので、`floe template check` が誤りにします。floe の format は floe が探す `properties` と `items` の下だけに書きます。`anyOf` や `$defs` などの下に書くと、その項目は文字列として描かれます。型は `"type": "string"` だけを宣言します。図とスクリプトは format ではなく[部品](#部品)です。`"format": "diagram"`・`"format": "script"` は、スキーマのどこに書いても `floe template check` が断ります。`properties` と `items` の下に書いたものは、テンプレートをどこで使っても断ります。

`"format": "markdown"` の項目には Markdown を書きます。`{{.Data.text}}` が、`markdown` と同じく HTML に描きます。すでに HTML なので、`{{markdown .Data.text}}` と書くと描画が止まります。

`"format": "html"` の項目はそのまま入りますが、`<style>` と `<link>` は書けず、その行と列を示して断ります。ページの CSS は floe が組み立てるので、HTML の見た目は `style` 属性で付けます。

`"format": "image"` の項目には、画像のパスを、データファイルのディレクトリからの相対で、その配下に、`..` やドットで始まる名前を使わずに書きます。画像は data URI としてページに入るので、`<img src="{{.Data.chart}}">` と書けて、ページだけで表示が完結します。

部品 `diagram`（`"$ref": "floe:///parts/diagram.json"`）の項目には、図の spec を [`floe diagram render`](commands.ja.md#floe-diagram-render-specyaml) に渡すのと同じ形でその場に書きます。spec は `floe diagram render` と同じく検査し、誤りはデータファイルの行と列で出します。1 ページの図の id は互いに違えます。`{{.Data.plan}}` がインライン SVG で図を描き、ページの末尾に図のスタイルとズームが 1 回だけ付きます。図は色と書体を、[部品](#部品) と同じくテーマの CSS 変数から取ります。

部品 `script`（`"$ref": "floe:///parts/script.json"`）の項目には、ページで動かすものを、関数の `js` と、任意の値の `data` で書きます。`{{.Data.calculator}}` は空の要素を描き、その要素と data を渡して関数を呼びます（`function (el, data) { … }`）。data は JSON として入り、キーと値の組は名前の順に並びます。順番が意味を持つものはリストで書きます。floe が検査するのは形（`js` が関数の文字列であること、`js` と `data` のほかに何も無いこと）だけで、ページがそこでスクリプトの終わりと読む `</script` か `<!--` を含む `js` は断ります。関数の中の誤りはブラウザで分かります。関数は、ページを読み込む途中のその位置で動きます。後ろの要素や Web フォントはまだ無いので、それらを使うときは `load` を待ちます（`addEventListener('load', …)`）。関数はページの sandbox の中で、その CSP の下で動きます（[page.html](#pagehtml) を参照）。

## 部品

floe には、テンプレートの項目を描く部品があります。`table`（セルにピルを置ける）・`cards`・`note`・`code`・`terminal`・`stats`・`progress`・`steps`・`checklist`・`compare`・`options`・`facts`・`diff`・`tree`・`links`・`findings`・`details`・`matrix`・`decision`・`risks`・`fields`・`timeline`・`bars`・`faq`・`glossary`・`keys`・`tabs`・`cell`（表のセル。1 行の Markdown かピル `{ pill, tone }` で、`table`・`facts`・`findings`・`decision` が持つ）・`stack`（部品を縦に並べたもの。`details` と `tabs` が持つ）・`diagram`・`script`（後の 2 つは上で説明）です。`note` は Markdown の `text` と `tone` で、`title` を書けば上に見出しを出します。`code` は `text` にコードをそのまま書き、上にファイル名（`file`）を、その右端に言語（`lang`）を出します。`terminal` は `steps` に、打つコマンド（`run`。2 行目からはその続き）と出力（`out`）の組を並べ、`shell` のプロンプトを付け、シェルの名前を上に出します。`bash`・`sh`（`$`）・`zsh`（`%`）・`powershell`（`PS>`）・`cmd`（`>`）で、省くと名前は出さずにプロンプトは `$` です。どちらもエディタやターミナルと同じく、ライトモードでも暗い地です。プロンプトはスタイルで描くので、コマンドをコピーしても入りません。最初の語がプロンプト（`$`・`#`・`%`・`>`・`❯` で終わる語）の `run` は断ります。`stats` は大きな数の並びで、それぞれ `label` と引用符で囲んだ `value`、`unit`、`tone` の色の `note` です。`progress` は棒の並びで、それぞれ `label` と、`of` のうち `done` まで埋まる棒です。`steps` は番号付きの段で、それぞれ `title` と Markdown の `text`、`state` が `done`・`doing`・`todo`（省いたとき）です。`checklist` は 1 行の Markdown の項目で、`done` なら印が付き、済んだ数を最後に出します。`compare` は見出し `before` と `after` の下に、項目（`what`）ごとの `before` と `after` を `rows` に並べます。両側が同じ行は薄く描きます。`options` は `title`・Markdown の `text`・`pros`・`cons` のカードで、選ぶ案は `pick`（「勧める」などの語）の札と枠で示します。`facts` は `name` と `value` の組で、`value` は表のセルと同じく 1 行の Markdown かピルです。`diff` は git が書く形の差分を `text` に書き、コードの見た目で、上にファイル名（`file`）、足した行と消した行の数、言語（`lang`）を出します。`tree` はそれぞれ `path`（フォルダは `/` で終える）で、パスが指すフォルダに入れ子にし、`note` と `change`（`added`・`removed`・`changed`）を付けます。`links` は `url`（http・https・mailto かパス）へのリンクの `title` と、一言（`text`）と URL を、リンクの印と並べたカードです。`findings` はレビューの指摘で、それぞれ `level`（`must`・`should`・`nit`）、場所（`at`）、中身（`text`）、見送った理由（`why`）、結末のセル（`status`）です。`decision` は大きな `title`、理由の Markdown の `text`、退けた案（`rejected`）を、`status` のセルと `date` の下に描きます。`risks` はそれぞれ `what`、起きやすさ（`likely`）と影響（`impact`）を `low`・`mid`・`high` で書き、両方から色を決め、手当て（`fix`）を添えます。`matrix` は項目（`what`）と `cells` を `rows` に並べ、セルは `columns` と同じ数だけ、`yes`（✓）・`part`（△）・`no`（×）か、下に一言を添える `{ mark, note }` です。`fields` はそれぞれ `name`・`type`・必須か（`required`）・説明（`text`）・既定値（`default`）です。`timeline` はそれぞれ `date`・`title`・Markdown の `text` で、点は `tone` の色です。`bars` はそれぞれ `label` と 0 以上の `value` で、いちばん大きい値をいちばん長く、`tone` の色で描きます。`faq` はそれぞれ問い（`q`）と、押すと開く Markdown の答え（`a`）で、`open` なら最初から開きます。`glossary` はそれぞれ `term` と Markdown の `text`、`keys` はそれぞれ同時に押す `keys` と働き（`text`）です。`details` は `title` の下に `parts` を畳み（`open` なら最初から開く）、`tabs` はスクリプト無しで、`title` ごとの `parts` を 1 つずつ見せます（印刷ではすべて出します）。中に置く部品は `text`（Markdown）・`code`・`terminal`・`diff`・`note`・`table` のどれか 1 つのキーで、ほかの部品と同じく検査して描きます。`risks` の見出しのような部品の言葉は、ページの言語で描きます。floe が言葉を持たない言語では英語です。schema では、floe が探す `properties` と `items` の下で、部品を `$ref` で指します。`anyOf` や `$defs` などの下では描かれないので、`floe template check` が誤りにします。

```json
"risks": { "$ref": "floe:///parts/table.json", "description": "What may go wrong" }
```

`page.html` では、項目を置きたい場所に `{{.Data.risks}}` と書きます。floe は項目を部品の schema と検査で確かめ（誤りはデータファイルの行と列で出します）、HTML に描きます。`floe template show` は、`$ref` の横に書いた説明に続けて、部品の項目を出します。floe に無い部品を指す `$ref` は、floe にある部品を添えて断ります。

部品の要素はその部品の箱で、`data-floe-part="note"` のように部品の名前を名乗ります。要素は 1 つの部品の箱なので、名前は 1 つです。部品は、フォルダの `part.yaml` の `looks: [table]` でほかの部品の見た目を借ります。借りた見た目がさらに借りている見た目も付き、それぞれ借りる部品の前に入って、借りる部品がそれを直します。floe には、描く schema を持たない `part.css` だけの「見た目だけの部品」もあります。多くの部品が描かれるパネルの箱 `panel` と、パネルに行を並べる `rows` です。部品の箱にならずに見た目を借りる要素は、`data-floe-look` で名乗ります。`cards` のカードはこれでパネルの見た目を借りていて、テンプレート自身の要素も借りられます（`<section data-floe-look="panel">`）。ページには、名乗った部品と見た目のスタイルが、それらが借りる見た目とともに 1 回ずつ入り（Markdown の表や、`<span class="good" data-floe-part="cell">`（tone が `good` のピル）のように部品を名乗るデータの HTML も含む）、その後に部品のテーマが続きます。`page.html` が部品に無い名前を名乗ると、`data-floe-part` が書いたとおりで 1 つの部品の名前でないと（`"table compare"`・`" table"`）、箱を持たない見た目だけの部品を `data-floe-part` で名乗ると、`floe template check` が断ります。後の 2 つは、描くページの HTML にあっても描くときに断ります。部品のスタイルと、部品のテーマのその部品の分は、その見た目を持つ要素と、それらが持つ部品の箱には届きますが、持つ部品の中には届きません。より詳しい規則が勝ち、同じなら後の規則が勝つ順に入ります。部品のスタイルが先で、それぞれ借りる見た目の後に入り、部品のテーマはその後です。箱にならずに見た目を借りる要素では、その見た目が部品とテンプレートより下の層に入るので、要素を持つ部品や、自分の要素に借りさせたテンプレートは、セレクタにかかわらずそれを直せます。そうした要素どうしは同じ層に入るので、見た目を借りる要素を中に持つ要素は部品の箱（`data-floe-part`）にしてください。箱の規則はそれらの規則に勝ちます。`compare`・`fields`・`findings`・`matrix`・`risks` は表の、`stats`・`options`・`facts`・`links`・`decision` はカードの、`diff`・`terminal` はコードの見た目を借り、`checklist`・`faq`・`glossary`・`keys` は `rows` の、ほかのパネルで描く部品は `panel` の見た目を借ります。そのため部品のテーマのそれらの規則がこれらにも効きます。カードのうち番号が付くのは `cards` と `options` です。

部品のテーマは floe が持つテーマで、どのテンプレートでも同じものを選べます。データファイルの `parts-theme:` で選びます（無ければ `default`。一覧は `floe template show` が出します）。部品や見た目ごとのファイル `<名前>.css` でそれぞれの見た目を変え（その見た目を持つすべての要素に届きます。`panel.css` はすべてのパネルに、`code.css` はコード・差分・ターミナルに）、どのページにも入るファイルで Markdown の引用（`blockquote`）の見た目を変えます。後者は部品を 1 つも使わないページにも入ります。floe はページの CSS を `</head>` の前（無ければページの末尾）の `<style>` にまとめます。floe の基本の見た目が先で、部品ごとのスタイル、部品のテーマのファイル、テンプレートの `page.css` とテーマが、ファイルごとに 1 つずつ後に続きます。ファイルが閉じ忘れたコメントやブロックは、そのファイルの `<style>` の中で終わります。CSS は 4 つのカスケードレイヤーに入れます。上の層はセレクタにかかわらず下の層に勝ちます。いちばん下の `floe.base` は、文章・リスト・リンク・コード・引用を `report` と同じ見た目にします。その上の `floe.looks` には部品の箱にならずに見た目を借りる要素の見た目を、その上の `floe.page` にはテンプレートの `page.css` とテーマを、いちばん上の `floe.parts` には部品のスタイル・図のスタイル・部品のテーマを、この順に入れます。テンプレートの CSS は `@scope (:root) to ([data-floe-part] > *)` に入り、部品の要素には届いて部品を置けますが、部品の中には届きません。部品の中は、継承するもの（色・フォント・変数）だけをテンプレートから受け取り、`floe.base` から始まります。部品の要素には部品のクラス（`good`・`bad` などの色合い、`numbered`、`sh` などのシェル）も付いていて、テンプレートの同じ名前のクラスの規則はそこにも届きます。部品はその名前（`[data-floe-part="note"]`）で選び、テンプレート自身のクラスには別の名前を付けてください。そのためテンプレートは部品の見た目も、部品のテーマが決める引用の見た目も変えられません。テンプレートが決めるのは、部品の周りのページです。部品のテーマはコードの色を `code.css` に書き、`:root` には自分の名前の変数（`--brutal-ink`）だけを書きます。テンプレートが `:root` に書く変数と同じ名前だと、ページ全体で部品のテーマが勝つからです。

部品と図は色・書体・注記の幅を、テーマの CSS 変数から取ります。変数の無いテーマでは、部品が持つ明るい値（report と同じ）を使います。

| 変数 | 意味 |
| --- | --- |
| `--bg`・`--surface` | ページの地、部品の面 |
| `--fg`・`--muted` | 文字、薄い文字 |
| `--line` | 線と枠 |
| `--accent`・`--accent-soft` | 強調の色と、その淡い地 |
| `--good`・`--warn`・`--bad` とそれぞれの `-soft` | 良い・注意・悪いの色と、その淡い地 |
| `--font`・`--mono` | 本文の書体、等幅の書体 |
| `--text-width` | 注記が広がる幅（本文と同じ） |
| `--code-bg`・`--code-fg`・`--code-prompt` | コードとターミナルの地（ライトモードでも暗い）、その字（コマンドの出力はこれを薄くする）、ターミナルのプロンプトとコードの言語の札 |
| `--code-add`・`--code-del` | 差分の足した行と消した行（暗い地の上） |

変数はテーマの `:scope` で定義するか、すべてのテーマの分を `page.css` で定義します。`floe template check` は、floe の基本の見た目、部品や図、部品のテーマが使う変数のどれかを定義していないテーマを警告します。そのテーマの部品はダークモードでも明るい色のままになります（コードとターミナルは暗いまま）。`tone`（ピル・注記・図の部品）は `accent`・`good`・`warn`・`bad` のどれかで、`--accent`・`--good` などの色と、その `-soft` を使います。

## 自分の部品

- 自分の部品は、部品の名前のディレクトリ `$FLOE_PARTS_DIR/<名前>/` に、floe の部品と同じファイルを置きます。`schema.json`・`part.html`・`part.css` と、要れば `part.yaml`・`checks.yaml`・`words.yaml` です。自分の見た目だけの部品は `part.css` と、見た目を借りるなら `part.yaml` です。`FLOE_PARTS_DIR` の既定は `$XDG_CONFIG_HOME/floe/parts`、無ければ `~/.config/floe/parts` です。ディレクトリはシンボリックリンクでもかまいません。その中からシンボリックリンクで外へ出るファイルは断ります。
- 名前の決まりはテンプレートと同じです（英小文字・数字・`-`・`_`）。ほかの名前のディレクトリは誤りとして知らせ、ドットで始まるものは無視します。floe が自分で描く `diagram`・`script`、部品が共有する色合いの `tone`、どのページにも入る部品のテーマのファイルの名前 `base` は断ります。
- schema からは floe の部品と同じく `"$ref": "floe:///parts/quote.json"` で指します。floe の部品と同じ名前なら、その部品に丸ごと代わります。どのテンプレートでも、その見た目を借りる部品でも同じです（自分の `table` は Markdown の表と `compare` も描き、それらは floe の表ではなく自分の `table` が借りる見た目を借ります）。置かなかったファイルを floe の部品が補うことはなく、floe の部品のテーマのその部品のファイルは、その部品にも、その見た目を借りる部品にも当たりません。見た目だけの部品は、描く floe の部品には代われません。
- 部品の要素は箱として `data-floe-part="quote"` で自分を名乗り、借りる見た目は `part.yaml` に書きます。`looks: [panel]` と書けば、どの部品のテーマでもパネルで描かれます。見た目だけの部品は何も描かないので、`$ref` で指すと断ります。schema の `$ref` は、JSON Schema と同じく schema の場所から読みます（`cell.json` は `floe:///parts/cell.json`）。`part.css` は floe の部品と同じく部品の範囲に入るので、要素は `:scope` と書き、中のクラスはほかの部品ともテンプレートともぶつかりません。どのブロックも閉じない `}`、`@import`、セレクタの `:root`・`html` は行を示して断ります。閉じ忘れたブロックは、その部品の `<style>` の中で終わります。
- 使えない自分の部品（ファイルの誤り、断る名前、`$ref` で持つ部品がまた自分を持つもの、借りる見た目がまた自分の見た目を借りるもの）を使うテンプレートは、schema からでも `page.html` からでも、理由を添えて断ります。Markdown の表などでその部品を名乗るページも断ります。同じ名前の floe の部品には戻しません。その見た目を借りる部品や、その部品を持つ部品も、floe の部品も含めて使えなくなります。テンプレートが使わない壊れた部品は、`floe template check` が警告で知らせます。

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption class="source">{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

自分の部品のテーマは、ディレクトリ `$FLOE_PARTS_DIR/themes/<名前>/` に floe のテーマと同じファイルを置きます。`base.css`（どのページにも入る。自分の名前の変数と Markdown の引用）と `<名前>.css`（部品や見た目のスタイル。その見た目を持つ要素の範囲に入る）で、要らないものは省けます。ほかの名前の CSS ファイルはどのページにも入らず、`floe template check` が警告します。CSS でないファイルと、ドットで始まるファイル（`.DS_Store`）は無視します。floe のテーマと同じ名前なら、同じ名前のファイルだけが代わり、ほかは floe のものが残ります。`themes/brutal/quote.css` を置けば、brutal で自分の quote の見た目を決められます。floe のファイルを消すには空のファイルを置きます。使えないテーマは、それを選んだデータファイルで断ります。

公開したページとリンクしたページは、サーバーの部品で描きます。サーバーは、それを起動した `floe publish` か `floe serve` の部品を使います。部品のファイルを保存すると、リンクしたページを読み直します。

## 画像

画像は、データのものも `assets/` のものも、png・jpeg・gif・webp・svg のファイルで、名前と中身の両方で判定し、1 MiB までです。macOS で作ったテンプレートが linux でも描けるよう、パスの各名前は大文字小文字まで書いたとおりに照らし、どこを指すものであれ symlink は通しません。データの画像の誤りは、その行と列で出します。`floe template check` は、`assets/` のすべての画像と、`page.html` が `asset` に渡すすべての名前を、見本が通らない分岐の中のものも含めて確かめます。`example.yaml` が書く画像はテンプレートのディレクトリから読むので、見本から始めたデータファイルには、その画像も隣に要ります。

## checks.yaml

スキーマでは書けない、項目どうしの関係です。リストの要素 1 つずつに検査を当てます。

```yaml
- for: access_patterns            # リストまでのキーのパス（sections.parts ならすべての節の部品）
  field: key                      # 落ちたときに指す、要素のキー（任意）
  expr: item.key in map(doc.attributes, .name)
  message: キーには属性一覧にある属性を使う
```

`for` の途中にあるリストは、要素ごとに辿ります。`sections.parts` はすべての節の部品を検査し、落ちた部品は `sections[1].parts[0]` のように出します。`field` は要素の下のキーのパスだけです。

`expr` は、成り立つべき [expr](https://expr-lang.org/) の式です。リストの要素を `item`、データファイルの項目を `doc` で参照します。落ちた要素は、`field` の位置にその値を添えて出します。

```text
orders.yaml:13:12: access_patterns[0].key: キーには属性一覧にある属性を使う（「user_id」）
```

`for`・`field` のパスは、`schema.json` の `properties` と `items` で宣言されている必要があります。打ち間違えたパスはデータの中に検査するものが見つからず、何も検査しません。`floe template check` は `properties` と `$ref` を（`for` は `items` も）辿り、キーを持たないスキーマがほかのキーを断る（`"additionalProperties": false`、`"unevaluatedProperties": false`、または `object` 以外の `type`）位置のパスと、型に `array` を含まない `for` を誤りにします。`patternProperties`・`allOf`・`anyOf`、許したほかのキーなどで宣言されうるパスは通します。`checks.yaml` そのものの誤りは、`checks.yaml` の行と列で出します。データファイルと同じく、アンカー・エイリアス・タグは使えません。

## computed.yaml

データから計算する値です。`page.html` は `.Calc.<名前>` で表示します。

```yaml
item_bytes: sum(attributes, .avg_bytes)
storage_gb: item_bytes * records.in_1_year / 1e9
```

式からは、スキーマで宣言したデータファイルの項目と、ほかの計算の値が、名前でそのまま見えます。書く順番は自由です。名前は文字で始まり、文字・数字・`_` が続くもので、データの項目とも、expr の語や関数とも重ねられません。データから計算できない値（0 で割った、式に要る項目が無い）は誤りとして出します。値が NaN や ±Inf になるときも同じです。`computed.yaml` そのものの誤りは、`computed.yaml` の行と列で出します。データファイルと同じく、アンカー・エイリアス・タグは使えません。

## 検査の順番

データファイルは次の順に検査します。各段は、前の段を通ったデータにだけ動きます。`floe check` は公開せずにすべての段を通します。

1. YAML
2. テンプレート、テンプレートが要る floe の版、テーマと部品のテーマ、テンプレートの `schema.json`、画像、図
3. テンプレートの `checks.yaml`
4. テンプレートの `computed.yaml`
5. `page.html` の描画
