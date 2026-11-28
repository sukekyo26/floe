# リポジトリの文書

リポジトリの仕様書・設計書・手順書を、データファイル（YAML）で持ち、そのリポジトリのテンプレートで描き、CI で確かめられます。floe は、足りない鍵や打ち間違えた鍵、収まらない図、描き直していないページを断ります。テンプレート・データファイル・描いたページ（HTML）は、全部コミットします。エージェントはデータファイルを直し、人は自分のマシンの floe のサーバーでページを読みます。

## 置き方

```text
.floe/
  repository.yaml         設定（floe の版と、文書の置き場所）
  templates/api-spec/     page.html・schema.json・example.yaml など
  parts/                  自分の部品（あれば）
  themes/                 自分のテーマ（あれば）
docs/specs/
  orders.yaml             直すデータファイル
  orders.html             そのページ（floe repository render が書く）
  invoice.yaml
  invoice.html
.gitattributes
```

`.floe/repository.yaml`（`floe repository new` が書く）：

```yaml
version: '0.4.0'
documents:
  - 'docs/specs'
```

- `floe repository` は、このファイルでリポジトリを見つけます。リポジトリの中のどのディレクトリからでも上へ探し、`.git` のあるディレクトリより上は探しません。そのコマンドが読むテンプレート・部品・テーマは `.floe/` のものだけで、`~/.config/floe` や `FLOE_*_DIR` の設定にあるものは読まず、サーバーも保存先も使いません。`.floe/` の中の無いディレクトリは「無い」として扱うので、`.gitkeep` は要りません。
- `version` は、ページを描く floe の版です。別の版の floe は、ほかを読む前に、両方の版を言って断ります。ソースからビルドした floe は、版の後に `+` と commit が付き（`0.4.0+67f6e156bc78`）、`+` の前を比べます。そうした、リリースとリリースの間の floe は違う形に描くことがあり、それは check が「描き直していないページ」として言います。
- `documents` には、文書のディレクトリを、根から `/` で区切り、前に `./`、後ろに `/` を付けずに書きます（`docs/specs`）。その下の `.yaml` と `.yml` は、フォルダの中のものも全部文書です。点で始まる名前の下と、ディレクトリへのリンクの下は見ません（そのディレクトリは `documents` に書きます）。データファイルの無い HTML には何もしないので、手で書いたページも置けます。データファイルを消した後に残ったページも同じで、check は見つけません。
- floe は、`floe icons` のアイコン集のほかは、リポジトリの外を読みも書きもしません。リポジトリの外へのリンクのページ・データファイル・`.floe/` のディレクトリは断り、`.floe/` のディレクトリの中の、そのディレクトリの外へのリンクも断ります。
- 点で始まる名前は、リンクしたフォルダの一覧に出ないので、`.floe/` はページと並びません。
- ページは、データファイルが `theme:` で名指すテーマ、無ければ `default` で描きます。管理ページで選んだテーマはリポジトリのページに届かず、テンプレートは自分のテーマを持てません。

`.gitattributes` で、ページと比べるものと、ページにそのまま入るものを、どのマシンでも同じバイトにし、GitHub の差分ではページを畳みます。

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

`docs/specs` に手で書いた HTML を置くときは、これらの後に、それを `-linguist-generated` で名指す行を足します（`/docs/specs/notes.html -linguist-generated`）。

## 最初のテンプレート

リポジトリの根で走らせます：

```sh
floe repository new docs/specs
mkdir -p docs/specs
floe repository template new api-spec
```

`floe repository new` は `documents` のディレクトリを作らないので、`mkdir` で作ります。floe のテンプレートだけを使うなら（`template: tech-spec`）、`template new` は要りません。[templates.md](agents/templates.md) のとおりに 3 つのファイルを書き（そこに出るコマンドには、頭に `floe repository` を付けます。`floe repository template check`）、`docs/specs/orders.yaml` を `template: api-spec` で書きます。`schema.json` には `"additionalProperties": false` を書きます（floe が作るものは、どの鍵も通します）。文書は、floe のテンプレート（`template: tech-spec`）も名指せます。

## 描く・確かめる

ページを描きます：

```sh
floe repository render
```

確かめます（CI がすることです）。リポジトリのテンプレート・部品・テーマを、各テンプレートの `example.yaml` を描いて確かめ、次に全部の文書を確かめて、誤りを全部言います。文書の誤りが無くなると、無いページ・描き直していないページを言います。

```sh
floe repository check
```

- どちらも、リポジトリのどのディレクトリからでも走らせられ、見つけた文書を根からの名前で言います。データファイルを渡すと、それだけを描く・確かめます。渡すパスは、今いる所からのパスです（根で `floe repository render docs/specs/orders.yaml`）。
- データファイルに 1 つでも誤りがある間、`floe repository render` はどのページも書かず、check はページより先に誤りを言います。
- 2 つを、プロジェクトのコマンドを走らせる仕組みに置きます。

## CI

どちらも floe のサーバーを使わずに描きます。ページを描いた版（`.floe/repository.yaml` が名指す版）を入れます。PR がマージされる先のブランチ（ここでは `main`）でも確かめます。別々には通る 2 つの PR が、両方マージされると落ちることがあるからです（片方がテンプレートを変え、もう片方が文書を足したときなど）。

GitHub Actions の `.github/workflows/specs.yml`：

```yaml
name: specs
on:
  pull_request:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  specs:
    runs-on: ubuntu-latest
    steps:
      - uses: 'actions/checkout@v7'
      - name: 'Install floe'
        run: |
          curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.4.0/install.sh | FLOE_VERSION=0.4.0 sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - name: 'Check the documents'
        run: 'floe repository check'
```

AWS CodeBuild の `buildspec.yml`：

```yaml
version: 0.2
phases:
  install:
    commands:
      - 'curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.4.0/install.sh | FLOE_VERSION=0.4.0 FLOE_INSTALL_DIR=/usr/local/bin sh'
  build:
    commands:
      - 'floe repository check'
```

## 文書を直す

1. `.html` でなく `.yaml` を直します（`.html` は、次に描くと上書きされます）。
2. ページを描き（`floe repository render`）、`.yaml` と `.html` を一緒にコミットします。

- マージがぶつかったら、`.yaml` を直して描き直し、`.html` を add します（`.html` は、ぶつかりの印ごと書き直されます）。
- 文書を消すときは、両方のファイルを消します。名前を変えるときも、古いページを消します。
- テンプレート・部品・テーマを変えたら、描き直した全部のページを、その変更と一緒にコミットします。

## ページを読む

```sh
floe publish --link docs/specs
```

マシンごとに 1 回走らせます。フォルダは管理ページの 1 行になり、ページは木で並びます（[links.ja.md](links.ja.md#フォルダ)）。開いているページは、描き直すと読み直されます。

リポジトリのテンプレートで描く `docs/specs` のデータファイルは publish しません。サーバーはリポジトリのでなく自分のテンプレートで描くので、そのデータファイルを断ります（`unknown template "api-spec"`）。`floe publish` は `.floe/repository.yaml` を読みません。

## エージェントに知らせる

リポジトリの AGENTS.md（か CLAUDE.md）に次を書くと、文書を直すエージェントに伝わります。

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe repository guide` first, and commit its data file (.yaml or .yml) and its .html together. Do not edit the .html. Draw with `floe repository render`, check with `floe repository check`. Their templates are in .floe/, which some searches leave out.
```

## 気を付けること

- **誰も変えていないページが落ちるとき**：ページを描いたのと別の版の floe か、その間にマージされた PR がテンプレート・部品・テーマを変えたときです。最新を取り込み、描き直して、ページをコミットします。
- **floe の版**：別の版はページを違う形に描くことがあるので、`floe repository` は `.floe/repository.yaml` が名指すのと別の版を断ります。版を上げるのは専用の PR にします。`.floe/repository.yaml` と CI（同じ行に 2 か所）に新しい版を書き、入れて、`floe repository render` を走らせ、ページをコミットします。ソースからビルドした、リリースとリリースの間の floe は、`+` の前の版が同じでも違う形に描くことがあります。ページは、版に `+` の無いリリースの floe で描きます。
- **AWS・Google Cloud・Azure のアイコン**：floe は同梱しないので、描く前に、各マシンと CI が取ってきます（CI では Install floe の次の段。[commands.ja.md](commands.ja.md#floe-icons-fetch-セット-zip-か-url)）。`floe icons fetch aws` と `floe icons fetch gcp` は最新のセットを取り、アイコンが変わるとページも変わります。AWS を 1 つのセットに留めるには、その zip の URL（日付が入る）を、CI でも自分のマシンでも渡します（`floe icons fetch aws https://…/Icon-package_….zip`）。Google の 2 つの zip は、中身が変わっても URL が同じなので、zip を取っておいて両方を渡します。Azure の zip は、条件に同意して手で落とし、`floe icons fetch azure` に渡します。Lucide のアイコンは組み込みです。
- **ページが多いとき**：1 回の `floe repository render` が使えるメモリは、そのページ全部を合わせて 512 MiB までです（[commands.ja.md](commands.ja.md#floe-render-datayaml)）。超えたら、`docs/specs` のフォルダのデータファイルを、1 つのフォルダずつ渡します。
