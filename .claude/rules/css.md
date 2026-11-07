---
paths:
  - "**/*.css"
  - "internal/kit/floe/**"
  - "internal/draw/style.go"
---

# CSS（変数の一覧と細かい形は docs/parts.md と docs/templates.md）

- **行の余白は `--floe-row-pad` と `--floe-pad-inline`**：線で区切った行の上下と左右の余白はこの 2 つだけが決める。見た目 rows を借りる側は行の余白を書かず、テーマは替えない。`TestUIRowPad`
- **箱の中の余白は `--floe-pad-block` と `--floe-pad-inline`**：部品の間に地や枠を持つ箱を描くときは、縁から字までをこの 2 つで書き、数で書かない（箱の中の小さな升、行、code の行は外）。見た目 panel 自身は余白を持たない。テーマは置き直さず、箱の余白を書かない。`TestUIBoxPad`
- **部品の小さい字は 2 つの段**：本文より小さい字は数で書かず、`--floe-text-small`（札・印と名前・付記）か `--floe-text-aside`（添えの文・小さな題）で書く。code の本文と diagram の SVG は外。テーマは置き直さない。`TestTextScale`
- **折り返す並びは、並びの位置で描き分けない**：幅で折り返す並びの子の形（枠・余白）を `:first-child`・`:nth-child` などで描き分けない（どの子が行の先頭になるかは幅で決まる）。並んだ順に地の色を替えるのはよい。`TestUIWrappedAlike`
- **写すボタンは `copyButton` 1 つ**：part.html の `{{copyButton}}` だけが描き、写す要素は `data-floe-copy` で決める。部品ごとに script・ボタン・言葉・ボタンの形を書かない。部品の CSS が決めるのは置き場所だけで、大きさと間は base.css の `--floe-copy-*`
- **拡大は `zoom` 1 つ**：枠の中で拡大する図と絵は、`part.html` の `{{zoom}}` と、見た目 `zoom` の枠 `.floe-zoom` で描く。部品ごとに script・道具の並び・言葉を書かない。部品の CSS が書くのは枠の大きさと中身の形だけ
- **code の一族は code の変数で描く**：code の見た目を借りる部品（diff・terminal）は、行の余白と薄い字を code が根に置く `--floe-code-*` で描く。行は `pre` の子の要素か、`<code class="lines">` の子の要素にし、行に padding を書かず、数と `:has(> .floe-copy)` を写さない（行に書くと code の余白が黙って消える）。テーマは置き直さない。`TestCodeLookCopiesNothing`
- **opacity は 0 だけ**：floe の CSS は `opacity` を 0 にだけ使い、薄く描くのは色（`color-mix(… transparent)`）で書く。`TestOpacity`
- **CSS の記号は読ませず、数は読ませてよい**：`content` は `/` の後ろに読み上げを書く。記号は `/ ""`、数は前の `counter()` の並びで、字の文字列は書かない。記号だけが持つ意味は `.floe-hidden` の見えない字で書く（言葉は words.yaml）。`TestMarkSpeech`・`TestUISpeech`
- **本文の列は `.floe-column`**：テンプレートの本文の列は class で付け、列の字（max-width・margin-inline・padding-inline）を page.css に書かない。幅は `--column-width` で、置くのは page.css だけ（テーマは置かない）。`--page-width`・`--text-width`・`--page-pad` はパレット 1 か所だけ。字の大きさはページで決めない。列は見た目にしない
- **部品はパレットの変数をそのまま読む**：部品・見た目の CSS は `var(--accent)` のように既定なしで読み、値を写さない。パレットの根は `:scope`。パレットの無いページへ出す CSS は自分の根だけに当てる（`diagramAssets`。`TestUIDiagramFragment`）
- **tone は `data-floe-tone` と、floe が tone.json から組み立てる変数**：tone の印は class でなく `data-floe-tone`。名前を色に写すのは style.go の `Tones` だけで、部品・テーマ・テンプレートは `--tone`・`--tone-soft`・`--tone-fill` を読む。tone ごとのセレクタは tone の色でないものにだけ書く。テーマが自分の色で塗るときはパレットに `--<tone>-fill` を置く。`TestUITone`
- **部品の箱と見た目の並びは floe が外す**：`ul`・`ol`・`menu` の印・左の余白・項目の間は style.go の `boxes` が外し、grid で並べる。部品と見た目は並びの見た目を外す規則を書かない。`TestUILists`
- **色で塗った帯は見た目 `band` 1 つ**：テンプレートは、`--accent` で塗った帯を page.css に書かず、`data-floe-look="band"` で借りる。page.css が書くのは帯の余白と中の並びだけ
- **塗った面は `--face` と `--on-face`**：地と字の色を自分で決め、中にリンク・行の code・フォーカスの輪が来る面は、2 つの変数を置いて `background: var(--face); color: var(--on-face)` で塗る。中の色は base.css 1 か所が決め、面ごとに書かない。部品の中身は base.css が変数を外すので、部品の根を面にするときは根と根の子に置く。自分の地を塗る見た目・部品は、地と一緒に `color: var(--fg)` を書き、面の変数を外す。地を塗らずに字や輪の色を書くときは、面の中に来うるなら `var(--on-face, …)` で書く。部品の箱の字の色を外から直す規則は書かない。印刷の地は base.css の `print-color-adjust` 1 か所。`TestUIBand`・`TestUIContrast`
- **CSS の勝ち負けは層で決まる**：セレクタの詳しさでなく、floe が並べる層で決まる（順は `draw.pageStyle` のコメント。`TestUILayers`・`TestUINesting`・`TestUIPrint`）。テーマの base.css はページの段で部品の箱と text の外枠で止まり、Markdown の見た目は markdown.css に書く（ページ全体に届くので、部品が描かない要素だけ）。部品・見た目の名前に `base`・`palette`・`markdown` は使えない
- **状態と頭はテーマが変数で塗る**：表の頭、選んだタブ、code の頭は、部品が `var(--x, 既定)` で読む変数を、テーマがその部品のファイルの `:scope` に置く。テーマは状態（`:checked`）と印刷を知る規則を書かない。画面だけの状態は部品が `@media screen` で塗る
- **テンプレートが部品の形で描くものは、部品の箱として置く**：札や注意の囲みは見た目として借りず、page.html に部品の箱（`data-floe-part="cell"`・`"note"`）で置き、色は `data-floe-tone`。page.css は箱の置き場所だけを書く。書き手の自由な字は cell にしない（長さに上限が無い）。`TestUIPills`
- **番号と線の手順は見た目 `numbered` 1 つ**：丸い番号と線を part.css・page.css に書かず、`numbered` を借りる。丸の色は tone と `data-floe-mark` で決め、`::before`・`::after` の色を上書きしない
- **項目に添える短い値の並びは見た目 `tags`**：部品の 1 つの項目に、cell の書き方の値の配列を並べるときは、間・折り返し・字の形を part.css に書かず `data-floe-look="tags"` を借り、値を 1 つずつ `<span>` に包む（pill は cell の箱のまま）。表の行、floe が組み立てる並び、配列でない値の組は外
- **行の印は floe が floe.marks に 1 つ並べる**：左に印のある行は part.yaml の `marks:` 1 つ（`box`・`items`・`joined`）で言い、HTML に class を書かない。floe が書くのは置き場所と大きさだけで、色・丸さ・中身は部品・見た目・テンプレートが書く。上の層で行の `display`・`gap`・`grid-template` を書かない（黙って勝つ）。中身の間は `> * + *` の `margin-top`。テーマは印を `content: none` で消し、大きさと間は `--floe-mark`・`--floe-mark-gap` で変える。1 つの字・題の前の飾り、`::marker`、code の行の番号と線（floe が選ぶ行は部品の根か `> li` で、`<code>` の中に届かない）、中に部品を持つ行の左の線は外（線を枠で描くと、行が箱になり、中の部品の影が外へ出る）。`TestUIMarks`・`TestUIShadows`
- **横に流れる箱は、画面で中身の幅を外へ言わない**：横に流れる箱は `overflow-x: auto` と `contain: inline-size` を `@media screen` の中の 1 つの規則に書き、紙では中身を折り返す。中で地を塗る要素は中身の端まで伸ばす。中身で幅が決まる所には置かない（画面で潰れる。例は base.css）。`TestUIWrap`・`TestUIPrint`
- **部品の間は `--floe-room`、箱の外へ出る飾りはそれ以下**：部品が並ぶ所と Markdown のブロックの間は `var(--floe-room)` で、数で書かない。部品の根は margin を書かない。部品の中の Markdown は `MarkdownFlow`、テンプレート自身のものは `kit.Markdown` が包むので、部品は流れの中の間を書かない。page.css は自分の見出しの規則を包みの外の形（`header h1`・`section > h2`）で指す。箱の外へ描く飾りは四方とも `--floe-room` 以下で、傾きや拡大を使わず、場所を空けない。`box-shadow` は base.css の 1 つの規則だけが書き（ほかで書くと影が黙って消える）、テーマはパレットの `--floe-shadow-*` と箱の `--floe-shadow-color`・`--floe-inset` で描く。`TestUIRoom`・`TestUIShadows`・`TestUITemplateMarkdown`
- **角の丸さはパレットの 3 つの変数**：角は `--radius`・`--radius-small`・`--radius-pill` を読むか、形（`0`・`50%`・`999px`）だけを書く。縁に沿う内の箱は `calc(var(--radius) - 間)`。3 つの値はどれも 1 つの長さで、4 つの角が違う形はテーマの自分の変数に置く。`TestRadius`
- **静かな地は `--quiet`**：箱の中の静かな地はパレットの `--quiet` を読み、部品が割合を混ぜない。テーマだけの変数はパレットか base.css の `:scope` に置く（default の palette.css はどのテーマの下にも入るので、default だけのものは base.css に）
- **テーマの飾りは場所を取らない**：padding を広げず、地の色や透明の枠で場所を作らず、今ある余白の上に重ねて描く（地、`--floe-inset`、`::before`）。部品が輪や印を outline で描く箱に、飾りの outline を書かない（輪に置き換わって消える）
- **テーマの飾りは傾けず、言葉を書かない**：テーマの CSS は受ける形だけを書く：変形は `translate` の類と `none` だけ、文字列は記号だけで、字は書体の名前にだけ（CSS の字は words.yaml を通らない）、数え方は decimal の類と形だけ。`TestThemeMarks`
- **字の大文字と小文字は変えない**：floe の CSS（部品・見た目・テーマ・テンプレート）は `text-transform` を `none` のほか書かない。大文字にしたい字は字そのもので書く。`TestTextCase`
