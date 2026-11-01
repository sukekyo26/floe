# ページを書く

floe がページを描く元の YAML、データファイルの書き方です。floe のテンプレートでも自分のテンプレートでも同じです。部品 `diagram` の spec は [diagrams.ja.md](diagrams.ja.md)、HTML のファイルやフォルダをそのまま見せるには [links.ja.md](links.ja.md) を見てください。自分のテンプレートの作り方は [templates.ja.md](templates.ja.md)、コマンドとフラグは全部 [commands.ja.md](commands.ja.md) にあります。

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
| `theme` | ページを描くテーマ（既定は `default`）。色・書体・部品の形を決める。`floe template show <テンプレート>` が `themes:` の行に並べる：`default`・`soft`・`editorial`・`ink`・`brutal`・`blueprint`・`notebook`・`retro`・`swiss`・`washi`・`aurora`・`app`。 |

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

`floe template list` が各テンプレートの説明を出し、`floe template show <テンプレート>` が、取る鍵（説明は `schema.json` に）と、書き始めの `example.yaml` を出します。`"format": "markdown"` の項目と、説明が Markdown と言う項目が Markdown です。鍵のほかにページが見せるもの:

- `report` は `eyebrow`（題の上の短い行）、`lead`（題の下の 1〜2 文）、`sections`（`heading` と `parts` の組の並び）を取ります。節は、部品の後に自分の `sections`（小節。`heading`（`<h3>`）と `parts` の組の並び。1 段だけ）を持てます。小節のある節には見出しが要り、見出しは空の字にできないので、見出しの無い節は鍵を書きません。
- Markdown でない鍵は、書いたとおりの字で描きます。report の `eyebrow`・`lead`・見出し、design の `status`・`owner`・`date`、handoff の `date`・`from`・`to`・`by`・`ref`・`who`、adr の `number`・`date`・`deciders`、postmortem の `date`・`authors`・`owner`・`ref`、runbook の `owner`・`updated` と手順の `who`、`more` の節の見出しです。
- `handoff` は、済んだもの・進行中・決めること・次の数をページの頭に出し、`asks` に Q1、Q2 … と番号を付けます。
- `runbook` は、手順の `who` を題の横の札に、`run` を `terminal` のコピーのボタン付きで描き、`rollback` を別の枠に入れます。
- `slides` は、1 枚ずつを画面の高さか幅いっぱいにし、番号と枚数を隅に出し、印刷では 1 枚ずつ横向きのページにします。部品は `report` と同じ大きさなので、大きくするにはブラウザで拡大します。`direction: across` では、マウスのホイールは下へしか動かず、管理ページの枠の中では枠をクリックしてから ← → のキーが効きます。

## 部品

部品は、種類を名指す鍵 1 つで書きます。`floe part list` が 1 行ずつ並べ、`floe part show <名前> schema.json` が取るもの（schema を書いたとおり）を出します。

並べるものは、行が何かで部品を選びます。

- 1 行に 1 つ：`text` の Markdown の箇条書き。
- 名前と、それより長い文（要件、リスクとその手当て、用語。表では長い文が細い列に押し込まれる）：`entries`。状態は札に書く。
- 判断：理由と退けた案を添えて 1 つを伝えるなら `decision`、案を利点と欠点で横に比べるなら `options`、短い判断を並べるだけなら `entries`（選んだものを札に）。
- 時の順に起きたこと：`timeline`。
- 言葉と意味：Markdown の箇条書き。意味が長いなら `entries`。
- 前と後の 1 つずつ：`compare`。
- ページの頭の、短い値の数個：`facts`。
- 行でも列でも見比べる、升の短い並び：`table`。

`report` が節の中に取る部品：

| 部品 | 描くもの |
|---|---|
| `text` | Markdown（表も）。中の生の HTML は除く。囲んだコードのブロック（```` ``` ````）はただのコードとして描き、警告する：コードは `code`、コマンドは `terminal` に書く。中で名指した画像のファイルは出ない（出るのは png・gif・jpeg・webp の data URI だけ）ので、画像は `image` に書く。 |
| `code` | コードをそのまま、`file` と `lang` の下に。右上のボタンでコードをコピーできる。 |
| `terminal` | コマンド（`run`）と出力（`out`）を、`shell`（`bash`・`sh`・`zsh`・`powershell`・`cmd`）のプロンプトで。プロンプトは floe が描くので、プロンプトで始まる `run`（`$ ls`）は断る。右上のボタンで、コマンドを 1 行ずつ、プロンプトと出力を除いてコピーできる。 |
| `table` | 行でも列でも見比べる、升の短い並び。`columns` と `rows`。列は題の字か `{ title, num, pills }`。`num: true` は数の列、`pills` は言葉とその色（`{ 済: good }`）で、その列の升が言葉と丸ごと同じなら、その pill で描く。セルは Markdown の 1 行、引用符で囲んだ数（そのまま描く）、または pill `{ pill, tone }`（引用符で囲まず、オブジェクトのまま書く）。 |
| `entries` | 記録を縦に並べます。1 つが `title`（Markdown の 1 行）とその横の `tags`（それぞれ表のセルと同じく Markdown の 1 行か pill）、その下の全部の幅の `text`（Markdown）。札には列の名前が出ないので、何の値かを札の中で言います（`マージ 2026-10-30`）。`text` の中のコードは、写すボタンの無いまま描くので、entries の後に `code` の部品で置きます。 |
| `image` | 画面の写しなどの画像。`file` と、何を写すかの `alt`（[画像](#画像)）。 |
| `note`・`stats`・`progress`・`steps`・`checklist` | 色の付いた注、大きな数、進みの棒、番号付きの手順、済めば印の付く項目。 |
| `compare`・`options`・`facts`・`diff`・`tree`・`links` | 前と後で変わるもの、利点と欠点を持つ案、名前と値、diff（右上のボタンで、書いたままの diff をコピーできる）、ファイルとフォルダ、リンク。 |
| `findings`・`decision`・`fields` | レビューで見つけたもの、決めたこと、設定の鍵。 |
| `timeline`・`bars`・`faq`・`keys` | 日付の順の出来事、値の棒、問いと答え、押すキー。 |
| `details`・`tabs` | 題の下に畳んだ部品、またはタブで 1 つずつ見せる部品。中に `text`・`code`・`terminal`・`diff`・`note`・`table`・`entries`・`image`・`diagram` を置ける。 |
| `diagram` | 確かめてから inline SVG で描く図（[diagrams.ja.md](diagrams.ja.md)）。 |
| `script` | 動くもの（[スクリプトと HTML](#スクリプトと-html)）。 |
| `html` | そのまま入る HTML。`any.json` の鍵で、部品そのものではないので、`floe part show` にはありません（[スクリプトと HTML](#スクリプトと-html)）。 |

`tone` は `accent`・`good`・`warn`・`bad`・`muted` のどれかです。

## 文の書き方

データファイルは、鍵と値の YAML の文書 1 つです。文書の間の `---` や、アンカー・エイリアス・マージキー（`<<`）・タグがあれば断ります。

- `: `、` #`、`{ }` や `[ ]` の中の `,` を含む文は、引用符（`"…"`）で囲むかブロック（`|-`）で書きます。YAML はそれを、鍵と値、コメントの始まり、文の終わりと読みます。
- 引用符の無い値の行のコメントは、floe が読むどの YAML でも断ります。値を途中で切ったかもしれないからです（`lead: PR #100` は `PR` と読む）。コメントは別の行に書きます。
- `|` や `>` の文で、空白だけでない最初の行が空白の後にタブで始まるものは、floe が読むどの YAML でも断ります。YAML 1.2 ではタブは文の字ですが、floe の YAML のライブラリは字下げと読み、後の行を落としたり短くしたりするからです。行をタブで始めないか、`|` や `>` の後に文の字下げの数を書きます（キーから 2 つ右の文なら `|2`）。
- `{ }` の中の値の無い鍵も断ります。`,` が文を切った残りかもしれないからです（`{ label: 1,500 words }` は `label: 1` と鍵 `500 words` と読む）。`[ ]` の中の `,` は見つけられません：`[1,500 words]` は 2 つの項目です。
- ファイルは UTF-8 で、先頭のほかに BOM を持ちません。
- プログラムからデータファイルを変えるときは、YAML のライブラリで読み、値を変えて書き出します。要る所はライブラリが引用符で囲みます。アンカーやエイリアスを書かず、ASCII でない文をそのまま、鍵を元の順に書く設定にします（PyYAML なら `allow_unicode=True`、`sort_keys=False`、`ignore_aliases` が真を返す Dumper）。

## 画像

`report` では、画像は部品 `image`（`- image: { file: shots/list.png, alt: ページの一覧 }`）で、節・小節・`details`・`tabs` に置けます。`file` は画像のパス、`alt` は何を写すかです。自分のテンプレートでは `"format": "image"` の項目で取り、その値が画像のパスです。パスは、データファイルのディレクトリからの相対パスで、その下だけです。名前の間は `/` 1 つで、`./`・`..`・両端の `/` を使いません。画像は data URI でページに入るので、ページは見せるものを全部持ちます。png・jpeg・gif・webp・svg のファイルを名前で見分け、1 MiB までです。`floe publish` は、ページが見せる画像をサーバーに送ります。

Markdown で名指した画像のファイル（`![x](shots/list.png)`）は書いたまま描かれ、ページの CSP で出ません。出るのは、Markdown に直に書いた png・gif・jpeg・webp の data URI だけです。画像は `image` に書いてください。

## 図

部品 `diagram` は、節や `details`・`tabs` の中に書いた spec から図を描き、ページを描く前に確かめます。spec の書き方は [diagrams.ja.md](diagrams.ja.md) にあります。

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
- 形の誤り（`additional properties 'x' not allowed`・`missing property 'x'`・`got string, want array`）は、テンプレートか部品の schema がデータを断ったものです。`floe template show <テンプレート> schema.json` と `floe part show <部品> schema.json` が、取るものを出します。
- 自分の部品・見た目・共有の schema・テーマ・テンプレートの誤りは、使うかどうかによらず、どのページも止めます。誤りはそのファイルの所で言います（`~/.config/floe/parts/quote/schema.json:2:1: …`）。`floe part check` と `floe template check` が並べます。
- 同じ誤りは、何か所から指されても 1 回だけ言います。

### 警告

描いたページに警告が付くことがあります。より良い書き方の勧めで、ページは描かれ、終了コードも変わりません（誤りが無ければ 0）。`floe check`・`floe render`・`floe template check`・`floe publish` が、誤りの後に出します:

```text
plan.yaml:12:15: warning: this text holds a fenced block of code on its line 1, drawn as plain code: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button
```

- `text` の部品（report の節と小節、design・handoff・adr・postmortem・runbook の `more` の中、runbook の手順の `text`、slides の枚、`details`・`tabs` の中も）の、囲んだコードのブロックを警告します。コードは `code` に、コマンドは `terminal` に書きます。
- 誤りのあるファイルは描かないので、その警告は誤りを直した後に出ます。ほかの描けたファイルの警告は、誤りと一緒に出します。
