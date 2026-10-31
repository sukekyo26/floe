# テンプレート

自分のテンプレート（データファイルを描き入れるページ）の作り方です。データファイルは `template:` でテンプレートを名指します（書き方は [pages.ja.md](pages.ja.md)）。floe のテンプレートは `report`・`design`・`handoff`・`adr`・`postmortem`・`runbook`・`slides` です（`floe template list` が並べます）。自分の部品やテーマの作り方は [parts.ja.md](parts.ja.md) にあります。

## テンプレートの置き場所

自分のテンプレートは、`FLOE_TEMPLATES_DIR`（既定は `$XDG_CONFIG_HOME/floe/templates`、無ければ `~/.config/floe/templates`）の中の、テンプレートの名前のディレクトリです。floe のテンプレートと同じ名前なら、floe のものを隠します。`FLOE_TEMPLATES_DIR` の直下のファイルは読まず、何も言いません。テンプレートのファイルは、その名前のディレクトリの中に置いてください。

```sh
floe template new memo     # 埋めて使うテンプレート memo を作る
floe template check        # 自分のテンプレートを全部、example.yaml を描いて確かめる
floe template show memo    # テーマと、読むファイルを全部出す（schema と見本が先）
```

自分のテンプレート・部品・見た目・共有の schema・テーマに誤りが 1 つあると、そのテンプレートで描くかどうかによらず（`report` のページも）、直すまでどのページも止めます。`floe check`・`floe render` も、サーバーの publish とリンクしたページもです。`floe part check` と `floe template check` が全部を並べます。公開したページはサーバーのテンプレートで描きます（[commands.ja.md](commands.ja.md#サーバー)）。テンプレートを変えたら、`floe rebuild` が保存したページを描き直します。

### 別の置き場所で作る

サーバーは、テンプレートを保存するとすぐ読み直すので、書きかけのテンプレートは、同じサーバーを使う人みんなのページを止めます。新しいテンプレートは別の置き場所で書き、確かめてから移してください。

```sh
export FLOE_TEMPLATES_DIR=~/.config/floe/staging-templates
mkdir -p "$FLOE_TEMPLATES_DIR"
floe template new memo                      # ~/.config/floe/staging-templates/memo を作る
floe template check                         # 誤りが無くなるまで memo を直す
floe render --out-dir /tmp/memo notes.yaml  # template: memo のデータファイル。/tmp/memo/notes.html を開く
unset FLOE_TEMPLATES_DIR
mkdir -p ~/.config/floe/templates
mv ~/.config/floe/staging-templates/memo ~/.config/floe/templates/
```

- `floe template check` と `floe render` は、サーバーを使わずに描きます。変数を設定したまま `floe publish` しないでください。動いているサーバーは、起こしたときの置き場所で描くので断り、動いていなければ、publish が別の置き場所でサーバーを起こします。
- `mv` の前に `~/.config/floe/templates` を作ってください。無いと、`mv` が `memo` の名前を `templates` に変え、そのファイルがテンプレートの置き場所の直下に並びます。floe は直下のファイルを読まず、ページが `memo` を名指すまで何も言いません。
- 別の置き場所は、テンプレートと同じファイルシステムに置いてください。`mv` が 1 回で入れ替え、サーバーが写しかけを読むことがありません。
- 今あるテンプレートを変えるときも、別の置き場所に写して直し、確かめてから、古いものと入れ替えます。
- 部品も同じに作ります。`FLOE_PARTS_DIR` と、`floe part new`・`floe part check` を使います。テーマには `new` が無いので、`FLOE_THEMES_DIR` の中にディレクトリを手で作り（[parts.ja.md](parts.ja.md#テーマ)）、`floe part check` で確かめます。

## ファイル

```text
memo/
├── page.html        # ページ。Go の html/template
├── schema.json      # データファイルが持てる項目（JSON Schema）
├── example.yaml     # 書き始めのデータファイル。floe template check が描く
├── page.css         # 部品の周りのページの CSS（任意）
├── checks.yaml      # 項目どうしの関係（任意）
├── computed.yaml    # データから計算する値（任意）
├── words.yaml       # テンプレートの言葉、言語ごと（任意）
└── assets/          # page.html が asset で見せる画像（任意）
```

ほかの名前のファイルは断ります。`example.yaml` は、そのディレクトリのテンプレートを名指します（`template: memo`）。

## page.html

`page.html` は Go の [html/template](https://pkg.go.dev/html/template) で、入れるものをエスケープします。`floe template new` は次のものを書きます。floe が受ける最小のものに viewport の `<meta>` を足したもので、viewport が無いと、スマホは直に開いたページを 980px の幅で描いて縮めます。charset は書かなくてかまいません。サーバーを通さずに開くページ（`floe render` の出力、ダウンロード）には floe が頭の先頭に 1 つ置き、サーバーはヘッダーで渡します。

```html
<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
</head>
<body>
</body>
</html>
```

見えるもの：

| 名前 | 中身 |
|---|---|
| `.Title` | ページの題。データファイルの `title:`、無ければ slug |
| `.Lang` | ページの言語。データファイルの `lang:` |
| `.Data` | データファイルの項目。floe が自分で読む `lang`・`template`・`theme`・`title` は除く。schema が宣言し、データが省いた項目は nil。`{{with .Data.lead}}`・`{{range}}`・`{{if}}` は空として読むが、`len`・`index`・`slice` は断る（`len of nil pointer`）ので、`with` の中で書く：`{{with .Data.done}}{{len .}}{{else}}0{{end}}`。部品は描いた形で来る（[テンプレートの中の部品](#テンプレートの中の部品)）。`"format": "markdown"` の項目は HTML、`"format": "image"` の項目は data URI で来る |
| `.Calc` | `computed.yaml` の値。名前で引く |
| `markdown` | Markdown の文を HTML に（`{{markdown .text}}`）。中の生の HTML は除く |
| `markdownLine` | 表のセルのような Markdown の 1 行を、段落で囲まない HTML に |
| `num` | 数を桁区切りで（`{{.Calc.total \| num}}` は `1,234,567`）、または有効数字で丸めて（`{{.Calc.rate \| num 3}}` は `0.00648`） |
| `bytes` | バイト数を 2 進の単位で（`{{.Calc.size \| bytes}}` は `1.5 KiB`） |
| `asset` | テンプレートの `assets/` の画像を data URI で（`<img src="{{asset "logo.svg"}}">`） |
| `word` | テンプレートの [`words.yaml`](#wordsyaml) の言葉を、ページの言語で（`{{word "status_doing"}}`） |

- ページは `<!doctype html>` で始め、`</head>` を持ちます。floe はその前にページの CSS を入れます。
- 描いたページは、`page.html`・部品・データの項目の誰が書いても、`<style>`（CSS は `page.css` に書く）、`<base>`、`<noscript>`、charset と viewport のほかの `<meta>` を持てません。ページを描くときに、データファイルの `template:` の所で断ります。
- 描いたページの見出し（`<h1>`〜`<h6>`）で `id` の無いものには、文から作った `id` を付けます。そこへのリンク（`#1-what-happens`）に使えます。
- データファイルから描いたページは、見せるものを全部持ち、スクリプトはページの中身を外へ送れません。CSP が読ませるのは、data URI の画像・フォント・メディア、インラインのスタイルとスクリプト、Google Fonts、cdnjs・jsDelivr・unpkg のライブラリだけです。`fetch`・XHR・ほかの所の画像・フォームは止めます。CSP はうっかりを防ぐもので、悪意のあるテンプレートは防げません。信頼できるテンプレートを使ってください。

## page.css

`page.css` は、部品の周りのページを並べます（部品の置き場所、ページの幅、ヘッダー）。部品のスタイルとテーマの部品ごとのファイルより下の層に入るので、部品の形は変えません。テーマの `base.css` より上の層です。

- floe は `@scope (:root) to ([data-floe-part] > *)` で包みます。部品の箱までは届いて部品を置けますが、部品の中には届きません。
- 根は `:scope` です。ページの変数は `:scope` に書きます。`:root` や `html` は scope の外です。
- どのブロックも閉じない `}` と `@import` は、その行で断ります。Google Fonts のようなスタイルシートは `page.html` の `<link>` で読みます。
- パレットの変数（`var(--muted)`）で描くと、データファイルが選んだテーマに従います。
- 続けて並ぶ部品の間（節の中、項目の中）は `var(--floe-room)`（0.75rem）以上をあけます。テーマの影が箱の外へ出る、一番大きい長さです。

たとえば `report` の `page.css` は、floe のテンプレートがどれも使う列で、ページを中央にそろえます：

```css
main {
  max-width: calc(var(--page-width) + 2 * var(--page-pad)); margin-inline: auto; padding-inline: var(--page-pad);
  display: grid; gap: 2.5rem;
}
section > h2 { border-top: var(--rule); padding-top: 1.3rem; }
.lead { color: var(--muted); font-size: 1.05rem; }
```

## テンプレートの中の部品

部品はデータの項目を描きます。`floe part list` が並べ、`floe part show <名前>` が取るものを出します。schema は `properties` か `items` の下で、`$ref` で名指します。

```json
"findings": { "$ref": "floe:///parts/findings.json", "description": "What the review found" }
```

`page.html` は項目を置きたい所に置きます（`{{.Data.findings}}`）。floe は部品の schema と checks で確かめ（誤りはデータファイルの行と列で言う）、HTML で描きます。

部品 `text` は、ほかの部品と並び、横に `code`・`terminal` を置ける所の Markdown です。floe は中の囲んだコードのブロックを警告し、`code` に書くよう勧めます。code の部品を置かない、ただの Markdown の欄は、`{"type": "string", "format": "markdown"}` にします。

種類を名指す鍵 1 つの部品は、種類によらず `any.json` を指します。そのリストは、report の節と同じく、`items` で `any.json` を指します。テンプレートの根の schema では、次のとおりです。

```json
"part":  { "$ref": "floe:///parts/any.json" },
"parts": { "type": "array", "items": { "$ref": "floe:///parts/any.json" } }
```

その値は、全部の種類の map で `page.html` に来ます。書いた種類には描いた部品が入り、ほかは nil です。`{{.Data.part}}` と書くと map を字で書く（`map[bars:<nil> …]`）ので、range で回します。項目を省いたときは、range は何もしません。

```html
{{range .Data.part}}{{.}}{{end}}
{{range .Data.parts}}{{range .}}{{.}}{{end}}{{end}}
{{with .Data.part}}{{with .note}}<aside>{{.}}</aside>{{end}}{{end}}
```

最後の行は、部品が note のときだけ描きます。このように、種類より先に項目を確かめます。`{{.Data.part.note}}` は、項目を省くと失敗します。

### 借りられる見た目

テンプレートの要素は、`data-floe-look` で見た目を借ります。どのテーマでも同じに描きます（決まりは [parts.ja.md](parts.ja.md#見た目を借りる) にあります）。

- `panel`：ほとんどの部品を描く箱（`<section data-floe-look="panel">`）。
- `rows`・`table`：`checklist` と `table` の見た目。
- `cards`：`class="card"` の子を横に並べ、子はそれぞれ `data-floe-look="panel"` でパネルにし、題を `class="title"` の要素（見出しなら `<h3 class="title">`）で書きます。`class="numbered"` を付けると、場所の番号を付けます（`options` と同じ）。
- `band`：アクセントの色で塗った帯。中の字・リンク・code・フォーカスの輪を、塗った面の上の字の色にします。design の結論や handoff の頭のように。
- `numbered`：`<ol>` に付けると、項目に丸い番号を付けて線でつなぎます。部品 `steps` と runbook の手順のように。項目の字は要素で包みます（`<li><p>Restart <code>api</code> now</p></li>`）。項目に直に書いた字は、字の続きと要素ごとに grid の別の項目になって崩れます。項目は字で始めます。字の無いもの（画像、空のブロック）で始まると、丸はその下の縁に来ます。

## パレット

ページの色と書体は、テーマの `palette.css` が決める CSS の変数で、部品・図・floe の土台・テンプレートが使います。floe の `default` のパレットが全部を定めて、どのページにも入り、データファイルが選んだテーマがその上に重ねます。

| 変数 | 中身 |
| --- | --- |
| `--bg`・`--surface` | ページの地、部品の面 |
| `--fg`・`--muted` | 文字、薄い文字 |
| `--line` | 線と枠 |
| `--accent`・`--accent-soft` | 目立つ色と、その淡い地 |
| `--good`・`--warn`・`--bad` と `-soft` | 良い・注意・悪いと、その淡い地 |
| `--on-fill` | `--accent` やトーンの色で塗った面の上の文字（pill など） |
| `--shadow` | 影の色の候補。影は、テーマが箱に `--floe-shadow-color` を置いたときだけ描かれる（[parts.ja.md](parts.ja.md#テーマ)） |
| `--floe-shadow-x`・`-y`・`-blur`・`-spread` | テーマの 1 組の影。px か rem（既定は無し） |
| `--font`・`--display`・`--mono` | 本文・見出し・等幅の書体 |
| `--code-bg`・`--code-fg`・`--code-prompt` | コードとターミナルの地（いつも暗い）、その文字、プロンプト |
| `--code-add`・`--code-del` | diff の足した行と消した行 |
| `--page-bg` | ページ全体の地。方眼のような模様も（既定は `--bg`） |
| `--rule` | 節の間のような、ページを横切る線の `border` |
| `--text-width` | 1 列のページの幅。テンプレートが列の幅をここから決める（`adr`・`runbook` のように）。部品 note の幅にも使う。文そのものは、入っている列か箱の幅まで使う |
| `--page-width`・`--page-pad` | ページを中央にそろえる列の幅と、その左右の余白。余白は箱の内側なので、列は `max-width: calc(var(--page-width) + 2 * var(--page-pad)); margin-inline: auto; padding-inline: var(--page-pad)` |

floe の土台は、ページの根にこれらで地・色・書体を描きます（`:root { background: var(--page-bg); color: var(--fg); font-family: var(--font); }`）。`page.css` は `:scope` でこれを変えられます。自分のテーマも変えられます（[parts.ja.md](parts.ja.md#テーマ)）。印刷でも、どの地も画面のとおりに出ます。

帯や塗った注意のように、色で自分を塗り、中に字を持つ要素は、`--face` に地を、`--on-face` に字の色を置き、それで自分を塗ります（`background: var(--face); color: var(--on-face)`）。floe の土台は、中のリンク・行の code（地なしの枠）・フォーカスの輪を `--on-face` で描きます（アクセントでは塗った地に溶けるため）。部品の中は、部品の地で初めから描きます。パネルの上の引用のように、自分で明るい地を塗る要素も、中を初めから描きます（`blockquote > * { --face: initial; --on-face: initial; }`）。地を塗らずに色を決めるときは、塗った面の中でも読めるように `var(--on-face, …)` と書きます。

## schema.json

データファイルの項目の [JSON Schema](https://json-schema.org/)（2020-12）です。書き損じた鍵を断るように `"additionalProperties": false` を書きます。`floe template new` が作る `schema.json` には無いので、自分で足します。

- `lang`・`template`・`theme`・`title` は floe が読むので、宣言せず、必須にもしません。
- floe に文の描き方を伝える `format` は `markdown`・`html`・`image` です。floe が辿る `properties` か `items` の下で宣言します。
- `$ref` は、部品か共有の schema の全体（`floe:///parts/table.json`）を `properties` か `items` の下で、または自分のファイルの中の場所（`#/$defs/row`）を名指します。floe の外（`file://`・`https://`）や部品の中を指すものは断ります。schema の根には `$id` を書きません。

`"format": "markdown"` の項目は Markdown の文で、`{{.Data.text}}` が HTML で描きます。

`"format": "html"` の項目は、テンプレートのものも部品のものも、そのまま入ります。ブラウザが読む描いたページで、要素を開いたままにすることも、周りのページの要素を閉じることもできず、項目の所で断ります。要素は丸ごと書きます。`page.html` か、その項目を持つ部品の `part.html` は、項目を要素を置ける所に置き、HTML が働かない文字や属性の中（`<title>`・`title="…"`）には置きません。

`"format": "image"` の項目は画像のパスで、データファイルのディレクトリからの相対パス、その下だけです（[画像](#画像)）。

## words.yaml

テンプレートが自分で描く言葉（状態の名前など）を、言語ごとに書きます。

```yaml
en:
  status_doing: In progress
ja:
  status_doing: 進行中
```

鍵は言語のタグです。`{{word "status_doing"}}` は、ページの `lang:` の言葉を描き（`ja-JP` は `ja` を使う）、ファイルにその言語が無ければ `en` の言葉を描きます。その言語に無い言葉は、`words.yaml` を名指して描くのを止めます。

## checks.yaml

schema では言えない項目どうしの関係を、リストの各項目に、またはデータファイル全体に 1 回、確かめます。

```yaml
# リストへの鍵のパス（sections は各節）
- for: sections
  # 外れたときに指す、項目の鍵（任意）
  field: heading
  expr: item.heading != nil || len(item.parts ?? []) < 2
  message: give a section of more than one part a heading
```

- `for` の途中のリストは 1 つずつ辿ります。`sections.parts` は各節の部品を確かめ、外れたものはその項目で言います。
- `for` が無ければ、データファイル全体を `item` として 1 回確かめます。
- `expr` は、成り立つべき [expr](https://expr-lang.org/) の式です。リストの項目を `item`、データファイルの項目を `doc` で見ます。外れた項目ごとに、`field` の所で `message` を言います。
- `for` のパスは schema が宣言するものにします。`field` は確かめません。
- ` #` を含む式（`filter(xs, # > 1)` の expr の `#` など）は引用符で囲みます。YAML がそこからコメントと読むからです。

## computed.yaml

データから計算し、`page.html` が `.Calc.<名前>` で見せる値です。

```yaml
item_bytes: sum(attributes, .avg_bytes)
storage_gb: item_bytes * records.in_1_year / 1e9
```

- 式は、schema が宣言するデータファイルの項目と、ほかの計算した値を、書いた順によらず名前で見ます。
- 名前は、expr と `page.html` がどちらも 1 つの名前と読むもので、データの項目や expr の言葉・関数とは違うものにします。互いから計算する値は断ります。
- データからは出せない値（0 での割り算、式が要る項目が省かれている）や、有限の数でない値は、データファイルを名指して式の所で言います。省かれた項目の代わりの値は `??` で（`count(prs ?? [], .state == "merged")`）、その中へは `?.` で（`records?.in_1_year ?? 0`）書きます。

## example.yaml

書き始めのデータファイルです。`floe template show` が出し、`floe template check` が `floe check` と同じに描きます。`page.html` の全部に届くように、テンプレートが取る項目を全部書きます。

## 画像

データの画像と `assets/` の画像は、png・jpeg・gif・webp・svg のファイル（名前で見分ける）で、1 MiB までです。データの画像は [pages.ja.md](pages.ja.md#画像) のとおりに名指します。`assets/` の画像は `assets/` の中のパスで、名前の間は `/` 1 つ、`./`・`..` を使わず、ドットで始まる名前の下にも置きません。floe はそれを読まないからです（`assets/ has no ".x/a.png"`）。

- `floe template check` は `assets/` の画像を全部読みます。
- `example.yaml` が名指す画像は、テンプレートのディレクトリから読みます。見本から書き始めたデータファイルは、その隣に画像が要ります。

## 確かめる順

データファイルは次の順に確かめ、前の段を通ったデータだけを次の段にかけます。`floe check` は全部を走らせます。

自分の部品・見た目・共有の schema・テーマ・テンプレートは、その前に読みます（[テンプレートの置き場所](#テンプレートの置き場所)）。

1. YAML と、名指したテンプレートとテーマ
2. テンプレートの `schema.json`
3. テンプレートの `checks.yaml`、部品が描く項目ごとのその部品の `checks.yaml`、テンプレートの `computed.yaml`（まとめて）
4. 描いたページ：部品と format の項目（画像・図・HTML）ごとに、それから `page.html`、最後に描いたページの全体
