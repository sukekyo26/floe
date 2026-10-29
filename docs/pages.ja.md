# ページを書く

floe がページを描く元の YAML、データファイルの書き方です。floe のテンプレート `report`・`design`・`handoff`・`adr`・`postmortem`・`runbook` でページを書くには、これだけで足ります。自分のテンプレートの作り方は [templates.ja.md](templates.ja.md)、コマンドとフラグは [commands.ja.md](commands.ja.md) にあります。

## データファイル

データファイルはテンプレートを名指し、テンプレートがそのデータからページを描きます。floe のテンプレートは、見出しの下に節を並べ、節ごとに見出しと部品を置く `report`、設計書の `design`、引き継ぎと状況の知らせの `handoff`、決めたことの記録の `adr`、障害の振り返りの `postmortem`、手順書の `runbook` です。

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
      - table:
          columns:
            - title: リスク
            - { title: ページ, num: true }
            - title: 状態
          rows:
            - [リンク切れ, '120', { pill: 見張り中, tone: warn }]
      - note: { tone: warn, text: 止めている間の編集は失われる。 }
```

```sh
floe check plan.yaml     # 誤りと警告を出すか、何も出さない
floe publish plan.yaml   # ページを保存し、URL を出す
```

どのデータファイルも、floe が自分で読む次の鍵を持ちます。

| 鍵 | 中身 |
|---|---|
| `template` | ページを描くテンプレート（`report`・`design`・`handoff`・`adr`・`postmortem`・`runbook`、または自分のもの）。必須。 |
| `lang` | ページの言語（`<html lang>`）。`en`・`ja`・`pt-BR` のようなタグ。必須。floe と部品が描く言葉（`risks` の見出し、図の拡大のボタン、コードのコピーのボタン）と図の文はこれに従い、その言語の言葉が無ければ英語になる。 |
| `title` | ページの題。 |
| `theme` | ページを描くテーマ（既定は `default`）。色・書体・部品の形を決める。`floe template show <テンプレート>` が `themes:` の行に並べる：`default`・`soft`・`editorial`・`ink`・`brutal`・`blueprint`・`notebook`・`swiss`・`washi`。 |

ほかはテンプレートのものです。`report` は `eyebrow`（題の上の短い行）、`lead`（題の下の 1〜2 文）、`sections`（`heading` と `parts` の組の並び）を取ります。`floe template show <テンプレート>` が、テンプレートが取るものと、書き始めの見本を出します。

`design` は、結論を先に出す設計書です。`background`（Markdown）と `goals` が必須で、`status`・`owner`・`date`・`summary` が頭に、`non_goals`・`decisions`・`decided`・`impact`・`more` を続けられます。判断（`decisions`）は 1 つずつ `question` と、その `options`（部品 `options`）と `why` で、`pick` を付けた案が、問いと一緒にページの一番上に出ます。`pick` の無い判断は未決と出ます。`decided` は案を比べずに決めたこと、`impact` は `what` と `text` の行、`more` は report と同じ `heading` と `parts` の自分の節です。`background` は Markdown、`summary`・`goals`・`non_goals`・`decided` の各項目・`question`・`why`・影響の範囲の `what` と `text` は Markdown の 1 行、`status`・`owner`・`date`・`more` の見出し・案の `title` と `pick` は、書いたままの字です。

`handoff` は、決めてほしいことを先に出す、作業の引き継ぎと状況の知らせです。必須は `date` だけで、`from`・`to`・`summary` が頭に、その下に終えた・進行中・決めてほしい・次にやるの数が出ます。`asks` は決めてほしいことで、1 つずつ `question` と `options`・`recommend`・`why`・決める人（`by`）を書き、Q1・Q2 と番号が付きます。`done` は `what` と `ref`（PR やチケット）の行、`doing` は `who`・`what`・`where`（どこまで）・`state`（`doing`・`waiting`・`blocked`）の行、`next` と `notes`（気を付けること）は行、`more` は report と同じ自分の節です。`date`・`from`・`to`・`by`・`ref`・`who`・`more` の見出しは書いたままの字で、ほかの行は Markdown の 1 行です。

`adr` は、1 つの決めたことを紙 1 枚に書く記録です。`status`（`proposed`・`accepted`・`rejected`・`deprecated`・`superseded`。題の下の判子）・`context`・`decision`（Markdown）が必須で、`status` が `superseded` の記録は、置き換えた記録を `superseded_by`（`[ADR 0012](/a/adr-0012/)`）に書きます。`number`・`date`・`deciders` が頭に出て、`alternatives` は選ばなかった案（`title` と `why`）、`consequences` は良くなること（`good`）と引き受けること（`bad`）の行、`more` は report と同じ自分の節です。`number`・`date`・`deciders`・`more` の見出しは書いたままの字で、`superseded_by` とほかの行は Markdown の 1 行です。

`postmortem` は、人を責めずに書く障害の振り返りです。`summary`（Markdown の 1 行。アクセントの色で塗った帯の中）・`timeline`・`cause`（Markdown）が必須です。`date`・`severity`（`high`・`medium`・`low`。帯の中の札で、赤は `high` だけ）・`authors` が頭に出て、`effects` は影響（部品 `facts` と同じ `name` と `value`）、`timeline` は部品 `timeline` の値（書いた順に描き、`date` は `10:42` のような書いたままの字）、`lessons` はうまくいったこと（`good`）・まずかったこと（`bad`）・運が良かったこと（`lucky`）の行、`actions` は `what`（Markdown の 1 行）・`owner`・`state`（`todo`・`doing`・`done`。札）・`ref`（チケットなど）の行、`more` は report と同じ自分の節です。`date`・`authors`・`owner`・`ref`・`more` の見出しは書いたままの字です。

`runbook` は、何度も行う作業を、打つコマンドと一緒に残す手順書です。必須は `steps` と、各手順の `title`（`rollback` の手順も）だけで、1 つずつ `title`（Markdown の 1 行）・`who`（読む人のほかが行うとき。題の横の札）・`text`（部品 `text`。囲んだコードは警告するので、コマンドは `run` に書く）・`run`（部品 `terminal` の値。写すボタン付き）・`if_fails`（Markdown の 1 行。書いたとおりにいかなかったときにすること）を書きます。`rollback` は同じ形の手順で、戻し方を別の枠に描きます。`summary`（Markdown の 1 行）・`owner`・`updated`（最後に直した日。書いたままの字）が頭に出て、`needs` は始める前に確かめること（Markdown の 1 行ずつ）、`caution` は作業の間にしてはいけないこと（Markdown の 1 行。手順の前）、`more` は report と同じ自分の節です。

## 部品

部品は、種類を名指す鍵 1 つで書きます。`floe part list` が 1 行ずつ並べ、`floe part show <名前>` が取るもの（schema を書いたとおり）を出します。`report` が節の中に取る部品：

| 部品 | 描くもの |
|---|---|
| `text` | Markdown（表も）。中の生の HTML は除く。囲んだコードのブロック（```` ``` ````）はただのコードとして描き、警告する：コードは `code`、コマンドは `terminal` に書く。中で名指した画像のファイルは出ない（出るのは png・gif・jpeg・webp の data URI だけ）ので、画像は `image` に書く。 |
| `code` | コードをそのまま、`file` と `lang` の下に。右上のボタンでコードをコピーできる。 |
| `terminal` | コマンド（`run`）と出力（`out`）を、`shell`（`bash`・`sh`・`zsh`・`powershell`・`cmd`）のプロンプトで。プロンプトは floe が描くので、プロンプトで始まる `run`（`$ ls`）は断る。右上のボタンで、コマンドを 1 行ずつ、プロンプトと出力を除いてコピーできる。 |
| `table` | `columns` と `rows`。セルは Markdown の 1 行、引用符で囲んだ数（そのまま描く）、または pill `{ pill, tone }`。 |
| `image` | 画面の写しなどの画像。`file` と、何を写すかの `alt`（[画像](#画像)）。 |
| `cards`・`note`・`stats`・`progress`・`steps`・`checklist` | カード、色の付いた注、大きな数、進みの棒、番号付きの手順、済めば印の付く項目。 |
| `compare`・`options`・`facts`・`diff`・`tree`・`links` | 前と後で変わるもの、利点と欠点を持つ案、名前と値、diff（右上のボタンで、書いたままの diff をコピーできる）、ファイルとフォルダ、リンク。 |
| `findings`・`decision`・`risks`・`matrix`・`fields` | レビューで見つけたもの、決めたこと、リスク、○・△・× の表、設定の鍵。 |
| `timeline`・`bars`・`faq`・`glossary`・`keys` | 日付の順の出来事、値の棒、問いと答え、言葉と意味、押すキー。 |
| `details`・`tabs` | 題の下に畳んだ部品、またはタブで 1 つずつ見せる部品。中に `text`・`code`・`terminal`・`diff`・`note`・`table`・`image` を置ける。 |
| `diagram` | 確かめてから inline SVG で描く図（[図](#図)）。 |
| `script` | 動くもの（[スクリプトと HTML](#スクリプトと-html)）。 |
| `html` | そのまま入る HTML（[スクリプトと HTML](#スクリプトと-html)）。 |

`tone` は `accent`・`good`・`warn`・`bad` のどれかです。

## 文の書き方

データファイルは、鍵と値の YAML の文書 1 つです。文書の間の `---` や、アンカー・エイリアス・マージキー（`<<`）・タグがあれば断ります。

- `: `、` #`、`{ }` や `[ ]` の中の `,` を含む文は、引用符（`"…"`）で囲むかブロック（`|-`）で書きます。YAML はそれを、鍵と値、コメントの始まり、文の終わりと読みます。
- 引用符の無い値の行のコメントは、floe が読むどの YAML でも断ります。値を途中で切ったかもしれないからです（`lead: PR #100` は `PR` と読む）。コメントは別の行に書きます。
- `{ }` の中の値の無い鍵も断ります。`,` が文を切った残りかもしれないからです（`{ label: 1,500 words }` は `label: 1` と鍵 `500 words` と読む）。`[ ]` の中の `,` は見つけられません：`[1,500 words]` は 2 つの項目です。
- ファイルは UTF-8 で、先頭のほかに BOM を持ちません。
- プログラムからデータファイルを変えるときは、YAML のライブラリで読み、値を変えて書き出します。要る所はライブラリが引用符で囲みます。アンカーやエイリアスを書かず、ASCII でない文をそのまま、鍵を元の順に書く設定にします（PyYAML なら `allow_unicode=True`、`sort_keys=False`、`ignore_aliases` が真を返す Dumper）。

## 画像

`report` では、画像は部品 `image`（`- image: { file: shots/list.png, alt: ページの一覧 }`）で、section・`details`・`tabs` に置けます。`file` は画像のパス、`alt` は何を写すかです。自分のテンプレートでは `"format": "image"` の項目で取り、その値が画像のパスです。パスは、データファイルのディレクトリからの相対パス、その下だけ、`..` やドットで始まる名前を含まないものです。画像は data URI でページに入るので、ページは見せるものを全部持ちます。png・jpeg・gif・webp・svg のファイルを名前で見分け、1 MiB までです。`floe publish` は、ページが見せる画像をサーバーに送ります。

Markdown で名指した画像のファイル（`![x](shots/list.png)`）は書いたまま描かれ、ページの CSP で出ません。出るのは、Markdown に直に書いた png・gif・jpeg・webp の data URI だけです。画像は `image` に書いてください。

## 図

`diagram` の部品は、その場に書く spec です。キャンバスと、その上に絶対座標（左上が 0, 0）で置く部品から成ります。ページを描く前に確かめ（誤りはデータファイルの行と列で言う）、拡大できる inline SVG で描きます。`floe diagram render` は、ファイルに書いた spec を同じように描きます（[commands.ja.md](commands.ja.md#floe-diagram-render-specyaml)）。

```yaml
- diagram:
    width: 640
    height: 200
    # スクリーンリーダーが読み上げる
    title: API の配置
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, w: 200, h: 56, tone: accent, rows: [[ALB, main code], 公開の入口] }
      - box: { name: api, x: 400, y: 72, w: 200, h: 56, rows: [API サーバー, ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

- ほかの鍵の代わりに `example: <種類>` を書くと、組み込みの見本を描きます（`floe diagram example` が並べる）。ページは図だけを見せ、spec は見せません。見本の spec を読む・写すときは、`floe diagram example <種類>` で出します。
- 文（`title`・行・ラベル）は、文か、言語ごとの言葉（英語を含む）です：`{ en: Store, ja: 保存先 }`。ページは `lang:` の言葉を描き、spec は書いた言語ごとに並べて確かめます。
- 部品は、種類を名指す鍵 1 つで書きます。`floe part show diagram` がすべての鍵を出し、`floe diagram example <種類>` が種類ごとの spec を、格子を説明するコメント付きで出します。

| 部品 | 鍵 |
|---|---|
| `frame` | `x` `y` `w` `h`、`label`、`name`、`accent`、`square`（角を丸めない）、`icon`（`aws/group/region` のようなアイコン。AWS のグループの形で、角にアイコン）。ラベルは左上の内側に置くので、中身は上から 32 以上（アイコンがあれば 40）下に置く。 |
| `box` | `x` `y`、`rows`、`w` `h`（省けば行に要る大きさ）、`name`、`origin`（`left`。`x` を真ん中にするなら `centre`）、`tone`、`shape`（`rect`。`pill`、判断の `hex`、`state`）。行は文か `[文, 形]` で、形は `main`・`sub`・`main code`・`sub code`。形が無ければ最初の行が `main`、残りが `sub`。 |
| `table` | `x` `y`、`title`、`rows`、`w`、`name`、`origin`、`tone`。行は `[名前]`、`[名前, 型]`、区切りの `---`、または線が行に着く `{ name, cells: [名前, 型] }`。 |
| `icon` | `x` `y`（左上）、`icon`、`label`（アイコンの下）、`size`（48）、`name`。`icon` はセットとその中の名前で `aws/amazon-ec2` のように書く（`floe icons search` で探す）。 |
| `edge` | `from` と `to`（部品・名前付きの行・`tree` の節の `name`、または点 `[x, y]`）、`fromSide` と `toSide`（`top`・`right`・`bottom`・`left`）、`via`（曲がる点）、`straight`（直角でなく点から点へ）、`label` と `at: [x, y]`、`dashed`、`bold`、`tone`、`start`（`none`）、`end`（`arrow`）、`crossing`。端は `arrow`・`triangle`・`diamond`・`odiamond`・`one`・`many`・`zero-one`・`zero-many`・`none`。`crossing: true` でこの線はほかと交われる。 |
| `dot` | `cx` `cy`、`r`（7）、`name`、`tone`、`hollow`。線は中心まで届いてよい。 |
| `bar` | `x1` `x2` `y`、`name`、`tone`、`label`（棒の右）。 |
| `axis` | `x1` `x2` `y`、`ticks: [[x, 文], ...]`、`grid`（格子の線の上端）。 |
| `circle` | `cx` `cy` `r`、`name`、`tone`。円どうしは重なってよい。 |
| `label` | `text`、`x` `y`、`anchor`（`middle`。`start`・`end`）、`plain`（背景なし）、`strong`、`in`（ベン図で、内側に置く円の名前。ほかの円の外。`x` と `y` を省けば floe が置く）。 |
| `line` | `x1` `y1` `x2` `y2`、`kind`（`line`。`lifeline`・`divider`）。ほかの部品と交わってよいが、キャンバスから出ない。 |
| `tree` | `x` `y`（根の中心と上）、`root`：節。節は `box` と同じ `rows`・`name`・`tone`・`shape` と、子の節を並べた `children` を持つ。floe が各節を親の下に置き（子は横に並べ、深さごとに 1 段にそろえる）、親から子へ線を引く。節の `name` は、手で書いた線の端にできる。 |
| `raw` | `svg`：どの部品でも描けない SVG。SVG の要素だけで（script や `foreignObject` は無し）、配置も確かめもしない。 |

`via` も辺も無い線は、相手に向いた辺から、直角に、曲がり 2 つまでで進みます。別の道にするには `via` か辺を書きます。

確かめること：

- 箱に対して広すぎる・高すぎる字（frame のラベルは frame の幅に対して）。
- キャンバスからはみ出す部品（線・line・axis も）。
- 箱・印・ラベル・frame のラベルどうしの重なりと、frame の縁をまたぐ箱と印。frame どうしは比べないので、交わってよい。
- 箱・ラベル・frame のラベルを通る線と、X の形に交わる線（片方に `crossing: true` があればよい）。
- どの部品も持たない名前や、端の部品に無い辺に着く線。

## スクリプトと HTML

- `script` は `js`（ページが、その場所の空の要素と `data` を渡して呼ぶ関数 `function (el, data) { … }`）と `data`（どんな値でも）です。`data` の中のマッピングは鍵の名前の順で渡ります（順が大事ならリストで書く）。関数はその場所で、ページの残りを読む前に動きます。`</script` や `<!--` を含む `js` は断ります。関数の中の誤りはブラウザに出ます。
- `html` の部品は、そのまま入ります。要素を開いたままにすることも、周りのページの要素を閉じること（開いたままの `<b>`、`</section>`）もできず、部品の所で断ります。要素は丸ごと書きます。描いたページは、誰が書いても `<style>`・`<base>`・`<noscript>`、charset と viewport のほかの `<meta>` を持てず、データファイルの `template:` の所で断ります。HTML の見た目は `style` 属性で付けます。
- データファイルから描いたページのスクリプトは、ページの中身を外へ送れません。`fetch`・XHR・ほかの所の画像・フォームは CSP が止めます。cdnjs・jsDelivr・unpkg のライブラリと Google Fonts は読めます。

## 誤りの読み方

`floe check` は誤りを 1 行ずつ出し、誤りがあれば終了コードは 1 です。データファイルは段ごとに確かめます（[templates.ja.md](templates.ja.md#確かめる順)）。1 つの段の誤りは全部まとめて言い、前の段に誤りがあれば、後の段は走りません。だから、まず形を確かめ：

```text
plan.yaml:5:5: additional properties 'colour' not allowed
plan.yaml:8:23: value must be one of 'accent', 'good', 'warn', 'bad'
```

それを直すと、その後の確かめに進みます。

```text
plan.yaml:8:36: write the command without its prompt, which floe draws from the shell ("$ ls")
```

- `file:line:column:` が誤りの場所です。変える値か、それを持つ鍵を指します。言われた誤りを全部直してから、`floe check` をもう一度走らせます。次の段の誤りが出ることがあります。
- 形の誤り（`additional properties 'x' not allowed`・`missing property 'x'`・`got string, want array`）は、テンプレートか部品の schema がデータを断ったものです。`floe template show <テンプレート>` と `floe part show <部品>` が、取るものを出します。
- 自分の部品・テーマ・テンプレートの誤りは、使うかどうかによらず、どのページも止めます。誤りはそのファイルの所で言います（`~/.config/floe/parts/quote/schema.json:2:1: …`）。`floe part check` と `floe template check` が並べます。
- 同じ誤りは、何か所から指されても 1 回だけ言います。

### 警告

描いたページに警告が付くことがあります。より良い書き方の勧めで、ページは描かれ、終了コードも変わりません（誤りが無ければ 0）。`floe check`・`floe render`・`floe template check`・`floe publish` が、誤りの後に出します:

```text
plan.yaml:12:15: warning: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button
```

- `text` の部品（report の節、design・handoff・adr・postmortem・runbook の `more` の中、runbook の手順の `text`、`details`・`tabs` の中も）の、囲んだコードのブロックを警告します。コードは `code` に、コマンドは `terminal` に書きます。
- 誤りのあるファイルは描かないので、その警告は誤りを直した後に出ます。ほかの描けたファイルの警告は、誤りと一緒に出します。

## 確かめと公開

- `floe check <data.yaml>...` は、何も書かず、サーバーも起こさずにデータファイルを確かめます。変えるたびと、リポジトリの CI で走らせます。
- `floe publish <data.yaml>` は、描いたとおりにページを保存して URL を出します。もう一度 publish すると上書きします。`floe publish --link <data.yaml>` は、開くたびにファイルから描き、保存すると開いたタブを読み直すので、書きながら読めます。
- `floe render <data.yaml>...` は、floe 無しで開ける 1 つの HTML のファイルを、データファイルの隣に書きます。

フラグは [commands.ja.md](commands.ja.md) にあります。
