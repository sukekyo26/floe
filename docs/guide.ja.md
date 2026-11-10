# floe の使い方：エージェント向けの手引き

floe は、あなたが YAML のファイルで書いたページをユーザーに見せます。ファイルを検査し、テンプレートで描いて、`floe publish` が出す URL で配ります。要ることは、この手引き、`floe <command> --help`、`list` と `show` のコマンドが言います。

（`floe guide` が出すのは英語の [guide.md](guide.md) です。これはその訳です。）

## ページを書く

1. `floe template list` からテンプレートを選ぶ：
   - ページの目的のために作られたものがあれば、それ（設計書なら `design`、`handoff`、`runbook` など）
   - 無ければ `report`
   - 書く前に、見本と schema を読む：`floe template show <template> example.yaml` と `floe template show <template> schema.json`。見本はファイルの組み立て（項目と節の組み方）を見せるもので、その部品を決まった組と思わない。部品は手順 3 で選ぶ
2. ファイルにはページの名前を付ける（`db-choice.yaml` など）。`.yaml` を除いたファイルの名前が、ページの URL の名前（slug）になります。データファイルには必ず `template` と `lang` を書く。`title` はページの題、`report` の `lead` は題の下の要約です。`theme` は書かない（[テーマ](#テーマ)）。

   ```yaml
   template: report
   lang: en
   title: Moving the team wiki
   lead: The wiki moves in two weekends.
   sections:
     - heading: Steps
       parts:
         - text: Freeze edits on **Friday evening**, then copy the pages.
   ```

3. `floe part list` を走らせ、手順 1 で読んだテンプレートの schema が許す部品から、中身に合うものを選ぶ。迷いやすいときは[部品を選ぶ](#部品を選ぶ)の表を使う。見本に無い部品は、書く前に、その節のとおりに鍵を調べる。推した鍵は断られます。
4. `floe check db-choice.yaml` を走らせ、誤りを全部直して、誤りが無くなるまで走らせ直す。`warning:` の行だけが残ったら済みです。これは勧めです。
5. `floe publish db-choice.yaml` を走らせ、出た URL をユーザーに渡す。ページを自分で取りに行かなくてよい。

## 出し直す

- floe はページを、floe を走らせた場所でなくファイルで見分けます。同じファイルを、どのディレクトリからでも、`--slug` を付けずに publish し直す。ユーザーが管理ページで改名した後でも、そのページを置き換えます。
- 名前を断られたら、別のファイルのページがその名前を使っています。ファイルの名前を変えるか（`db-choice-2.yaml`）、別の `--slug` を付けて出し直す。

## テーマ

- ユーザーがそのページのテーマを名指さない限り、`theme` を書かない。テーマはユーザーが管理ページ（ページの URL の根）で選びます。
- URL をユーザーに渡すときに `floe theme default` を走らせる。名前が出たら、テーマのことは何も言わない。何も出なければ、新しいページの既定のテーマを管理ページの設定（歯車）で選べることを、会話で 1 回だけ伝える。

## 部品を選ぶ

`parts` の各項目は、部品を名指す鍵 1 つです。`floe part list` が各部品の鍵と使い道を出します。鍵が並ばない部品（`chart`・`stats`・`diagram` など）は、`floe part show <part> schema.json` を読んで、鍵と型と詳しいことを見る。手順 1 で読んだテンプレートの `schema.json` に、節や欄ごとに置ける部品が書いてあります。

合う部品が分かりにくいときは、この表を使う：

| 中身 | 部品 |
|---|---|
| 文、または短い並び | `text`（Markdown） |
| 順にすること、したこと | `steps`。floe が番号を振るので、題に番号を書かない |
| 長めの説明の付く項目（リスク、要件、用語など） | `entries`。短い状態や印は `tags` に書く（`tags: [done]`、いくつでも） |
| 長所と短所を比べる案 | `options` |
| 理由の付いた 1 つの決定 | `decision` |
| 読み手に決めてほしい問い | `asks` |
| 確かめた結果、テストの結果 | `checks`（各項目に `result: ok`・`ng`・`unchecked`。floe が結果の数を上に出す）。選ぶ条件（`table`・`options`）やまだすること（`checklist`）には使わない |
| 日付のある出来事 | `timeline`。日付の無い段は `steps` |
| 行と列で比べる短い値。前と後の値は別々の列に（先頭の列の見出しは空にする） | `table` |
| 横に並べて見る、互いに独立したまとまり。それぞれに部品（画面やコードの前と後に） | `columns` |
| 頭に置く少しの値 | `facts` |
| 数の動き、数の比べ | `chart` |
| 構成や流れ：箱と矢印 | `diagram`（[図](#図)） |
| コード、コマンドと出力 | `code`、`terminal`（`$ ` は書かない） |

表や図を `text`・`code` の中に字で描かない。値はこう書きます：

```yaml
- text: |
    One paragraph of **Markdown**.
- note: { tone: warn, text: Edits during the freeze are lost. }
- facts:
    - { name: Owner, value: Ana }
- steps:
    - { title: Freeze edits, state: done }
- entries:
    - { title: Broken links, tags: [120 pages], text: A link check runs after the copy. }
- checks:
    - { title: Pages copied, result: ok, actual: All 1840 pages. }
    - { title: Search, result: ng, actual: The index is empty. }
    - { title: Old links, result: unchecked, actual: Waits for the copy. }
- table:
    columns: [Page, { title: Links, num: true }]
    rows:
      - [Home, "12"]
```

## 図

`graph` を書く：箱と、何が何につながるか。箱は floe が置くので、箱の位置は考えない。`x` と `y`（必須）は graph の始まりの位置だけを言います。

```yaml
- diagram:
    title: How a page is published
    parts:
      - graph:
          x: 20
          y: 20
          nodes:
            - box: { name: write, rows: [Write the data] }
            - box: { name: check, shape: hex, rows: ["Checks pass?"] }
            - box: { name: show, rows: [Show the page] }
          edges:
            - { from: write, to: check }
            - { from: check, to: show, label: "yes" }
            - { from: check, to: write, label: "no" }
```

`floe diagram example` が種類ごとの見本（構成、シーケンス、ER など）を並べ、`floe diagram example <kind>` が 1 つを写せる形で出します。字がはみ出す図、部品が重なる図は、直す行で断られます。

## floe が書いたとおりに読む YAML

まず：

- `: `・` #` を含む文、`{ }`・`[ ]` の中で `,` を含む文は引用符で包む：`heading: "Step 1: plan"`。包まないと、YAML は鍵、コメント、文の終わりと読みます。floe はほとんどを断りますが、`[ ]` の中の `,` は黙って項目を分けます：`tags: [Tokyo, Osaka office]` はタグ 2 つです。
- コメントは、それだけの行に、上の値の鍵（リストの項目なら `-`）より深くなく書き、`{ }`・`[ ]` の中には書かない。
- `|`・`>` の文は、1 行目を後の行と同じ深さに字下げする。1 行目が深いと文がそこで終わり、後の行が断られます。
- 表の升のような文を取る所の数は、引用符で包む：`"42"`。
- 文書は 1 つ：`---`、アンカー、エイリアス、マージの鍵（`<<`）、タグは書かない。

次に：

- 日本語・中国語・韓国語の字の隣では、`**`・`*`・`~~` は約物をまたいでも読まれます（`**「題」**に` は太字）。記号のつもりの `*` は、そこでは `\*` か backtick で書く。
- `|`・`>` の文は、空白だけで字下げする。1 行目の空白のすぐ後のタブは断られます。

`heading: Step 1: plan`、`lead: PR #100`、表の升の `42` は、こう断られます：

```text
h.yaml:5:14: mapping value is not allowed in this context
c.yaml:4:10: a # began a YAML comment, which may have cut the value short, to PR: if the # is part of the value, put the value in quotes, or, in a | or > text, indent all its lines alike, the # line too, or write its indentation after the | or > (like |2); else write the comment on a line of its own, no deeper than the value's key (or the - of an item)
t.yaml:10:16: got number, want string or object
```

## 誤りを読む

誤り（mistake）は書いたものの中にあるので、書き手が直す。失敗（failure）は floe 自身の中にあるので、出た文をユーザーに伝える。

`floe check` は誤りを 1 行に 1 つ、頭に `file:line:column:` を付けて（ファイル全体の誤りは `file:` だけを付けて）出し、終了コード 1 で終わります。検査は段ごとなので、誤りを直すと、次の段の誤りが出ることがあります。

- schema の誤りには、部品が受けない鍵、足りない必須の鍵、型の違う値がある：[部品を選ぶ](#部品を選ぶ)のとおりに schema を読む。
- YAML の誤り：上の節を見る。

```text
plan.yaml:8:11: additional properties 'colour' not allowed
plan.yaml:9:17: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

## ほかに

- `floe render db-choice.yaml` は、リポジトリに置く `db-choice.html` を隣に書きます。
- `floe publish --link file.html` は、自分で作った HTML のファイルをそのまま見せます。
- プログラムからデータファイルを書き換えるときは、アンカーとエイリアスを書かず、ASCII でない字と鍵の順を保つ設定の YAML ライブラリで読み書きする（PyYAML なら `allow_unicode=True`、`sort_keys=False`、`ignore_aliases` が true を返す Dumper）。
