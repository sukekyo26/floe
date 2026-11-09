# 部品とテーマ

自分の部品・見た目・テーマの作り方です。部品はデータファイルの項目を描き、テーマはページの色・書体・部品の形を決めます。テンプレートで部品を使うには [templates.ja.md](templates.ja.md#テンプレートの中の部品)、データファイルに部品を書くには [pages.ja.md](pages.ja.md#部品) を見てください。

## 部品とテーマの置き場所

- 自分の部品・見た目・共有の schema は `FLOE_PARTS_DIR`（既定は `$XDG_CONFIG_HOME/floe/parts`、無ければ `~/.config/floe/parts`）に置きます。部品と見た目はディレクトリ、共有の schema はその隣のファイル `<名前>.json` です。直下のほかのファイルは読まず、何も言いません。部品のファイルは、その名前のディレクトリの中に置いてください。
- 自分のテーマは `FLOE_THEMES_DIR`（既定は `$XDG_CONFIG_HOME/floe/themes`、無ければ `~/.config/floe/themes`）に、1 つずつディレクトリで置きます。直下のファイルは読まず、何も言いません。テーマのファイルは、その名前のディレクトリの中に置いてください。
- 名前は英小文字・数字・`-`・`_` で、英小文字か数字で始めます。floe の部品・見た目・共有の schema・テーマが持つ名前と、floe のテンプレートの節が取る鍵（`html`）は断ります。自分のものは floe のものと並べて置き、置き換えることはありません。部品と見た目には、テーマがどのページにも入れるファイルの名前 `base`・`palette` も付けません。
- その中のディレクトリはシンボリックリンクにできません。ファイルはできます。

```sh
floe part new quote     # 埋めて使う最小の部品 quote を作る
floe part check         # 自分の部品・見た目・共有の schema・テーマを全部確かめる
floe part show quote    # 見た目と、読むファイルの並びを出す
```

誤りが 1 つあると、[templates.ja.md](templates.ja.md#テンプレートの置き場所) のとおり、どのページも止めます。`floe part check` が並べます。書いている間は、別の置き場所で作り、確かめてから移してください（[templates.ja.md](templates.ja.md#別の置き場所で作る)）。`FLOE_PARTS_DIR` か `FLOE_THEMES_DIR` をそこに向け、`floe part check` で確かめます。

## 部品のファイル

```text
quote/
├── schema.json    # データファイルが書けるもの（JSON Schema）
├── part.html      # 描き方。Go の html/template
├── part.css       # スタイル（空でもよい）
├── part.yaml      # 借りる見た目、印のある行（任意）
├── checks.yaml    # 項目どうしの関係（任意。テンプレートのものと同じ書き方）
└── words.yaml     # 言葉、言語ごと（任意。テンプレートのものと同じ書き方）
```

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- schema は、floe の部品と同じく `"$ref": "floe:///parts/quote.json"` で名指します。部品の schema も、floe の部品と共有の schema をこう名指します。
- 自分の部品は `any.json` に加わります。floe のテンプレートの節（report の `parts`）は floe の部品と同じく自分の部品を取り、`any.json` を名指す自分の schema も同じです。`stack`（`details`・`tabs`・`columns`・`options`・`entries`・`checks` の中）には加わりません。自分の部品は form や HTML を持てて、それらの中では崩れるためです。`floe part list` は floe の部品と一緒に並べ、`floe part show any any.json` は書いてあるとおりの floe の `any.json` を出します。
- 部品の要素は箱で、`data-floe-part="quote"` で自分を名乗ります。
- 見出し（`<h1>`〜`<h6>`）は描きません。部品は、テンプレートの節・`details`・ほかの部品の中など、自分が置かれる段を知らないからです。題は `class="title"` の要素（`<div class="title">`）で書き、見た目 `cards` がカードの題として描きます。見出しは、テンプレートと、その Markdown が描くものです。floe は確かめません。
- `part.html` は項目を `.` で見て、テンプレートの `page.html` と同じ関数を使えます（`asset` は除く。[templates.ja.md](templates.ja.md#pagehtml)）。`part.html` の `markdown` は、部品の値の中の format `markdown` の項目と同じく、Markdown を text の外枠（`<div data-floe-text>`）に描きます。外枠の中のブロックの間は floe が `--floe-room` にし、部品は外枠だけを置いて、ブロックの間を書きません（`:scope p` は外枠の中に届き、`:scope > p` は届きません。上の quote では、blockquote が外枠を持ちます）。空の Markdown は、外枠を描きません。schema が宣言し、値が省いた項目は、`page.html` の `.Data` と同じく nil で来ます。`part.html` を走らせた誤り（schema が宣言していない鍵、range できない値）は、それを持ち込んだ値でなく、`part.html` の行と列で言います。値の形を部品の schema で決めておくと、違う形の値はその値の位置で言われます。
- floe の部品が描くのに使う関数も使えます（[part.html の関数](#parthtml-の関数)）。
- 部品の schema は、テンプレートと同じく floe の `format`（`html`・`markdown`・`image`）を取ります。floe が辿る `properties`・`additionalProperties`・`items` の下か、根で宣言します。format を持つ共有の schema も取ります。その項目は `part.html` に描いたものが来ます（`html` は書いたとおり、`markdown` は HTML に、`image` は data URI に。`<img src="{{.file}}" alt="{{.alt}}">`）。`{{.h}}` と書き、関数（`markdown`・`markdownLine`・`printf`・`slice`・`eq` など）に渡したり、`<script>` の中に置いたりしません（書いた字でなく HTML なので、そこでは正しく動きません）。`html` の項目は、テンプレートの項目と同じく描いたページで確かめます（[templates.ja.md](templates.ja.md)）。format の無い文字を Markdown で描くのは `{{markdown .text}}` です。`any.json` の値は、描いた 1 つでなく、全部の種類の map で来ます（[見た目と共有の schema](#見た目と共有の-schema)）。
- 行が縦に並ぶ部品は、見た目 `rows` を借り（`looks: [rows]`）、行の余白を書かずに `rows` に任せます（上下は `--floe-row-pad`、左右は `--floe-pad-inline`）。`rows` を借りられない行は `var(--floe-row-pad)` を読みます。テーマはこれらの変数を替えません。詳しくは[見た目を借りる](#見た目を借りる)にあります。
- `floe part show <名前>` は、floe の部品も含めて、読むファイルの並びを `files:` の行に出し、`floe part show <名前> <ファイル>` はそのファイルを出します。合うものを floe の部品から写してください。

## part.html の関数

floe の部品が描くのに使う関数も使えます。floe の部品の `part.html` が使い方の見本で、`floe part show <名前> part.html` が出します（`floe part show code part.html`）。

### `copyButton`

`{{copyButton}}` は、写すボタンをページの言語で描きます。部品の箱の直下に置きます（`<figure data-floe-part="quote">…{{copyButton}}</figure>`）。写す範囲は、ボタンから上へ一番近い部品の箱で決まるので、自分の部品の中の部品の箱に置くと、自分のでなくその部品の中身を写します。部品の中の `data-floe-copy` を付けた要素の字を、どれも改行で終えて並べて写します（code は `<code>`、terminal はコマンドの 1 つずつに付けます）。見た目は floe のもので、置き場所と字の逃がし方は、code と同じく部品の CSS が書きます。部品の箱に `isolation: isolate`（ボタンをページの外の物の下に置くため）、ボタンのまわりに `margin: var(--floe-copy-gap)`、ボタンの下を字が通るなら、`:scope:has(> .floe-copy)` の中で、一番長い行の後ろに `padding-inline-end: var(--floe-copy-room)`（右端までスクロールすると、行の終わりがボタンの左に出る）。`--floe-copy-size`（ボタンの大きさ）・`--floe-copy-gap`・`--floe-copy-room` は floe がページの根に置き、部品には当てません。

### `diagram`

`{{diagram}}` は引数を取らず、部品の値の全体を図として描きます。値は、部品 `diagram` と同じ spec そのものでなければならず、横に別の鍵（`caption` など）を置くと、図の schema が断ります。

### `zoom`

`{{zoom}}` は、拡大の script を描きます。部品 `image` と `diagram` が使うものと同じです。部品の箱の中で、`class="floe-zoom"` の枠の前に置き、`part.yaml` で見た目 `zoom` を借ります（`looks: [zoom]`。`<figure data-floe-part="shot">{{zoom}}<div class="floe-zoom"><img src="{{.file}}" alt="{{.alt}}"></div></figure>`）。枠の最初の要素は、拡大するもの（`svg` か `img`）そのものにします（包む要素は動かしません）。`svg` は `viewBox` を、`img` は `transform` で動かし、大きさは `viewBox` か絵の大きさから取ります。見た目 `zoom` が `img` を枠の幅に縮めて並べるので（`display: block; max-width: 100%; height: auto`）、上の例をそのまま写せば拡大できます。拡大中は絵の枠線を透明にします。原寸より小さく描かれている間と拡大している間だけ、枠の前に道具の並びを出し、枠を Tab で止めます。枠の大きさは部品の CSS が決めます。script は使う部品ごとに入り、ページに何回あっても枠ごとに 1 回だけ動きます。枠に `title` を書くと、その `title` が残り、拡大の操作を伝える説明の代わりになります。枠の中の要素の `title` は、その要素に重ねている間、枠の説明より優先されます。拡大の案内の文は枠の次の要素として script が入れ、キーボードで枠を選んでいる間だけ出ます。そのため `.floe-zoom + p` のような隣接セレクタは文に当たらず効かなくなり、grid や flex の親に枠を直に置くと、選んでいる間だけ文が並びの新しい項目になります。枠はそれだけを持つ箱に入れてください。

### `script`

`{{script .js .data}}` は、要素と、関数 `.js` をそれと `.data` で呼ぶ script を描きます。部品 `script` と同じです。

### `diff`

`{{$d := diff .text}}` は、`git diff` が書く形の diff を読みます。`$d.Lines` はそれぞれ `.Kind` と `.Text` を持ち、`$d.Added` と `$d.Removed` は `add` と `del` の行の数です。`.Kind` は、`@@` で始まる行が `hunk`。`\` で始まる行（hunk の中も）、hunk の外の `--- `・`+++ ` で始まる行、`+`・`-`・空白のどれでも始まらない行（`diff --git`・`index` など。hunk も終える）が `meta`。それ以外の `+`・`-` で始まる行が、hunk の外でも `add`・`del`。空白で始まる行と空の行が `""` です。

### `code`

`{{$c := code .text .start .mark}}` は、部品 `code` と同じく字を行に分けます。`$c.Lines` のそれぞれに `.Text`（改行を含む）と `.Mark`（`.mark` がその番号を持つか）があります。`.mark` は行の番号の並びか、無し、`.start` は最初の行の番号か、無しです。`.start` を書くと `$c.Numbered` が true になり、`$c.Before` は最初の行の前の数、`$c.Digits` は最後の行の番号の桁数です。行の番号は整数で、ほかは断ります。範囲は、部品 `code`（1 から 1,000,000,000 まで）のように自分の schema で決めます。どの行にも当たらない番号は何もしません。

### `bars`

`{{range bars .}}` は、キーと値で書いた項目（`label`・`value`・`of`・`tone`）のリストを読みます。それぞれ `.Label`・`.Tone`・書いたままの `.Value` と `.Of`（`of` が無ければ空）、区切りも指数も無い 10 進の `.Now` と `.Max`（値と of。`aria-valuenow`・`aria-valuemax` に。`num` は桁を区切ります）、`.Width` で来ます。`.Width` は、`of` のある項目では `of` に対する値の割合、ほかの項目では、`of` の無い項目の中で一番大きい値に対する割合です（百分率）。`bars` は null の `of` を書いていないのと同じに読みます。部品の schema が `of` を数と決めていれば、null はその前に schema が断ります。キーと値で書いていない項目は誤りで、何番目の項目かを言います。数でないか 0 より小さい `value`、書いていないか null の `value`、数でないか 0 以下の `of`、`of` を超える `value` も誤りで、並びの中の棒の番号と label を言います。全部の項目の誤りを、部品の値の位置で一度に言います。値の形の誤りを項目や値そのものの位置で言いたいときは、部品 `bars` と同じく、部品の schema で、並びの項目を `bars` が取る形に決めてください（`minimum` だけでは、数でない値は `bars` まで来ます）。

```json
"items": {
  "type": "object",
  "properties": {
    "label": {"type": "string", "minLength": 1},
    "value": {"type": "number", "minimum": 0},
    "of": {"type": "number", "exclusiveMinimum": 0},
    "tone": {"$ref": "floe:///parts/tone.json"}
  },
  "required": ["label", "value"],
  "additionalProperties": false
}
```

これは、`part.html` が `bars` に渡す並びの schema に置きます。`bars .items` なら `properties.items` の下に `properties.items.items` として置き、根に置いても並びは確かめられません。

### `stats`

`{{range stats .}}` は、キーと値で書いた項目（`label`・`value`・`unit`・`note`・`tone`・`from`・`better`）のリストを読みます。それぞれ `.Label`、書いたままの `.Value`、`.Unit`・`.Note`・`.Tone`・`.From`、`better`・`worse`・`same` のどれかの `.Way` と `good`・`bad`・`muted` のどれかの `.WayTone`（`from` が無ければ空）、`value` から `from` を引いた符号付きの厳密な `.Change`（小数の桁は 2 つの多い方。同じなら空）になります。`from` があるときは、`from` と `value` は数字だけの字（`^-?[0-9]+([.][0-9]+)?$`）、`better` は `lower` か `higher` で、ほかは部品の値に置き、項目の番号と label を言って断ります。ほかの鍵は部品の schema が決めます。

### `chart`

`{{$c := chart .}}` は、部品の値の全体から部品 `chart` の座標を計算し、その `part.html` が SVG を書きます。あの部品のための関数で、欄と、欄が前提にする高さ・字・class は保証しません。座標を決められない値（`kind` が `line`・`bar` でない、`x` か `series` が空、`values` の数が `x` と違う、数でない値や `min`）は断ります。`values` の数の違いと、数でない値や `min` は書いた所に置き、ほかは部品の値に置きます。

### `written`

`{{written}}` は、部品の値を書いたままで返します。`part.html` が比べるか、pill の字のように字のまま描くためのもので、Markdown や HTML として描く項目は `.` から取ります。中の部品と format は描いていないので `eq`・`index` に渡せます。値が書いていない鍵は nil でなく無いので、`note` を省いた値で `{{(written).note}}` は誤りです（`index` か `with` で引きます）。floe の `table` は、列の `pills` の升をこれで比べます。

### `tree`

`{{range tree .}}` は、`path`（フォルダは `/` で終える）・`note`・`change` を持つ項目の並びを読み、パスが名指すフォルダに入れ子にします（並びに無いフォルダも作ります）。それぞれ `.Name`・`.Note`・`.Change`・`.Dir`（フォルダか）・`.Children` で来ます。

## 見た目と共有の schema

- 見た目だけのものは、`part.css`（と、`description:`・借りる見た目・`marks:` を書く `part.yaml`）だけで schema の無いディレクトリです。何も描かず、借りる部品や要素にスタイルを貸します。floe の見た目だけのものは `panel`・`rows`・`cards`・`band`・`numbered`・`marked`・`tags`・`zoom` です（`zoom` のほかのそれぞれが何を描くかは [templates.ja.md](templates.ja.md#借りられる見た目) にあります。`zoom` は拡大の枠と道具の並びで、関数 [`zoom`](#zoom) と一緒に部品が借ります）。
- 部品は `part.yaml` の `looks: [panel]` でほかの見た目を借り、その見た目が借りるものも、自分より先に順に借りて、その上で整えます。
- 共有の schema は、テンプレートか部品の schema が、いつもその根を名指す `<名前>.json`（`floe:///parts/<名前>.json`）です。floe のものは `tone.json`（`accent`・`good`・`warn`・`bad`・`muted`）と `any.json`（どの種類の部品でも 1 つ）です。その中の schema は、テンプレートと同じく、その根から確かめます。持つ部品と floe の format は、根から `properties` と `items` で辿れる所で、テンプレートから指しても部品から指しても描きます。
- `any.json` の値は、テンプレートの `page.html` と同じく、全部の種類の map で `part.html` に来ます。書いた種類には描いた部品が入り、ほかは nil です。`{{.x}}` と書くと map を字で書くので、`{{range .x}}{{.}}{{end}}` で描きます。schema の根が `any.json` なら `{{range .}}{{.}}{{end}}` です（1 つの種類だけを描く書き方は [templates.ja.md](templates.ja.md#テンプレートの中の部品)）。

## CSS の層と @scope

floe はページの CSS を `</head>` の前の `<style>` に、6 つのカスケードの層で入れます。後の層ほど、セレクタによらず前の層に勝ちます（`floe.parts` がいちばん強い）。

1. `floe.palette`：パレット。floe の `default` の上にテーマのもの。
2. `floe.base`：floe の土台。ページの根の地・色・書体と、文・リスト・リンク・コード・引用。
3. `floe.theme`：テーマの `base.css`。テンプレートの `page.css` と同じく scope され、ページと部品の箱には届きますが、部品の中と、text の Markdown（外枠 `data-floe-text` の中）には届きません。根は `:scope` で、`:root` や `html` は scope の外です。部品の中への規則（`table th` など）は、その部品のテーマのファイルに書きます。その後ろにテーマの `markdown.css` が入り、ページ全体（部品の中も）に届きます。どこにある Markdown の見た目もここに書きます。
4. `floe.looks`：部品の箱でなく見た目を借りる要素（`data-floe-look`）の見た目。層を分けているので、見た目を借りたテンプレートや部品は、セレクタの詳しさや範囲の近さによらず見た目を整えられ、テーマの `base.css` は見た目に勝てません。ただし `!important` を使うと別です（層の順が逆になります。floe は断りません）。
5. `floe.page`：テンプレートの `page.css`。その下の層に、floe がどのページにも置く body の余白 0 と列 `.floe-column` があり（`page.css` と同じ scope）、`page.css` が勝ちます。
6. `floe.parts`：部品のスタイル（それぞれ借りる見た目の後に）、その後にテーマの部品ごとのファイル。テーマのファイルは、その部品の `part.css` に、`@media print` の規則にも勝ちます。印刷で外す状態（選んだタブなど）は、部品が `@media screen` の中で、テーマが置く変数（下）から塗ります。

部品の `part.css` と、テーマのその部品のファイルは、その見た目を持つ要素から、その中の部品が持つものの手前までに scope されます。要素そのものは `:scope` と書きます。部品のスタイルは、中の部品の箱には届きますが、その部品の中には届きません。

部品の箱か、見た目を借りる並び（`ul`・`ol`・`menu`）は、印・余白・項目の間を持たず、grid で並びます。floe が `floe.parts` と `floe.looks` の一番下の層（`floe.parts.boxes`・`floe.looks.boxes`）でそうするので、どの部品と見た目の自分の規則も、その上に書けます。部品は、並びの見た目を外す規則を書きません。テンプレートの `page.css` やテーマの `base.css` が並び全部への規則を書いても、部品の箱のこの 4 つは変わりません（margin と、項目（`li`・`::marker`）への規則は届きます）。見た目を借りる要素の 4 つも、テーマのそうした規則では変わらず、借りるテンプレートの `page.css` が整えられます。箱を印のある並びにしたい部品は、`part.css` に `list-style` と余白を書くか、箱の中に並びを置きます。印の無い並びを読み上げで並びとして読ませたいときは（Safari は読まない）、`role="list"` を書きます。

部品は、色をパレットの変数からそのまま取り（`var(--accent)`）、自分の既定の値を持ちません。floe のページは、どれもパレットを持つためです。そのため、部品や見た目が読む CSS（`floe part show <名前>` の `files:` の行にあり、名前を足すと出る）は、パレットの無いページでは色が付きません。そうしたページに図を置くときは、`default` のパレットを持つ `floe diagram render` の断片のスタイルを使います（[commands.ja.md](commands.ja.md#floe-diagram-render-specyaml)）。また、箱が並びの部品（checklist・links・steps・timeline）の印と余白は、floe がページを描くときに外すので（上のとおり）、その CSS だけでは外れません。floe の外で描くページは、自分で書きます。

tone（`tone.json` のどれか）は、class でなく、その色で描く要素の `data-floe-tone` で付けます（`<span data-floe-tone="good" data-floe-part="cell">`）。floe は、その要素と、その中の部品の箱でないものに、tone の色を `--tone`・`--tone-soft` で置きます（パレットの `--good`・`--good-soft` から）。部品の箱とページは accent の色を持つので、部品は外の tone を継ぎません（部品の箱でない text の外枠は、外の tone を継ぎます）。テンプレートや見た目の要素に付けても同じです。部品は 2 つを 1 つの規則で読み（`background: var(--tone-soft); color: var(--tone)`）、tone ごとの規則は、tone の色でないもの（note の印、diagram の bad の箱の字）にだけ書きます。tone の無いものを別の色で描く部品は、根に 2 つを置きます（stats は `--muted`・`--muted-soft`）。形ごとに別の色で描く部品（diagram）は、根で 2 つを `initial` にし、形の色を `var()` の後ろに書きます（`var(--tone, var(--line))`）。tone に無い値の `data-floe-tone` は、付けないのと同じで、部品の箱なら accent の色、ほかの要素なら外の tone で描かれます。

自分の地を塗る見た目・部品（見た目 `panel`、`note`）は、地と一緒に字の色（`color: var(--fg)`）を書き、外の面を継ぎません。帯の中でも読めるようにするためです。自分の地の無い部品は、外の字の色（帯やテンプレートのもの）を継ぎます。テーマがその地を外すときは、swiss の `panel.css`・`note.css` のように、`:scope` と中の両方で戻します（`:scope { color: inherit; --face: inherit; --on-face: inherit; }` と `:scope > * { --face: inherit; --on-face: inherit; }`）。floe の規則はテーマの `:scope` と同じ詳しさで、テーマのファイルが後ろに入ります。自分の地を持つ部品は、テンプレートの字の色を継ぎません。札（`cell`）は、表や grid の列の中でも、どこでも 1 行に保ちます。長い札を折り返させたい所は、自分の要素に `--floe-pill-wrap: normal` を置きます（entries の tags のように）。部品の箱はこれを止めるので、中の部品には届きません。折り返した札は角の丸い札になります。角の丸さは `--floe-pill-end`（base.css）までで、1 行の札の半分の高さより大きいです。

自分の色で箱を塗り、中にリンク・行の code・フォーカスの輪が来る部品は、その箱を面にします。`--face` に地を、`--on-face` に字の色を置いて、それで塗ります（`background: var(--face); color: var(--on-face)`）。floe の土台が、中のリンクなどを `--on-face` で描きます。部品の箱そのものを面にするときは、floe の土台が部品の箱の中を外の面から切り離すので、`:scope, :scope > *` の両方に置きます（`code` のように）。地を持たない字に部品が色を付けるときは `var(--on-face, …)` で書きます（options の印の `color: var(--on-face, var(--good))` のように）。テーマが塗ったカードなど、外の面の中でも読めるようにするためです。

`@keyframes`・`@counter-style`・`@font-face` が作る名前は、scope に閉じず、ページ全体のものです。自分の部品の名前を頭に付けます（`mypart-fade`）。同じ名前を 2 つのファイルが作ると、使う所によらず、層の優先の高い方が勝ちます（部品はテンプレートに、外の部品は中の部品に勝つ）。`@font-face` が置き換わるのは、descriptors（`font-weight`・`font-style`・`font-stretch`・`unicode-range` など）が全部同じ face だけで、違うものは同じ family にまとまります。

部品は自分の箱に margin を書きません。部品の箱になりうるブロック（`p`・`ul`・`ol`・`dl`・`figure`・`form`・`menu`・`fieldset` など）の margin は floe の base が 0 にし、部品の間は置く側（テンプレートの節、stack、columns、note）が、隣と `--floe-room` あけて空けます。部品の箱に margin を書くと、置く側の間に勝ってしまいます。部品の中の Markdown の流れ（段落の間と、その中の部品の箱の前後。Markdown の表など）の間は、部品が決めます。

部品の間に、地や枠を持つ箱（カード、見た目 panel を借りる部品や要素、note）を描く部品・テンプレートは、floe のものと同じく、その箱の縁から字までの余白を `var(--floe-pad-block) var(--floe-pad-inline)`（floe の base で 1.1rem と 1.3rem）で書き、自分の長さを書きません。見た目 `panel` 自身は余白を持ちません（表・rows・details は、行を箱の縁まで引くため）。

部品は、本文より小さい字を、自分の数で書かず、floe の base の 2 つの段で書きます：札・チップ・番号の丸・キー・表の見出し・名前・日付・数・値の横の注は `var(--floe-text-small)`（0.8rem）、カードや手順の文・note の題・並びの上の小さな題・タブの名前・値の単位は `var(--floe-text-aside)`（0.9rem）。code の本文（行、tree の名前）は、code の大きさに従います。どちらも rem なので、大きな字の中でも小さい字は大きくなりません。

横にスクロールする自分の箱は、floe の `pre`・code・table・tree と同じく、`overflow-x: auto` と `contain: inline-size` を `@media screen` の中の 1 つの規則に書き、紙では中身を折り返します（紙ではスクロールした所が読めません。floe の `pre` は紙で行を折り返します）。書かないと、箱が中身の幅を外へ伝え、外の grid・flex の項目（並びの項目、カード）が一番長い行の幅まで広がり、ページも広がります。そうした箱は、中身で幅が決まる所（inline-block、float、絶対位置などの shrink-to-fit の箱、`min-content`・`max-content`・`fit-content` の幅、自動の幅の表の升、`justify-self: start`、flex の行の `flex-basis: auto` の項目）には置きません。そこでは枠と余白だけの幅に潰れます。箱の中で地を塗る要素は、画面で中身の端まで伸ばします（折り返す中身なら tree の並びと同じく `width: fit-content; min-width: 100%`、折り返さない行なら code の行と同じく `max-content`）。伸ばさないと、スクロールした所で地が箱の幅で切れ、中身が地の外に出ます。

入れ子では外が勝ちます。`floe.looks` と `floe.parts` はそれぞれ、スタイルが scope される要素（見た目を借りる要素か、部品の箱）の深さ（まわりにある部品の箱と、見た目を借りる要素の数）で層に分かれ、外側ほど強い層に入ります。そのため、見た目や部品のどの規則も、その中の要素に scope された見た目や部品の規則に、セレクタの詳しさや範囲の近さによらず勝ちます。見た目の中の、別の見た目を借りる要素への規則や、部品の中の部品の箱への規則だけでなく、そうした要素の中身への規則も勝ちます（cards の `.card :is(p, ul, ol)` は、`panel` を借りるカードの中の字に、table の `:scope :is(th, td)` は、中で自分も `table` を借りる要素のセルに勝ちます）。部品の規則は今までどおり、中の部品の箱までで、その部品の中には届きません。見た目と部品の間は上の層の順のままで、見た目のスタイルは、中の部品の箱に負けます。1 つの要素の上では、上のとおり、テーマの見た目のファイルが、それを借りる部品の `part.css` に勝ちます。

`page.css` と同じく、どのブロックも閉じない `}` と `@import` は、その行で断ります。

## 見た目を借りる

- 部品の箱でない要素は `data-floe-look` で見た目を借ります。`<section data-floe-look="panel">` はどのテーマでもパネルです。その見た目は部品とテンプレートより下の `floe.looks` に入るので、中に置かれた部品やテンプレートが整えられます。`cards` を借りる要素の中のカードは、自分でも `panel` を借りられます（`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`）。外の cards が、部品のカードと同じに描きます。
- floe の部品もこうして見た目を借ります。`compare`・`fields`・`findings` は table の、`stats`・`options`・`asks`・`facts`・`decision` は cards の、`diff`・`terminal` は code の、`checklist`・`entries`・`keys`・`links` は `rows` の、`steps` は `panel` とともに `numbered` の、`entries`・`steps` は札の要素に `tags` の、ほかのパネルに描く部品は `panel` の見た目を借ります。テーマのある見た目へのルールは、それを借りる部品を全部描きます。
- code の見た目を借りる部品は、code が根に置く変数を読みます：`--floe-code-pad`（1.1rem、行の左右の余白）、`--floe-code-room`（行の後ろの余白。部品が写すボタンを描けばその場所も）、`--floe-code-faint`（行の薄い字）、`--floe-code-head-faint`（頭の薄い字）。floe のもので、テーマが置くものではありません。余白は `pre` でなく、`pre` の子の要素である各行に付きます。行は要素にします（code と diff のように 1 つの `<code class="lines">` の中に 1 行 1 要素、1 つの `<code>` で全部を包む、terminal のように `pre` の中に 1 行 1 要素のどれか）。`pre` の直下の要素でない字には余白が付かず、その中の行内の要素の前後には余白が付きます。行に padding を書くと code の余白が消えるので、書きません。`<code class="lines">` の行は、code の見た目が地を行の端まで伸ばし、余白を各行へ移します（diff はこれで描きます）。行の形がさらに違う部品は、これらの変数で自分の行へ余白を移します（`:scope pre > code { padding-inline: 0; } :scope code > span { padding-inline: var(--floe-code-pad) var(--floe-code-room); }`）。code の見た目が block にし、行内の code の形（大きさ・地・影・余白）を外すのは、`pre` の子の `<code>` だけです。それ以外の `<code>`（頭の中、`pre` の子の要素の中）は、floe の base の行内の code の形のまま描かれます。
- 見た目 `rows` は、行ごとに上下に `--floe-row-pad`（floe の base で 0.4rem）の余白を付けます。rows の箱の `::after` も行として、上下に同じ余白が付きます（checklist の数）。左右には `--floe-pad-inline`（1.3rem）が付きます（`::after` も。そこに描く飾りにも左右の余白が付きます）。借りる部品やテンプレートは行の余白も、行の中の要素の左右の余白も書かず（行の中の `summary` や `div` に左右の余白を書くと、字が 2 倍下がります）、行の中で字を下げる所だけ、その幅を書きます。行の幅で押せる要素は、行の左右の余白の上へ広がり、同じ幅の余白を書きます（`margin-inline: calc(-1 * var(--floe-pad-inline)); padding-inline: var(--floe-pad-inline)`）。行が `rows` を借りない部品は、表のセルや facts のタイルのように変数を読み（`padding-block: var(--floe-row-pad)`）、ほかの行と高さをそろえます。
- 部品と見た目の `part.yaml` の `marks:` で、どの要素が左に印のある行かを言うと、floe が並べます。`box`（部品の箱。見た目なら、それを借りる要素と、それを取る部品の箱）、`items`（その子の `li` だけがそれぞれ行。`div` を並べた行には印が付きません）、`joined`（`items` に、印から次の印への線を足す）のどれかです。部品か見た目と、それが取る見た目（その先も）のうち `marks:` を書くのは 1 つで、2 つ目は `floe part check` が断ります。`marks:` を持つ見た目を取る見た目（`looks: [numbered]`）を借りる要素も行になります。HTML で 1 つの要素に 2 つを重ねないでください（`data-floe-look="numbered marked"`、`marked` を借りる `links` の箱）。両方の規則が行に当たり、断られません。
- 印のある行（numbered の丸、timeline の点、links の記号、note の印、`marked` の字）は、floe が `marks:` から層 `floe.marks` に並べます。列 `mark` と `text` を持つ grid で、`::before` が印として `mark` に立ち、大きさは枠を含めて `--floe-mark`、字との間は `--floe-mark-gap` で、行の中身と同じく最初の行のベースラインにそろいます。`joined` の行の `::after` は次の印への線で、印の下から行の終わりの 0.35rem 手前まで引きます。floe が書くのは置き場所と大きさだけで、色・丸さ・中身は書きません。部品は印の見た目（`content`・地・枠）と線の色を書き、行の中身の間を `> * + * { margin-top }` で空けます。`box`・`items` の行で `position` の無い `::after` は grid の項目で、行の中身の下に列 `mark` の行を 1 つ増やし、印より広いと字も動きます。印に重ねるなら `position: absolute` と `position-anchor: --floe-mark` を書きます。部品・テンプレート・テーマが行に `display`・`gap`・`grid-template` を書くと、floe の並びに勝ちます。並びの行の前に 1 つずつ置く字の印も、見た目 `marked`（`<ul data-floe-look="marked">` か `looks: [marked]`）で印のある行になります。部品は字と色（`li::before { content: "×" }`）を書き、項目の字は 1 つの要素に包みます（`<li><span>…</span></li>`。字と code が 1 行で流れるように）。1 つの題・字・升の前の記号（compare の →、プロンプト）は字の流れの中に置き、行ではありません。並びそのものの印（`list-style`・`::marker`）も、ブラウザが並べるので行ではありません。
- 部品を持つ行の高さいっぱいの線は、行の印ではありません。`marks:` が置くのは、`--floe-mark` の大きさで最初の行に揃う印です。線は、行を `position: relative` にして、行の `::before` を `position: absolute`・`inset-block: 0` で描きます。行の枠（`border`）では描きません。枠のある行は箱になり、中の部品が外へ描くものが、そこから出られないためです。
- CSS が `content` で描くものは、`/` の後ろに読み上げソフトが読む字を書きます。記号は `""`（`content: "✓"; content: "✓" / "";`。1 つ目は `/` を読まないブラウザのため）、読ませる数は `/` の前にある数え方を全部、順に（`content: counter(step) / counter(step);`。言葉なしでは中身と紛れる数（slides の頁）は `""`）で、字の文字列は書きません（`words.yaml` を通らないため）。読み上げソフトは記号を形の名前（チェックマーク）で読み、意味を言いません。記号だけが持つ意味は、部品が HTML に見えない字で書きます：`<span class="floe-hidden">{{word "done"}}</span>` と `words.yaml`。行の中の字の要素の中に置き（`<li><span><span class="floe-hidden">…</span>…</span></li>`）、行の子にはしません（行の中身の間がずれる。字の要素を持たない行（note の Markdown）は、行の最初の子に置き、中身の間を見えない字の後ろから数えます：`:scope > :not(.floe-hidden) + *`）。写すボタンが写す要素（`data-floe-copy`）の外に置きます。`.floe-hidden` は floe の base のもので、見えず、読み上げソフトには読まれます。

## テーマ

自分のテーマは、次のファイルのディレクトリです。要らないものは省きます。`floe theme list` がテーマを並べ、`floe theme show <名前>` が、floe のものも含めて、読むファイルの並びを出し、名前を足すとそのファイルを出します（`floe theme show ink palette.css`）。

| ファイル | 中身 |
|---|---|
| `theme.yaml` | `description:` に、そのテーマが何かを 1 行の文字列で書く。`floe theme list` と `floe theme show` が出し、管理ページの設定は、テーマのカードの下に書いたとおりに出す（`description: "Black on cream, flat primaries"`）。floe のテーマの説明は、管理ページではページの言語で出す。無ければ説明は空。ほかのキーは持たない。 |
| `palette.css` | [パレット](templates.ja.md#パレット)の変数と、テーマ自身の名前の変数を `:scope` に（ページの根。`:root` も同じ要素に当たる）。floe の `default` のパレットの上に重なるので、変えるものだけを書く。 |
| `base.css` | 部品の周りのページのルール。ページに当たり、部品の中と text の Markdown には当たらず、テンプレートの `page.css` と見た目の下に入る。根は `:root` や `html`（scope の外）でなく `:scope` と書き、部品の中はその部品のファイルに書く。テーマ自身の名前の変数は、ここの `:scope` に置いてもよい。 |
| `markdown.css` | Markdown の見た目（引用、見出し）。ページの上、text の中、部品の中の、どこの Markdown にも当たる。部品・見た目・テンプレートの CSS はどれもそれに勝つ。引用や見出しのように部品が描かない要素に書く。`p` や `ul` に書くと、部品が描く同じ要素にも当たる。 |
| `<名前>.css` | 部品か見た目 `<名前>` のスタイル。その `part.css` と同じく scope される。`panel.css` はどのパネルにも、`code.css` はコード・diff・ターミナルに届く。 |

- ほかの名前のファイルは断ります。
- 色は `palette.css` に 1 回だけ書き、ほかのファイルでは `var()`（`var(--accent)`）で使います。テンプレートが `:scope` に書く色とパレットが食い違わないためです。
- 行の上下の余白 `--floe-row-pad` と、箱の余白と行の左右の余白 `--floe-pad-block`・`--floe-pad-inline` は、floe の base のまま替えません。どのテーマでも、どの部品の行と箱も同じ余白になります。テーマは箱の余白を書きません。テーマが箱のまわりに描く外枠（default の灰色）はテーマのもので、その中で地を塗る要素が 2 つの変数を読みます。
- 部品の小さい字の 2 つの段 `--floe-text-small`・`--floe-text-aside` は、floe の base のまま替えません。テーマが部品の字に自分の個性で付ける大きさ（大きめの表の見出し、地の無い札など）は、部品ごとのファイルに数のまま書きます。
- 行の印（note の印、numbered の丸、timeline の点、links の記号、`marked` の字。`checklist` は `marked` を借りる）の大きさ（枠を含む）は `--floe-mark`、字との間は `--floe-mark-gap` で変えます。印の `top`・`left` や行の余白では動かしません。印は `content: none` で消し、そのとき印の列は幅を取りません。見た目 cards の番号の札も同じ約束で、大きさは `--floe-mark`（`:scope.numbered` に置く）、消すのは `.numbered > .card::before` の `content: none`、字の札は `--floe-mark: auto` と地なしです。`--floe-mark` を読む自分の部品・見た目は、floe の部品と同じく自分の根で置きます（まわりの値、たとえばカードの番号の大きさが、自分の印に届かないように）。
- テーマは部品の変数を、その変数を置くか読む部品・見た目のファイルの `:scope` に置きます。見た目のファイルは、その見た目を借りる部品のすべてに届き（`marked.css` は `checklist` と `decision` にも）、部品のファイルはその部品だけに届きます。部品が中の要素に置く変数（chart の `--c`、code の `--digits`）は部品のもので、テーマは置きません。テーマは `--checks-gap`（`checks` の項目の中の間）と `--entries-row-gap`（`entries` の題と札の行の間）も変えられます。
- カード（見た目 cards を借りる部品の `.card`）は見た目 panel も借ります。テーマの `panel.css` が描き、`cards.css` は違う所（地・枠・番号の札）を書きます。カードの余白は 2 つの変数のもので、テーマは書きません。カードの中の間は `--cards-gap`（0.4rem）で、テーマはこの変数を `cards.css` の `:scope` に置いて変え、`.card` に `gap` を書きません（`parts` のある options は行の間を 0 にするので、書いた `gap` は誤りも出ずに効きません）。最初のカードもほかと同じ余白で、列の左端にそろえるために余白を 0 にしません。
- テーマは傾けず、言葉を書きません。CSS の字は `words.yaml` を通らず、ページの言語で変わらないためです。変形（`transform`・`-webkit-transform`・`-moz-transform`）は `translate()`・`translateX()`・`translateY()` で動かすか `none` だけで、`rotate`・`scale`・`zoom`・`offset-path`・`offset-rotate`・`perspective` と、回す関数を持つ変数は書きません。文字列は記号だけで、字は書体の名前（`font-family`・`font`・`--font`・`--display`・`--mono`）にだけ書き、エスケープでなくそのまま書きます。`content` に `attr()` を書きません。数え方（`counter()`・`counters()`・`list-style`・`list-style-type`）は `decimal`・`decimal-leading-zero` と形（`none`・`disc`・`circle`・`square`）だけで、変数で書かず、`@counter-style` も書きません。
- テーマの飾りは場所を取りません。余白を広げたり、地の色や透明の枠で場所を作ったりせず、今ある余白の上に重ねて描きます（地、`--floe-inset`、重ねる `::before`）。部品の余白と争わないためです。枠を太くするのは枠そのものの形（brutal の枠、editorial の上の線、washi の二重線）で、飾りの場所ではありません。部品がフォーカスの輪や選んだ印を `outline` で描く箱（options のカード）には、飾りを `outline` で描きません。輪がその outline に置き換わり、飾りが消えます。washi のカードの内側の線のように、`--floe-inset` か `::before` で描きます。
- 下を透かす面のテーマ（aurora のガラス）は、band の中では面を不透明にします（aurora のように自分の `band.css` で）。band は accent で塗られ、それが透けて字が沈むためです。
- テーマは `@keyframes` と `@font-face` の名前に、テーマの名前を頭に付けます（`washi-fade`）。名前はページ全体のもので、テンプレートの `page.css` の同じ名前はテーマの `base.css` と `markdown.css`（どちらも `floe.theme`）に勝ち、テーマの部品ごとのファイルのものはそのどれにも勝ちます。
- テーマも tone を `var(--tone)`・`var(--tone-soft)` で描き、tone ごとの規則を書きません。tone をテーマの色で塗るテーマ（brutal・notebook）は、`palette.css` に `--good-fill` などで 1 回だけ置き（tone の無いものは `--accent-fill`）、`var(--tone-fill)` で描きます。floe が `--<tone>-fill` から置き、テーマが名指さない tone では `--<tone>-soft` になります。テーマの部品ごとのファイルは、部品の `part.css` と同じ深さの層に後ろから入るので、同じ詳しさなら勝ち、詳しいセレクタなら詳しさで決まります。
- 表の頭、選んだタブ、code の頭は、部品が変数から塗ります。どれも置かれていなければ floe の値で読み、テーマはその部品のファイルの `:scope`（ページ全体なら `base.css` の `:scope`）に置きます。表の頭は `--table-head-bg` と `--table-head-fg`（地は頭の行に塗るので、グラデーションも行に 1 本で流れます）、行の間の線は `--table-rule`（線なしは `none`）。選んだタブは `--tabs-picked-bg`、`--tabs-picked-fg`、`--tabs-picked-ring`（`box-shadow`）。code の頭（diff・terminal も）は `--code-head-bg` と `--code-head-fg`、その上の diff の数は `--code-head-add` と `--code-head-del`、写すボタンの地は `--code-ground`（code の行は code の地の上にあります。テーマが `:scope pre` で行を自分の地で塗るときは、同じ色を `--code-ground` に置きます）。テーマは、状態（`:checked`）や印刷を知る規則も、ほかの部品のために頭の色を選び直す規則も書きません。code の頭を明るくするテーマは、数の色も一緒に置きます。terminal の頭は、テーマがどの色を置いても `--code-head-fg` の 85% で薄く描かれます。頭の `color` を規則で書くテーマでは、terminal の頭もその色になります。選んだタブの輪は、テーマが全部のタブの名前に付けた影の上に描かれます。選んだタブにもその影を付けるなら、輪に置きます。頭の線と字の形（書体・大きさ・太さ）は、テーマの規則（`:scope thead th`・`:scope figcaption`）のままです。テーマが自分の地で塗る頭は面（`--face`・`--on-face`）にします。
- 箱の中の静かな地（表の頭、棒の溝、キー）は `var(--quiet)` です。箱をそれに近い地で塗るテーマは、その箱で `--quiet` を置き直します（default は、灰色の panel の上では白、その中の白い面の上では灰色に）。
- グラフの形も変数で、それぞれ未設定なら floe の値で読みます。テーマは自分の `chart.css` の `:scope` に置き、場所を取りません：`--chart-line`・`--chart-dash`（線の太さと破線）、`--chart-dot`・`--chart-dot-fill`・`--chart-dot-edge`・`--chart-dot-line`（点の半径、塗り、縁、縁の太さ）、`--chart-bar-fill`・`--chart-bar-edge`・`--chart-bar-line`（棒の塗り、枠、枠の太さ）、`--chart-ground`（線の場所の地。箱の模様が目盛りとぶつかるときに）。`currentColor` が系列の色です。線は 3px、点の半径は 5px、点の縁と棒の枠は 2px までにします（字はその外に置かれます）。枠より細い棒は枠なしで描きます。
- 角の丸さはパレットのものです。テーマは `palette.css` に `--radius`・`--radius-small`・`--radius-pill` を 1 つの長さで書き、ほかのファイルは丸さを書きません。ほかの形の角（notebook の手書きなど）は、`palette.css` か `base.css` の `:scope` にテーマ自身の名前の変数で置き、テーマのファイルが読みます。3 つに 4 つの角や `/` を入れると、1 つの角として読む規則や `calc()`・`min()` の中で無効になります。角は 2 つの変数の小さい方を取ってもよいです（札の `min(var(--radius-pill), var(--floe-pill-end))`）。自分の部品も 3 つを読み、形（`0`・`50%`・`999px`）のほかに長さを書きません。
- 影は、テーマの 1 組です。`palette.css` に `--floe-shadow-x`・`-y`・`-blur`・`-spread` で 1 回だけ書き（px か rem。em は箱の字の大きさで変わり、% は影が無くなります）、影を落とす箱に、部品か見た目のファイルで色 `--floe-shadow-color` を置きます（`.card { --floe-shadow-color: var(--fg); }`）。色は中の箱に届きません。箱の中に描く影は `--floe-inset` に置きます（打ち消すのは `initial`。`none` を書くと、箱の影も一緒に消えます）。部品の箱・見た目を借りる要素・引用に `box-shadow` を書きません。floe の base がこれらから書き、上から書くと消えます。
- 箱の外へ描くもの（影、外へ出る疑似要素、outline）のために場所は空けません。四方とも、floe が部品の間に空ける `--floe-room`（0.75rem）以下にし、ほかの箱やページの端に届かないようにします。大きさは箱の大きさによらない値にします。傾きや拡大のように、箱が大きいほど外へ出る変形は使いません。テーマは部品の間を書きません。間はどこも `--floe-room` で、部品とテンプレートが決めます。テーマの `cards.css` や `tabs.css` が根に `gap` を書くと、間がずれます。
- データファイルは、floe のテーマと同じく `theme:` で選びます。

## 確かめる

`floe part check` は、自分の部品・見た目・共有の schema・テーマを floe のものの上で全部読み、すべての誤りを出します（終了コード 1）。ファイル、名前、借りる見た目、自分の見た目を借りる部品、schema が宣言しない `checks.yaml` のパス、部品と共有の schema の schema、CSS の誤りです。部品はデータが無いので描きません。それを使うテンプレートの見本は `floe template check` が描きます。
