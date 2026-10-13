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
├── schema.json      # データファイルに書ける項目（JSON Schema）
├── example.yaml     # 書き始めに使うデータファイル
├── checks.yaml      # 項目どうしの関係（任意）
├── computed.yaml    # データから計算する値（任意）
├── assets/          # page.html が asset で出す画像（任意）
└── themes/
    ├── default.css  # 既定のテーマ（必須）
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
| `.Lang` | ページの言語（`--lang`。既定は `en`） |
| `.Data` | データファイルの項目（`template`・`theme`・`title` を除く）。スキーマで宣言していてデータに無い項目は nil なので、`{{with .Data.lead}}` と書ける。スキーマで宣言していない名前は描画を止める。`"format": "html"` の項目は HTML としてそのまま入る |
| `.Calc` | `computed.yaml` の値（名前で引く） |
| `.Theme` | テーマの CSS。`<style>{{.Theme}}</style>` のように使う |
| `markdown` | Markdown の文字列を HTML にする（`{{markdown .text}}`）。中の生の HTML は除き、表は `<div class="table-wrap">` で包む |
| `num` | 数を桁区切りで書く（`{{.Calc.total \| num}}` は `1,234,567`）。有効数字で丸めることもできる（`{{.Calc.rate \| num 3}}` は `0.00648`） |
| `bytes` | バイト数を二進の単位で書く（`{{.Calc.size \| bytes}}` は `1.5 KiB`） |
| `asset` | テンプレートの `assets/` の画像を data URI にする（`<img src="{{asset "logo.svg"}}">`） |

## schema.json

データファイルの項目の [JSON Schema](https://json-schema.org/)（2020-12）です。キーの打ち間違いを断るため、`"additionalProperties": false` を書きます。`template`・`theme`・`title` は floe が読むので、宣言しません。

`format` には、floe が検査する JSON Schema の format（`date`・`email`・`uri` など）、`html`、`image` を使います。それ以外の format は、この floe ではどんな値も通してしまうので、`floe template check` が誤りにします。`html` と `image` は floe が探す `properties` と `items` の下だけに書きます。`anyOf` や `$defs` などの下に書くと、その項目は文字列として描かれます。

`"format": "image"` の項目には、画像のパスを、データファイルのディレクトリからの相対で、その配下に、`..` やドットで始まる名前を使わずに書きます。画像は data URI としてページに入るので、`<img src="{{.Data.chart}}">` と書けて、ページだけで表示が完結します。

## 画像

画像は、データのものも `assets/` のものも、png・jpeg・gif・webp・svg のファイルで、名前と中身の両方で判定し、1 MiB までです。データの画像の誤りは、その行と列で出します。`floe template check` は、`assets/` のすべての画像と、`page.html` が `asset` に渡すすべての名前を、見本が通らない分岐の中のものも含めて確かめます。`example.yaml` が書く画像はテンプレートのディレクトリから読むので、見本から始めたデータファイルには、その画像も隣に要ります。

## checks.yaml

スキーマでは書けない、項目どうしの関係です。リストの要素 1 つずつに検査を当てます。

```yaml
- for: access_patterns            # リストまでのキーのパス
  field: key                      # 落ちたときに指す、要素のキー（任意）
  expr: item.key in map(doc.attributes, .name)
  message: キーには属性一覧にある属性を使う
```

`expr` は、成り立つべき [expr](https://expr-lang.org/) の式です。リストの要素を `item`、データファイルの項目を `doc` で参照します。落ちた要素は、`field` の位置にその値を添えて出します。

```text
orders.yaml:13:12: access_patterns[0].key: キーには属性一覧にある属性を使う（「user_id」）
```

`for`・`field` のパスは、`schema.json` の `properties` と `items` で宣言されている必要があります。打ち間違えたパスはデータの中に検査するものが見つからず、何も検査しません。`floe template check` は `properties` と `$ref` を辿り、キーを持たないスキーマがほかのキーを断る（`"additionalProperties": false`、`"unevaluatedProperties": false`、または `object` 以外の `type`）位置のパスと、型に `array` を含まない `for` を誤りにします。`patternProperties`・`allOf`・`anyOf`、許したほかのキーなどで宣言されうるパスは通します。`checks.yaml` そのものの誤りは、`checks.yaml` の行と列で出します。データファイルと同じく、アンカー・エイリアス・タグは使えません。

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
2. テンプレート、テンプレートが要る floe の版、テーマ、テンプレートの `schema.json`、画像
3. テンプレートの `checks.yaml`
4. テンプレートの `computed.yaml`
5. `page.html` の描画
