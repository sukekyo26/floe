# 部品とテーマ

自分の部品・見た目・テーマの作り方です。部品はデータファイルの項目を描き、テーマはページの色・書体・部品の形を決めます。テンプレートで部品を使うには [templates.ja.md](templates.ja.md#テンプレートの中の部品)、データファイルに部品を書くには [pages.ja.md](pages.ja.md#部品) を見てください。

## 部品とテーマの置き場所

- 自分の部品・見た目・共有の schema は `FLOE_PARTS_DIR`（既定は `$XDG_CONFIG_HOME/floe/parts`、無ければ `~/.config/floe/parts`）に置きます。部品と見た目はディレクトリ、共有の schema はその隣のファイル `<名前>.json` です。直下のほかのファイルは読まず、何も言いません。部品のファイルは、その名前のディレクトリの中に置いてください。
- 自分のテーマは `FLOE_THEMES_DIR`（既定は `$XDG_CONFIG_HOME/floe/themes`、無ければ `~/.config/floe/themes`）に、1 つずつディレクトリで置きます。直下のファイルは読まず、何も言いません。テーマのファイルは、その名前のディレクトリの中に置いてください。
- 名前は英小文字・数字・`-`・`_` で、英小文字か数字で始めます。floe の部品・見た目・共有の schema・テーマが持つ名前は断ります。自分のものは floe のものと並べて置き、置き換えることはありません。部品と見た目には、テーマがどのページにも入れるファイルの名前 `base`・`palette` も付けません。
- その中のディレクトリはシンボリックリンクにできません。ファイルはできます。

```sh
floe part new quote     # 埋めて使う最小の部品 quote を作る
floe part check         # 自分の部品・見た目・共有の schema・テーマを全部確かめる
floe part show quote    # 見た目・schema・checks を出す
```

自分の部品・テーマ・テンプレートに誤りが 1 つあると、使うかどうかによらず、どのページの描き（`floe check`・`floe render`・`floe publish`）も止めます。見落とさないためです。`floe part check` と `floe template check` が全部を並べます。書いている間は、別の置き場所で作り、確かめてから移してください（[templates.ja.md](templates.ja.md#別の置き場所で作る)）。`FLOE_PARTS_DIR` か `FLOE_THEMES_DIR` をそこに向け、`floe part check` で確かめます。

## 部品のファイル

```text
quote/
├── schema.json    # データファイルが書けるもの（JSON Schema）
├── part.html      # 描き方。Go の html/template
├── part.css       # スタイル（空でもよい）
├── part.yaml      # 借りる見た目（任意）
├── checks.yaml    # 項目どうしの関係（任意。テンプレートのものと同じ書き方）
└── words.yaml     # 言葉、言語ごと（任意。テンプレートのものと同じ書き方）
```

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- schema は、floe の部品と同じく `"$ref": "floe:///parts/quote.json"` で名指します。部品の schema も、floe の部品と共有の schema をこう名指します。
- 部品の要素は箱で、`data-floe-part="quote"` で自分を名乗ります。
- `part.html` は項目を `.` で見て、テンプレートの `page.html` と同じ関数を使えます（`asset` は除く。[templates.ja.md](templates.ja.md#pagehtml)）。
- floe の部品が描くのに使う関数も使えます。floe のリポジトリ（https://github.com/sukekyo26/floe/tree/develop/internal/kit/floe/parts）の floe の部品の `part.html` が、使い方の見本です。
  - `{{copyButton}}` は、写すボタンをページの言語で描きます。部品の箱の直下に置きます（`<figure data-floe-part="quote">…{{copyButton}}</figure>`）。写す範囲は、ボタンから上へ一番近い部品の箱で決まるので、自分の部品の中の部品の箱に置くと、自分のでなくその部品の中身を写します。部品の中の `data-floe-copy` を付けた要素の字を、どれも改行で終えて並べて写します（code は `<code>`、terminal はコマンドの 1 つずつに付けます）。見た目は floe のもので、置き場所と字の逃がし方は、code と同じく部品の CSS が書きます。部品の箱に `isolation: isolate`（ボタンをページの外の物の下に置くため）、ボタンのまわりに `margin: var(--floe-copy-gap)`、ボタンの下を字が通るなら、`:scope:has(> .floe-copy)` の中で、一番長い行の後ろに `padding-inline-end: var(--floe-copy-room)`（右端までスクロールすると、行の終わりがボタンの左に出る）。`--floe-copy-size`（ボタンの大きさ）・`--floe-copy-gap`・`--floe-copy-room` は floe がページの根に置き、部品には当てません。
  - `{{diagram}}` は引数を取らず、部品の値の全体を図として描きます。値は、部品 `diagram` と同じ spec そのものでなければならず、横に別の鍵（`caption` など）を置くと、図の schema が断ります。
  - `{{script .js .data}}` は、要素と、関数 `.js` をそれと `.data` で呼ぶ script を描きます。部品 `script` と同じです。
  - `{{$d := diff .text}}` は、`git diff` が書く形の diff を読みます。`$d.Lines` はそれぞれ `.Kind` と `.Text` を持ち、`$d.Added` と `$d.Removed` は `add` と `del` の行の数です。`.Kind` は、`@@` で始まる行が `hunk`。`\` で始まる行（hunk の中も）、hunk の外の `--- `・`+++ ` で始まる行、`+`・`-`・空白のどれでも始まらない行（`diff --git`・`index` など。hunk も終える）が `meta`。それ以外の `+`・`-` で始まる行が、hunk の外でも `add`・`del`。空白で始まる行と空の行が `""` です。
  - `{{range bars .}}` は、キーと値で書いた項目（`label`・`value`・`tone`）のリストを読みます。それぞれ `.Label`・`.Tone`・書いたままの `.Value`・`.Width`（一番大きい値に対する割合を百分率で）で来ます。キーと値で書いていない項目は誤りで、部品の値の位置で、何番目の項目かを言います。数でないか 0 より小さい `value` も誤りで、同じ位置で、その値を言います。誤りを項目や値そのものの位置で言いたいときは、部品 `bars` と同じく、部品の schema に `"items": {"type": "object", "properties": {"value": {"type": "number", "minimum": 0}}, "required": ["value"]}` と書いて断ってください（`minimum` だけでは、数でない値は `bars` まで来ます）。
  - `{{range tree .}}` は、`path`（フォルダは `/` で終える）・`note`・`change` を持つ項目の並びを読み、パスが名指すフォルダに入れ子にします（並びに無いフォルダも作ります）。それぞれ `.Name`・`.Note`・`.Change`・`.Dir`（フォルダか）・`.Children` で来ます。
- 部品の schema は、テンプレートと同じく floe の `format`（`html`・`markdown`・`image`）を取ります。floe が辿る `properties` か `items` の下か、根で宣言します。format を持つ共有の schema も取ります。その項目は `part.html` に描いたものが来ます（`html` は書いたとおり、`markdown` は HTML に、`image` は data URI に。`<img src="{{.file}}" alt="{{.alt}}">`）。`{{.h}}` と書き、関数（`markdown`・`markdownLine`・`printf`・`slice`・`eq` など）に渡したり、`<script>` の中に置いたりしません（書いた字でなく HTML なので、そこでは正しく動きません）。`html` の項目は、テンプレートの項目と同じく描いたページで確かめます（[templates.ja.md](templates.ja.md)）。format の無い文字を Markdown で描くのは `{{markdown .text}}` です。`any.json` の値は、描いた 1 つでなく、全部の種類の map で来ます（[見た目と共有の schema](#見た目と共有の-schema)）。
- `floe part show <名前>` は、floe の部品も含めて取るものを出します。合うものを floe の部品から写してください。

## 見た目と共有の schema

- 見た目だけのものは、`part.css`（と、`description:` と借りる見た目を書く `part.yaml`）だけで schema の無いディレクトリです。何も描かず、借りる部品や要素にスタイルを貸します。floe の見た目だけのものは `panel`・`rows`・`band`・`numbered` です。`band` はアクセントの色で塗った帯で、中の字・リンク・code・フォーカスの輪を、塗った面の上の字の色にします。`numbered` は、リストの項目に丸い番号を付け、次の項目へ線でつなぎます（部品 `steps` と runbook の手順が借ります）。
- 部品は `part.yaml` の `looks: [panel]` でほかの見た目を借り、その見た目が借りるものも、自分より先に順に借りて、その上で整えます。
- 共有の schema は、テンプレートか部品の schema が、いつもその根を名指す `<名前>.json`（`floe:///parts/<名前>.json`）です。floe のものは `tone.json`（`accent`・`good`・`warn`・`bad`）と `any.json`（どの種類の部品でも 1 つ）です。その中の schema は、テンプレートと同じく、その根から確かめます。持つ部品と floe の format は、根から `properties` と `items` で辿れる所で、テンプレートから指しても部品から指しても描きます。
- `any.json` の値は、テンプレートの `page.html` と同じく、全部の種類の map で `part.html` に来ます。書いた種類には描いた部品が入り、ほかは nil です。`{{.x}}` と書くと map を字で書くので、`{{range .x}}{{.}}{{end}}` で描きます。schema の根が `any.json` なら `{{range .}}{{.}}{{end}}` です（1 つの種類だけを描く書き方は [templates.ja.md](templates.ja.md#テンプレートの中の部品)）。

## CSS の層と @scope

floe はページの CSS を `</head>` の前の `<style>` に、5 つのカスケードの層で入れます。後の層ほど、セレクタによらず前の層に勝ちます（`floe.parts` がいちばん強い）。

1. `floe.palette`：パレット。floe の `default` の上にテーマのもの。
2. `floe.base`：floe の土台。ページの根の地・色・書体と、文・リスト・リンク・コード・引用。
3. `floe.looks`：部品の箱でなく見た目を借りる要素（`data-floe-look`）の見た目。
4. `floe.page`：テンプレートの `page.css`。
5. `floe.parts`：部品のスタイル（それぞれ借りる見た目の後に）、その後にテーマの `base.css` と部品ごとのファイル。

部品の `part.css` と、テーマのその部品のファイルは、その見た目を持つ要素から、その中の部品が持つものの手前までに scope されます。要素そのものは `:scope` と書きます。部品のスタイルは、中の部品の箱には届きますが、その部品の中には届きません。

`page.css` と同じく、どのブロックも閉じない `}` と `@import` は、その行で断ります。

## 見た目を借りる

- 部品の箱でない要素は `data-floe-look` で見た目を借ります。`<section data-floe-look="panel">` はどのテーマでもパネルです。その見た目は部品とテンプレートより下の `floe.looks` に入るので、中に置かれた部品やテンプレートが整えられます。
- floe の部品もこうして見た目を借ります。`compare`・`fields`・`findings`・`matrix`・`risks` は table の、`stats`・`options`・`facts`・`links`・`decision` は cards の、`diff`・`terminal` は code の、`checklist`・`faq`・`glossary`・`keys` は `rows` の、ほかのパネルに描く部品は `panel` の見た目を借ります。テーマのある見た目へのルールは、それを借りる部品を全部描きます。

## テーマ

自分のテーマは、次のファイルのディレクトリです。要らないものは省きます。

| ファイル | 中身 |
|---|---|
| `palette.css` | [パレット](templates.ja.md#パレット)の変数と、テーマ自身の名前の変数を `:root` に。floe の `default` のパレットの上に重なるので、変えるものだけを書く。 |
| `base.css` | どのページにも入るルール（Markdown の引用など）。 |
| `<名前>.css` | 部品か見た目 `<名前>` のスタイル。その `part.css` と同じく scope される。`panel.css` はどのパネルにも、`code.css` はコード・diff・ターミナルに届く。 |

- ほかの名前のファイルは断ります。
- 色は `palette.css` に 1 回だけ書き、ほかのファイルでは `var()`（`var(--accent)`）で使います。テンプレートが `:scope` に書く色とパレットが食い違わないためです。
- データファイルは、floe のテーマと同じく `theme:` で選びます。

## 確かめる

`floe part check` は、自分の部品・見た目・共有の schema・テーマを floe のものの上で全部読み、すべての誤りを出します（終了コード 1）。ファイル、名前、借りる見た目、自分の見た目を借りる部品、schema が宣言しない `checks.yaml` のパス、部品と共有の schema の schema、CSS の誤りです。部品はデータが無いので描きません。それを使うテンプレートの見本は `floe template check` が描きます。
