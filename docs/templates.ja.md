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
| `markdown` | Markdown の文字列を HTML にする（`{{markdown .text}}`）。中の生の HTML は除き、表は表の部品の見た目にする（`<div class="floe-table" data-floe-part="table">`） |
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

部品の要素は `data-floe-part="note"` のように部品の名前を名乗ります。ほかの部品の見た目で描く部品は、その部品の名前を先に並べます（`data-floe-part="table compare"`）。ページには、名乗った部品のスタイルが 1 回ずつ（Markdown の表や、`<span class="floe-pill good" data-floe-part="cell">` のように部品を名乗って見た目を借りる HTML も含む）入り、その後に部品のテーマが続きます。`page.html` が部品に無い名前を名乗ると、`floe template check` が断ります。部品のクラス名は `floe-` で始まります（`floe-table`・`floe-pill`・`floe-cards`・`floe-card`・`floe-note`・`floe-note-title`・`floe-code`・`floe-terminal`、その後の部品は `floe-stats`・`floe-progress`・`floe-steps` のように部品の名前から）。部品のテーマは、同じセレクタ（か、より詳しいセレクタ）の規則を書けば、後に来る分だけ部品の規則より優先されて、見た目を変えられます。`compare` は表の、`stats`・`options`・`facts`・`links` はカードの、`diff` はコードの見た目で描くので、部品のテーマのそれらの規則がこれらにも効きます。

部品のテーマは floe が持つテーマで、どのテンプレートでも同じものを選べます。データファイルの `parts-theme:` で選びます（無ければ `default`。一覧は `floe template show` が出します）。部品のクラスと Markdown の引用（`blockquote`）の見た目を変え、部品を 1 つも使わないページにも入ります。floe はページの CSS を `</head>` の前（無ければページの末尾）の `<style>` にまとめます。floe のものが先で、テンプレートの `page.css` とテーマが 1 つずつ後に続きます。ファイルが閉じ忘れたコメントやブロックは、そのファイルの `<style>` の中で終わります。CSS は 3 つのカスケードレイヤーに入れます。上の層はセレクタにかかわらず下の層に勝ちます。いちばん下の `floe.base` は、文章・リスト・リンク・コード・引用を `report` と同じ見た目にします。その上の `floe.page` にはテンプレートの `page.css` とテーマを、いちばん上の `floe.parts` には部品のスタイル・図のスタイル・部品のテーマを、この順に入れます。テンプレートの CSS は `@scope (:root) to ([data-floe-part] > *)` に入り、部品の要素には届いて部品を置けますが、部品の中には届きません。部品の中は、継承するもの（色・フォント・変数）だけをテンプレートから受け取り、`floe.base` から始まります。そのためテンプレートは部品の見た目も、部品のテーマが決める引用の見た目も変えられません。テンプレートが決めるのは、部品の周りのページです。部品のテーマはコードの色を `.floe-code` と `.floe-terminal` に書き、`:root` には自分の名前の変数（`--brutal-ink`）だけを書きます。テンプレートが `:root` に書く変数と同じ名前だと、ページ全体で部品のテーマが勝つからです。

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
