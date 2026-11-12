# テーマ

自分のテーマの作り方です。テーマはページの色・書体・部品の形を決めます。

## テーマの置き場所

- 自分のテーマは `FLOE_THEMES_DIR`（既定は `$XDG_CONFIG_HOME/floe/themes`、無ければ `~/.config/floe/themes`）に、1 つずつディレクトリで置きます。直下のファイルは読まず、何も言いません。テーマのファイルは、その名前のディレクトリの中に置いてください。
- 名前は英小文字・数字・`-`・`_` で、英小文字か数字で始めます。floe のテーマが持つ名前は断ります。自分のものは floe のものと並べて置き、置き換えることはありません。
- その中のディレクトリはシンボリックリンクにできません。ファイルはできます。

```sh
floe theme new calm                   # 埋めて使う最小のテーマ calm（空の palette.css）を作る
floe theme check                      # 自分のテーマと、そのファイルが前提にする部品を全部確かめる
floe theme show default palette.css   # 自分のテーマが上に重なる floe のパレット
```

誤りが 1 つあると、`floe template guide` の「テンプレートの置き場所」のとおり、どのページも止めます。`floe theme check` が並べます。書いている間は、別の置き場所で作り、確かめてから移してください（`floe template guide` の「別の置き場所で作る」）。`FLOE_THEMES_DIR` をそこに向け、`floe theme check` で確かめます。

テーマの部品・見た目のファイル（`<名前>.css`）は、その部品の CSS で、`part.css` と同じ範囲に当たります。`floe part guide` が部品の CSS について言うこと（「CSS の層と @scope」）は、そのまま当たります。下は、テーマだけの決まりです。

## テーマのファイル

自分のテーマは、次のファイルのディレクトリです。要らないものは省きます。`floe theme new <名前>` が、埋めて使う空の `palette.css` で作ります。`floe theme list` がテーマを並べ、`floe theme show <名前>` が、floe のものも含めて、読むファイルの並びを出し、名前を足すとそのファイルを出します（`floe theme show ink palette.css`）。

| ファイル | 中身 |
|---|---|
| `theme.yaml` | `description:` に、そのテーマが何かを 1 行の文字列で書く。`floe theme list` と `floe theme show` が出し、管理ページの設定は、テーマのカードの下に書いたとおりに出す（`description: 'Black on cream, flat primaries'`）。floe のテーマの説明は、管理ページではページの言語で出す。無ければ説明は空。ほかのキーは持たない。 |
| `palette.css` | パレット（`floe template guide` の「パレット」）の変数と、テーマ自身の名前の変数を `:scope` に（ページの根。`:root` も同じ要素に当たる）。floe の `default` のパレットの上に重なるので、変えるものだけを書く。 |
| `base.css` | 部品の周りのページのルール。ページに当たり、部品の中と text の Markdown には当たらず、テンプレートの `page.css` と見た目の下に入る。根は `:root` や `html`（scope の外）でなく `:scope` と書き、部品の中はその部品のファイルに書く。テーマ自身の名前の変数は、ここの `:scope` に置いてもよい。 |
| `markdown.css` | Markdown の見た目（引用、見出し）。ページの上、text の中、部品の中の、どこの Markdown にも当たる。部品・見た目・テンプレートの CSS はどれもそれに勝つ。引用や見出しのように部品が描かない要素に書く。`p` や `ul` に書くと、部品が描く同じ要素にも当たる。 |
| `<名前>.css` | 部品か見た目 `<名前>` のスタイル。その `part.css` と同じく scope される。`panel.css` はどのパネルにも、`code.css` はコード・diff・ターミナルに届く。 |

- ほかの名前のファイルは断ります。
- 色は `palette.css` に 1 回だけ書き、ほかのファイルでは `var()`（`var(--accent)`）で使います。テンプレートが `:scope` に書く色とパレットが食い違わないためです。
- 行の上下の余白 `--floe-row-pad` と、箱の余白と行の左右の余白 `--floe-pad-block`・`--floe-pad-inline` は、floe の base のまま替えません。どのテーマでも、どの部品の行と箱も同じ余白になります。テーマは箱の余白を書きません。テーマが箱のまわりに描く外枠（default の灰色）はテーマのもので、その中で地を塗る要素が 2 つの変数を読みます。
- 部品の小さい字の 2 つの段 `--floe-text-small`・`--floe-text-aside` は、floe の base のまま替えません。テーマが部品の字に自分の個性で付ける大きさ（大きめの表の見出し、地の無い札など）は、部品ごとのファイルに数のまま書きます。
- 行の印（note の印、numbered の丸、timeline の点、links の記号、`marked` の字。`checklist` は `marked` を借りる）の大きさ（枠を含む）は `--floe-mark`、字との間は `--floe-mark-gap` で変えます。印の `top`・`left` や行の余白では動かしません。印は `content: none` で消し、そのとき印の列は幅を取りません。見た目 cards の番号の札も同じ約束で、大きさは `--floe-mark`（`:scope.numbered` に置く）、消すのは `.numbered > .card::before` の `content: none`、字の札は `--floe-mark: auto` と地なしです。
- テーマは部品の変数を、その変数を置くか読む部品・見た目のファイルの `:scope` に置きます。見た目のファイルは、その見た目を借りる部品のすべてに届き（`marked.css` は `checklist` と `decision` にも）、部品のファイルはその部品だけに届きます。部品が中の要素に置く変数（chart の `--c`、code の `--digits`）は部品のもので、テーマは置きません。テーマは `--checks-gap`（`checks` の項目の中の間）と `--entries-row-gap`（`entries` の題と札の行の間）も変えられます。
- カード（見た目 cards を借りる部品の `.card`）は見た目 panel も借ります。テーマの `panel.css` が描き、`cards.css` は違う所（地・枠・番号の札）を書きます。カードの余白は 2 つの変数のもので、テーマは書きません。カードの中の間は `--cards-gap`（0.4rem）で、テーマはこの変数を `cards.css` の `:scope` に置いて変え、`.card` に `gap` を書きません（`parts` のある options は行の間を 0 にするので、書いた `gap` は誤りも出ずに効きません）。最初のカードもほかと同じ余白で、列の左端にそろえるために余白を 0 にしません。
- テーマは傾けず、言葉を書きません。CSS の字は `words.yaml` を通らず、ページの言語で変わらないためです。変形（`transform`・`-webkit-transform`・`-moz-transform`）は `translate()`・`translateX()`・`translateY()` で動かすか `none` だけで、`rotate`・`scale`・`zoom`・`offset-path`・`offset-rotate`・`perspective` と、回す関数を持つ変数は書きません。文字列は記号だけで、字は書体の名前（`font-family`・`font`・`--font`・`--display`・`--mono`）にだけ書き、エスケープでなくそのまま書きます。`content` に `attr()` を書きません。数え方（`counter()`・`counters()`・`list-style`・`list-style-type`）は `decimal`・`decimal-leading-zero` と形（`none`・`disc`・`circle`・`square`）だけで、変数で書かず、`@counter-style` も書きません。
- テーマの飾りは場所を取りません。余白を広げたり、地の色や透明の枠で場所を作ったりせず、今ある余白の上に重ねて描きます（地、`--floe-inset`、重ねる `::before`）。部品の余白と争わないためです。枠を太くするのは枠そのものの形（brutal の枠、editorial の上の線、washi の二重線）で、飾りの場所ではありません。部品がフォーカスの輪や選んだ印を `outline` で描く箱（options のカード）には、飾りを `outline` で描きません。輪がその outline に置き換わり、飾りが消えます。washi のカードの内側の線のように、`--floe-inset` か `::before` で描きます。
- 下を透かす面のテーマ（aurora のガラス）は、band の中では面を不透明にします（aurora のように自分の `band.css` で）。band は accent で塗られ、それが透けて字が沈むためです。
- テーマは `@keyframes` と `@font-face` の名前に、テーマの名前を頭に付けます（`washi-fade`）。名前はページ全体のもので、テンプレートの `page.css` の同じ名前はテーマの `base.css` と `markdown.css`（どちらも `floe.theme`）に勝ち、テーマの部品ごとのファイルのものはそのどれにも勝ちます。
- テーマも tone を `var(--tone)`・`var(--tone-soft)` で描き、tone ごとの規則を書きません。tone をテーマの色で塗るテーマ（brutal・notebook）は、`palette.css` に `--good-fill` などで 1 回だけ置き（tone の無いものは `--accent-fill`）、`var(--tone-fill)` で描きます。floe が `--<tone>-fill` から置き、テーマが名指さない tone では `--<tone>-soft` になります。テーマの部品ごとのファイルは、部品の `part.css` と同じ深さの層に後ろから入るので、同じ詳しさなら勝ち、詳しいセレクタなら詳しさで決まります。
- 表の頭、選んだタブ、code の頭は、部品が変数から塗ります。どれも置かれていなければ floe の値で読み、テーマはその部品のファイルの `:scope`（ページ全体なら `base.css` の `:scope`）に置きます。表の頭は `--table-head-bg` と `--table-head-fg`（地は頭の行に塗るので、グラデーションも行に 1 本で流れます。グラデーションや、位置と大きさの付いた背景の層を置いた頭は、面（`--face`）にできません）、行の間の線は `--table-rule`（線なしは `none`）。選んだタブは `--tabs-picked-bg`、`--tabs-picked-fg`、`--tabs-picked-ring`（`box-shadow`）。code の頭（diff・terminal も）は `--code-head-bg` と `--code-head-fg`、その上の diff の数は `--code-head-add` と `--code-head-del`、写すボタンの地は `--code-ground`（code の行は code の地の上にあります。テーマが `:scope pre` で行を自分の地で塗るときは、同じ色を `--code-ground` に置きます）。テーマは、状態（`:checked`）や印刷を知る規則も、ほかの部品のために頭の色を選び直す規則も書きません。code の頭を明るくするテーマは、数の色も一緒に置きます。terminal の頭は、テーマがどの色を置いても `--code-head-fg` の 85% で薄く描かれます。頭の `color` を規則で書くテーマでは、terminal の頭もその色になります。選んだタブの輪は、テーマが全部のタブの名前に付けた影の上に描かれます。選んだタブにもその影を付けるなら、輪に置きます。頭の線と字の形（書体・大きさ・太さ）は、テーマの規則（`:scope thead th`・`:scope figcaption`）のままです。テーマが自分の地で塗る頭は面（`--face`・`--on-face`）にします。
- 箱の中の静かな地（表の頭、棒の溝、キー）は `var(--quiet)` です。箱をそれに近い地で塗るテーマは、その箱で `--quiet` を置き直します（default は、灰色の panel の上では白、その中の白い面の上では灰色に）。
- グラフの形も変数で、それぞれ未設定なら floe の値で読みます。テーマは自分の `chart.css` の `:scope` に置き、場所を取りません：`--chart-line`・`--chart-dash`（線の太さと破線）、`--chart-dot`・`--chart-dot-fill`・`--chart-dot-edge`・`--chart-dot-line`（点の半径、塗り、縁、縁の太さ）、`--chart-bar-fill`・`--chart-bar-edge`・`--chart-bar-line`（棒の塗り、枠、枠の太さ）、`--chart-ground`（線の場所の地。箱の模様が目盛りとぶつかるときに）。`currentColor` が系列の色です。線は 3px、点の半径は 5px、点の縁と棒の枠は 2px までにします（字はその外に置かれます）。枠より細い棒は枠なしで描きます。
- 角の丸さはパレットのものです。テーマは `palette.css` に `--radius`・`--radius-small`・`--radius-pill` を 1 つの長さで書き、ほかのファイルは丸さを書きません。ほかの形の角（notebook の手書きなど）は、`palette.css` か `base.css` の `:scope` にテーマ自身の名前の変数で置き、テーマのファイルが読みます。3 つに 4 つの角や `/` を入れると、1 つの角として読む規則や `calc()`・`min()` の中で無効になります。角は 2 つの変数の小さい方を取ってもよいです（札の `min(var(--radius-pill), var(--floe-pill-end))`）。
- 影は、テーマの 1 組です。`palette.css` に `--floe-shadow-x`・`-y`・`-blur`・`-spread` で 1 回だけ書き（px か rem。em は箱の字の大きさで変わり、% は影が無くなります）ます。
- テーマは部品の間を書きません。間はどこも `--floe-room` で、部品とテンプレートが決めます。テーマの `cards.css` や `tabs.css` が根に `gap` を書くと、間がずれます。
- データファイルは、floe のテーマと同じく `theme:` で選びます。

## 確かめる

`floe theme check` は `floe part check` と同じことをします。テーマの `<名前>.css` は部品があることを前提にするので、部品とテーマは一緒に確かめます。テーマのファイル・名前・CSS の誤りと、自分の部品・見た目・共有の schema の誤りを全部出します（終了コード 1）。
