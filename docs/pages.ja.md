# ページを書く

floe がページを描く元の YAML、データファイルの書き方です。floe のテンプレートでも自分のテンプレートでも同じです。部品 `diagram` の spec は [diagrams.ja.md](diagrams.ja.md)、HTML のファイルやフォルダをそのまま見せるには [links.ja.md](links.ja.md) を見てください。自分のテンプレートの作り方は [templates.ja.md](templates.ja.md)、コマンドとフラグは全部 [commands.ja.md](commands.ja.md) にあります。エージェントには、この短い手引き [guide.ja.md](guide.ja.md) を `floe guide` が、図の書き方 [diagrams.ja.md](diagrams.ja.md) を `floe diagram guide` が出します。

## データファイル

データファイルはテンプレートを名指し、テンプレートがそのデータからページを描きます。

```yaml
template: report
lang: ja
title: チームの wiki を移す
eyebrow: 計画 · 2030-04
lead: wiki を 2 回の週末で移す。
sections:
  - heading: 手順
    parts:
      - text: |
          1. **金曜の夜**に編集を止める。
          2. ページを写す。
      - terminal:
          shell: bash
          steps:
            - { run: wiki export --out pages/, out: 1840 pages written }
      - entries:
          - title: リンク切れ
            tags: [120 ページ, { pill: 見張り中, tone: warn }]
            text: 写した後にリンクを確かめる。
      - note: { tone: warn, text: 止めている間の編集は失われる。 }
```

```sh
floe check plan.yaml            # 誤りと警告を出すか、何も出さない
floe publish plan.yaml          # ページを保存し、URL を出す
floe publish --link plan.yaml   # 開くたびに描き、ファイルを保存すると読み直す
floe render plan.yaml           # floe 無しで開ける plan.html を隣に書く
```

フラグは [commands.ja.md](commands.ja.md) にあります。

どのデータファイルも、floe が自分で読む次の鍵を持ちます。

| 鍵 | 中身 |
|---|---|
| `template` | ページを描くテンプレート（下の floe のもの、または自分のもの）。必須。 |
| `lang` | ページの言語（`<html lang>`）。`en`・`ja`・`pt-BR` のようなタグ。必須。floe と部品が描く言葉（`fields` の「必須」、図の拡大のボタン、コードのコピーのボタン）と図の文はこれに従い、その言語の言葉が無ければ英語になる。 |
| `title` | ページの題。 |
| `theme` | ページを描くテーマ。色・書体・部品の形を決める。`floe template show <テンプレート>` が `themes:` の行に並べる：`default`・`soft`・`editorial`・`ink`・`brutal`・`blueprint`・`notebook`・`retro`・`swiss`・`washi`・`aurora`・`app`。管理ページでページの「…」の「テーマ」から選んだテーマは、これより強い。書かないと、保存したページは、初めて保存したときに管理ページの設定で選ばれていた既定のテーマで描く（無ければ `default`）。`floe check` と `floe render` は `default` で描く。 |

ほかはテンプレートのものです。floe のテンプレート:

| テンプレート | 何のため | 必須 |
|---|---|---|
| `report` | 報告。部品の節を並べ、節は 1 段だけ小節を持てる | — |
| `design` | 結論を先に出す設計書 | `background`・`goals` |
| `handoff` | 引き継ぎと状況の知らせ。決めてほしいことが先 | `date` |
| `adr` | 決めたことの記録。1 枚に 1 つ | `status`・`context`・`decision` |
| `postmortem` | 誰も責めない障害の振り返り | `summary`・`timeline`・`cause` |
| `runbook` | 走らせるコマンドを添えた、繰り返す手順 | `steps` |
| `slides` | 1 枚ずつ見せるスライド | `slides` |

`floe template list` が各テンプレートの説明と、データファイルの鍵（形。`-` なら `floe template show <テンプレート> schema.json` を読みます）を出し、`floe template show <テンプレート> schema.json` が取る鍵とその説明を、`floe template show <テンプレート> example.yaml` が書き始めの見本を出します。`"format": "markdown"` の項目と、説明が Markdown と言う項目が Markdown です。鍵のほかにページが見せるもの:

- `report` は `eyebrow`（題の上の短い行）、`lead`（題の下の 1〜2 文）、`sections`（`heading` と `parts` の組の並び）を取ります。`eyebrow` と `lead` はどちらも Markdown の 1 行で、`*` や `_` などの記号は読むので、字として書くならバッククォートで囲みます（取り消し線は[文の書き方](#文の書き方)）。裸の URL とメールのアドレスはリンクになります（IP や localhost の URL（`http://127.0.0.1:7325`）はなりません）。中で名指した画像のファイルは出ません（出るのは png・gif・jpeg・webp の data URI だけ）。節は、部品の後に自分の `sections`（小節。`heading`（`<h3>`）と `parts` の組の並び。1 段だけ）を持てます。小節のある節には見出しが要り、見出しは空の字にできないので、見出しの無い節は鍵を書きません。見出しは `lead` と同じく Markdown の 1 行です。id は描いた字から作ります。読んだ記号（`__init__`）、バックスラッシュ、文字参照（`&copy;`）、リンクの URL の分だけ、書いた字から作る id と変わります。`toc` は、ページに目次（見出しのある節とその小節）を置くかを決めます。幅 1000px 以上の画面では節の横に置き（ページは窓の幅いっぱいになる）、狭い画面では節の前に節だけを置きます。省くと、見出しのある節が 4 つ以上のページに置きます。紙には出ません。目次のリンク先は節の順の `#section.0`・`#section.0.1` などです。ほかから節へリンクするときは、今までどおりの見出しの id（`#1-what-happens`）を使います。CSS でこの id を指すときは、`#section.0` だと `.0` を class と読むので、`[id="section.0"]` と書きます。
- Markdown でない鍵は、書いたとおりの字で描きます。design の `status`・`owner`・`date`、handoff の `date`・`from`・`to`・`ref`・`who`、部品 `asks` の `by`、adr の `number`・`date`・`deciders`、postmortem の `date`・`authors`・`owner`・`ref`、runbook の `owner`・`updated` と手順の `who`、`more` の節の見出しです。
- `handoff` は、済んだもの・進行中・決めること（`decided` の無い問い）・次の数をページの頭に出し、`asks` を部品 `asks` で描きます。
- `runbook` は、手順の `who` を題の横の札に、`run` を `terminal` のコピーのボタン付きで描き、`rollback` を別の枠に入れます。
- `slides` は、1 枚ずつを画面の高さか幅いっぱいにし、番号と枚数を隅に出し、印刷では 1 枚ずつ横向きのページにします。部品は `report` と同じ大きさなので、大きくするにはブラウザで拡大します。`direction: across` では、マウスのホイールは下へしか動かず、管理ページの枠の中では枠をクリックしてから ← → のキーが効きます。

## 部品

部品は、種類を名指す鍵 1 つで書きます。`floe part list` が 1 行ずつ、取る鍵（形）と一緒に並べ、`floe part show <名前> schema.json` が残り（schema を書いたとおり）を出します。鍵ごとに取るものと、`-` の形の中身です。`html` は HTML を字で取り、schema.json はありません。`floe part show html any.json` で見ます。

並べるものは、行が何かで部品を選びます。

- 1 行に 1 つ：`text` の Markdown の箇条書き。
- 名前と、それより長い文（要件、リスクとその手当て、用語。表では長い文が細い列に押し込まれる）：`entries`。状態は札に書く。
- 判断：理由と退けた案を添えて 1 つを伝えるなら `decision`、案を利点と欠点で横に比べるなら `options`、読み手に判断を頼み、決まったら決定を書くなら `asks`、短い判断を並べるだけなら `entries`（選んだものを札に）。
- 確かめたことと実際の結果（調査、テスト）：`checks`。1 件ずつ OK・NG・未確認を付ける。
- 数の移り変わりや比べ：`chart`。線か縦の棒で描く。
- 時の順に起きたこと：`timeline`。
- 言葉と意味：Markdown の箇条書き。意味が長いなら `entries`。
- ページの頭の、短い値の数個：`facts`。
- 行でも列でも見比べる、升の短い並び（前と後は 1 列ずつ）：`table`。

`report` が節の中に取る部品。自分の部品も取り（[parts.ja.md](parts.ja.md)）、`floe part list` がこれらと一緒に並べます：

| 部品 | 描くもの |
|---|---|
| `text` | Markdown（表も）。中の HTML は書いたとおりの字で描く。取り消し線は[文の書き方](#文の書き方)のとおり。囲んだコードのブロック（```` ``` ````）はただのコードとして描き、警告する：コードは `code`、コマンドは `terminal` に書く。中で名指した画像のファイルは出ない（出るのは png・gif・jpeg・webp の data URI だけ）ので、画像は `image` に書く。 |
| `code` | コードをそのまま、`title`（何のコードかを言葉で）と `file`（どのファイルか）と `lang` の下に。`start` はその番号から行に番号を出す。`mark` はその番号の行に印を付ける（`start` が無ければ 1 から数える）。どの行にも当たらない番号は何もしない。右上のボタンでコードをコピーできる（番号は入らない）。図や表を文字で描かず、`diagram`・`table`・`tree` で書く。 |
| `terminal` | コマンド（`run`）と出力（`out`）を、`shell`（`bash`・`sh`・`zsh`・`powershell`・`cmd`）のプロンプトで。プロンプトは floe が描くので、プロンプトで始まる `run`（`$ ls`）は断る。右上のボタンで、コマンドを 1 行ずつ、プロンプトと出力を除いてコピーできる。 |
| `table` | 行でも列でも見比べる、升の短い並び。`columns` と `rows`。列は題の字か `{ id, title, num, pills }`。`num: true` は数の列、`pills` は言葉とその色（`{ 済: good }`）で、その列の升が言葉と丸ごと同じなら、その pill で描く。行は、列の順に並べたセルか、列の `id` を鍵にしたセル（`{ state: 済, who: walter }`）。書かなかった列は空で、どの列の `id` でもない鍵は断る。1 つの表で混ぜてよい。セルは Markdown の 1 行、引用符で囲んだ数（そのまま描く）、または pill `{ pill, tone }`（引用符で囲まず、オブジェクトのまま書く）。 |
| `entries` | 記録を縦に並べます。1 つが `title`（Markdown の 1 行）とその横の `tags`（それぞれ表のセルと同じく Markdown の 1 行か pill）、その下の全部の幅の `text`（Markdown）、その下の `parts`（`details` と同じ部品の並び）。札には列の名前が出ないので、何の値かを札の中で言います（`マージ 2026-10-30`）。コードは `parts` に `code`・`terminal` で書きます。`text` の中の囲んだコードは、ただのコードとして描き、警告します。 |
| `image` | 画面の写しなどの画像。`file` と、何を写すかの `alt`、任意で絵の下に出す説明 `caption`（[画像](#画像)）。 |
| `stats` | 大きな数を横に並べます。`label`・`value`・`unit` と、`tone` の色の `note` を取ります。前の数 `from` を書くと `3.1 → 1.3` と描き、下に `-1.8` のような差と、良くなったか悪くなったかをその色で（同じなら数なしで）描きます。`from` には、どちらが良いかを言う `better: lower` か `higher` を必ず添え、`from` と `value` は引用符で囲んだ数字だけで書きます（`'-3.5'`。`1,234` は、誤りの文の pattern のとおり断ります）。差の桁は 2 つの数の小数の桁の多い方で、全部の桁を描きます。割合は描かないので、要るなら `note` に書きます。 |
| `note`・`checklist` | 色の付いた注、済めば印の付く項目。 |
| `steps` | 番号付きの手順。1 つが `title` と、その下の `tags`（それぞれ表のセルと同じく Markdown の 1 行か pill）、`text`（Markdown）、`state` です。札の中で何の値かを言い（`PR #428`）、手順を指すときは並べ替えても変わらないもので書きます（`1 の後` でなく `#428 の後`）。丸の番号は付け直されますが、札の字は書いたままです。 |
| `options`・`facts`・`diff`・`tree`・`links` | 利点と欠点を持つ案（案ごとに題の下へ、`details` が持てる部品の `parts`（案の絵・図・コードなど）を置ける。同じ段に並んだ案は、部品と文の頭が揃う）、名前と値、diff（右上のボタンで、書いたままの diff をコピーできる）、ファイルとフォルダ、リンク。 |
| `asks` | 読み手に判断を頼む問い。Q1、Q2 … と番号が付きます。`question`、案の `options`、勧めの `recommend` と理由の `why`、決める人の `by`、決まったら `decided`。`recommend`・`decided` と同じ字の案に枠と札が付き、どの案とも違えば別の行に出ます。決まった問いは右上に丸い ✓ が付き、枠は決定の案だけで、ほかの案は薄い字になります（勧めの案の札は残ります）。 |
| `checks` | 調査やテストで確かめたことと実際の結果。書いた順に並び、上に結果ごとの数が出ます。1 件は、題の `title`、結果の `result`（`ok`・`ng`・`unchecked`。日本語のページでは OK・NG・未確認と描く）、実際の結果の `actual`（Markdown）、その下の `parts`（`details` が持てるもの。`code` など）、確かめ方の `how`。`unchecked` の `actual` には、確かめていない理由を書きます。これからやることは `checklist` に書きます。 |
| `chart` | 名前の並びの上の数を、移り変わりは線（`kind: line`）で、比べは縦の棒（`kind: bar`）で描き、数は畳んだ表にも置きます。`title`・`unit`、引用符で書く名前の並び `x`、4 つまでの `series`（`name` と、`x` の 1 つごとに数を 1 つ書く `values`）を取ります。目盛りは 0 から始まります。線は `min` でそこから始められ、`min` より小さい値があると `min` は使わず、0 とすべての値を含む目盛りになります。棒は `min` を取らず、書くと `additional properties 'min' not allowed` で断ります。字はどの幅でもページの大きさのままで、`34rem` より狭い箱では線の右の字は値だけになります。`8rem` を超える `x` の名前は … で切り、9 桁を超える数は丸めるか指数で書きます。表と重ねたときの説明には全部あります。目盛りの字はその線の値で、5 桁で見分けられない、または言えない目盛りは字を描きません。凡例は系列が 1 つでも名前を出します。 |
| `decision`・`fields` | 決めたこと、設定の鍵。 |
| `timeline`・`bars`・`keys` | 日付の順の出来事、値の棒、押すキー。`of` を書いた棒は、その全体のうち `value` までを塗り、`5 / 8` と出します（作業の進みに）。ほかの棒は、その中で一番大きい値に対する割合です。 |
| `details`・`tabs` | 題の下に畳んだ部品、または 1 つずつ読めばよいものをタブで見せる部品。中に `tabs`・`columns`・`html` のほかの全部の部品を置ける（tabs の form の中の form は落ち、columns は列の中に列ができ、`html` の `</form>` は tabs の form を閉じるため）。カードで並べる部品・keys・timeline・bars は、窓の幅でなく置いた場所の幅で並べ、`asks` の札はそこで折り返す。閉じた `details` や見えていないタブの中の `script` は、幅の無い要素に対して走る。 |
| `columns` | 部品を横に並べ、同時に見せます。前と後の画面、データと説明など。列は、省ける `title` とその下の `parts`（`details` と同じ部品）。列はどれも同じ幅で、幅は 16rem 以上なので、何列が横に並ぶかは本文の幅で決まり（`report` で最大 5 列、`adr`・`runbook` で 3 列）、残りは下に折り返します。 |
| `diagram` | 確かめてから inline SVG で描く図（[diagrams.ja.md](diagrams.ja.md)）。 |
| `script` | 動くもの（[スクリプトと HTML](#スクリプトと-html)）。 |
| `html` | そのまま入る HTML。`any.json` の鍵で、部品そのものではないので schema.json はありません。`floe part show html any.json` が、その鍵を持つ `any.json` の全体を出します（[スクリプトと HTML](#スクリプトと-html)）。 |

`tone` は `accent`・`good`・`warn`・`bad`・`muted` のどれかです。

## 文の書き方

データファイルは、鍵と値の YAML の文書 1 つです。文書の間の `---` や、アンカー・エイリアス・マージキー（`<<`）・タグがあれば断ります。

- Markdown では、`~~字~~` は取り消し線になり、`~` 1 つは字のままです（`2026-11-01~11-08`、`~/work`）。部品 text、表のセル、見出し、lead など、floe が Markdown として描くどの鍵でも同じです。
- 日本語・中国語・韓国語の字の隣では、`**`・`*`・`~~` を約物を挟んでも読みます（[markdown-cjk-friendly](https://github.com/tats-u/markdown-cjk-friendly) の読み方）。`**「題」**に` は太字になります。全角の英数字・記号（`１`、`（`）も、これらの字に入ります。`_`・`__` も同じく読みますが、ふつうの文では CommonMark と同じです。記号のつもりの `*` をこれらの字や約物の隣に書くとき（`*.yaml`、`L*、`）は、`\*` かバッククォートで書きます。書かないと、ほかの `*` と組になって斜体になることがあります。
- `: `、` #`、`{ }` や `[ ]` の中の `,` を含む文は、引用符（`"…"`）で囲むかブロック（`|-`）で書きます。YAML はそれを、鍵と値、コメントの始まり、文の終わりと読みます。引用符の無いまま書くと、YAML の言葉で断られます。文は `: ` の後ろの字で変わり（`mapping value is not allowed in this context`、``'`' is a reserved character``）、場所は値の始まりを指すので、その値の中の `: ` を探します。
- 引用符の無い字の値や `|`・`>` の文のすぐ後のコメントは、値の鍵（列の項目なら `-`）より深い所にあれば、同じ行でも次の行でも、floe が読むどの YAML でも断ります。値を途中で切ったかもしれないからです（`lead: PR #100` は `PR` と読み、`|` の文の、文の字下げより浅い `# 見出し` の行はコメントとして落ちる）。`#` が値の一部なら、値を引用符で囲みます。`|`・`>` の文なら、`#` の行も含めて文の行を同じ深さに字下げするか、`|` や `>` の後に字下げの数を書きます。コメントなら、`{ }` や `[ ]` の外の別の行に、値の鍵（列の項目なら `-`）と同じか、それより浅い所に書きます。
- `{ }` や `[ ]` の中には、別の行のものも含めて、コメントを書けません（floe が読むどの YAML でも）。コメントは、その外の行に書きます。
- `|` や `>` の文で、空白だけでない最初の行が空白の後にタブで始まるものは、floe が読むどの YAML でも断ります。YAML 1.2 ではタブは文の字ですが、floe の YAML のライブラリは字下げと読み、後の行を落としたり短くしたりするからです。行をタブで始めないか、`|` や `>` の後に文の字下げの数を書きます（キーから 2 つ右の文なら `|2`）。
- `|` や `>` の文で字下げの数を書かないと、空白だけでない最初の行の字下げが、文の字下げになります。最初の行を後の行より深くすると（表の頭の行をそろえる、diff の変わらない行の頭の空白など）、文はそれより浅い行の前で終わり、浅い行は文の外の YAML と読まれて断られます。文は浅い行の形で変わり（`value is not allowed in this context. map key-value is pre-defined`、`tab character cannot use as a map key directly`）、場所は浅い行か、その後の行を指します。誤りの場所が `|` や `>` の文の後にあるときは、その文の最初の行の深さを確かめます。最初の行を後の行と同じ深さにするか、`|` や `>` の後に文の字下げの数を書きます（キーから 2 つ右の文なら `|2`）。
- `{ }` の中の値の無い鍵も断ります。`,` が文を切った残りかもしれないからです（`{ label: 1,500 words }` は `label: 1` と鍵 `500 words` と読む）。`[ ]` の中の `,` は見つけられません：`[1,500 words]` は 2 つの項目です。
- ファイルは UTF-8 で、先頭のほかに BOM を持ちません。
- プログラムからデータファイルを変えるときは、YAML のライブラリで読み、値を変えて書き出します。要る所はライブラリが引用符で囲みます。アンカーやエイリアスを書かず、ASCII でない文をそのまま、鍵を元の順に書く設定にします（PyYAML なら `allow_unicode=True`、`sort_keys=False`、`ignore_aliases` が真を返す Dumper）。

## 画像

`report` では、画像は部品 `image`（`- image: { file: shots/list.png, alt: ページの一覧 }`）で、節・小節・`details`・`tabs`・`columns`・`entries`・`checks`・`options` の項目の `parts` に置けます。`file` は画像のパス、`alt` は何を写すか（読み上げと、絵が出ない所のため）です。`caption` を書くと、絵の下に説明を出します（1 行の Markdown。`caption: 図 1：ページの一覧`）。列より大きく縮んで描かれた画像は、図と同じく、上の道具の並び、Ctrl / ⌘ + ホイール、ピンチ、`+ - 0` のキーで拡大し、拡大中はドラッグか矢印キーで動かせます。道具の並びはボタンだけで、この操作は、枠に重ねたときの説明と、キーボードで枠を選んでいる間に枠の下に出る文で伝えます。文は枠の説明（`aria-describedby`）でもあります。`viewBox` だけで `width`・`height` の無い SVG は大きさが分からないので、拡大できません。自分のテンプレートでは `"format": "image"` の項目で取り、その値が画像のパスです。パスは、データファイルのディレクトリからの相対パスで、その下だけです。名前の間は `/` 1 つで、`./`・`..`・両端の `/` を使いません。画像は data URI でページに入るので、ページは見せるものを全部持ちます。png・jpeg・gif・webp・svg のファイルを名前で見分け、1 MiB までです。`floe publish` は、ページが見せる画像をサーバーに送ります。

Markdown で名指した画像のファイル（`![x](shots/list.png)`）は書いたまま描かれ、ページの CSP で出ません。出るのは、Markdown に直に書いた png・gif・jpeg・webp の data URI だけです。画像は `image` に書いてください。

## 図

部品 `diagram` は、節や `details`・`tabs`・`columns`・`entries`・`checks`・`options` の項目の `parts` の中に書いた spec から図を描き、ページを描く前に確かめます。spec の書き方は [diagrams.ja.md](diagrams.ja.md) にあります。

## スクリプトと HTML

- `script` は `js`（ページが、その場所の空の要素と `data` を渡して呼ぶ関数 `function (el, data) { … }`）と `data`（どんな値でも）です。`data` の中のマッピングは鍵の名前の順で渡ります（順が大事ならリストで書く）。関数はその場所で、ページの残りを読む前に動きます。`</script` や `<!--` を含む `js` は断ります。関数の中の誤りはブラウザに出ます。
- `html` の部品は、そのまま入ります。要素を開いたままにすることも、周りのページの要素を閉じること（開いたままの `<b>`、`</section>`）もできず、部品の所で断ります。要素は丸ごと書きます。描いたページは、誰が書いても `<style>`・`<base>`・`<noscript>`、charset と viewport のほかの `<meta>` を持てず、データファイルの `template:` の所で断ります。HTML の見た目は `style` 属性で付けます。
- データファイルから描いたページのスクリプトは、ページの中身を外へ送れません。`fetch`・XHR・ほかの所の画像・フォームは CSP が止めます。cdnjs・jsDelivr・unpkg のライブラリと Google Fonts は読めます。

## 誤りの読み方

`floe check` は誤りを 1 行ずつ出し、誤りがあれば終了コードは 1 です。データファイルは段ごとに確かめます（[templates.ja.md](templates.ja.md#確かめる順)）。1 つの段の誤りは全部まとめて言い、前の段に誤りがあれば、後の段は走りません。だから、まず形を確かめ：

```text
plan.yaml:5:5: additional properties 'colour' not allowed
plan.yaml:8:23: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

それを直すと、その後の確かめに進みます。

```text
plan.yaml:8:36: write the command without its prompt, which floe draws from the shell ("$ ls")
```

- `file:line:column:` が誤りの場所です。変える値か、それを持つ鍵を指します。行の無い `file:` だけの誤りは、ファイル全体の誤りです。根に足りない鍵（`design.yaml: missing properties 'background', 'goals'`）や、テンプレートのデータファイル全体にかける検査がこう出ます。言われた誤りを全部直してから、`floe check` をもう一度走らせます。次の段の誤りが出ることがあります。
- YAML そのものの誤り（`mapping value is not allowed in this context`）は、floe の YAML ライブラリの言葉で、ライブラリが名指す場所に出ます。引用符の無い値の中の `: ` なら、場所は値の始まりです。場所が `|` や `>` の文の後の行なら、その文の最初の行が深すぎるかもしれません。文の最初の行の深さを確かめます（[文の書き方](#文の書き方)）。
- 形の誤り（`additional properties 'x' not allowed`・`missing property 'x'`・`got string, want array`）は、テンプレートか部品の schema がデータを断ったものです。`floe template show <テンプレート> schema.json` と `floe part show <部品> schema.json` が、取るものを出します。
- 自分の部品・見た目・共有の schema・テーマ・テンプレートの誤りは、使うかどうかによらず、どのページも止めます。誤りはそのファイルの所で言います（`~/.config/floe/parts/quote/schema.json:2:1: …`）。`floe part check` と `floe template check` が並べます。
- 同じ誤りは、何か所から指されても 1 回だけ言います。

ページの中で一度に開いておける要素は 512 までで、`<html>`・`<body>` と、テンプレートや部品が開く要素も数えます。超えると `html: open stack of elements exceeds 512 nodes` で断ります。Markdown・`html` の部品・部品・`page.html` のどれからも深くなるので、場所はデータファイルだけです。図の `raw` がそれだけで超えるときは、その行と列です。入れ子を浅くしてください。

### 警告

描いたページに警告が付くことがあります。より良い書き方の勧めで、ページは描かれ、終了コードも変わりません（誤りが無ければ 0）。`floe check`・`floe render`・`floe template check`・`floe publish` が、誤りの後に出します:

```text
plan.yaml:12:15: warning: this text holds a fenced block of code on its line 1, drawn as plain code: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button
```

- `text` の部品（report の節と小節、design・handoff・adr・postmortem・runbook の `more` の中、runbook の手順の `text`、slides の枚、`details`・`tabs`・`columns`・`entries`・`checks`・`options` の項目の `parts` の中も）、`entries`・`options` の項目の `text`、`checks` の項目の `actual` の、囲んだコードのブロックを警告します。コードは `code` に、コマンドは `terminal` に書きます。
- 誤りのあるファイルは描かないので、その警告は誤りを直した後に出ます。ほかの描けたファイルの警告は、誤りと一緒に出します。
