# リポジトリの文書

リポジトリの仕様書・設計書・手順書を、データファイル（YAML）で持ち、そのリポジトリのテンプレートで描き、CI で確かめられます。floe は、足りない鍵や打ち間違えた鍵、収まらない図、描き直していないページを断ります。テンプレート・データファイル・描いたページ（HTML）は、全部コミットします。エージェントはデータファイルを直し、人は自分のマシンの floe のサーバーでページを読みます。

## 置き方

```text
.floe/
  templates/api-spec/     page.html・schema.json・example.yaml など
  parts/.gitkeep          自分の部品（あれば）
  themes/.gitkeep         自分のテーマ（あれば）
docs/specs/
  orders.yaml             直すデータファイル
  orders.html             そのページ（floe render が書く）
  invoice.yaml
  invoice.html
.gitattributes
```

- 点で始まる名前は、リンクしたフォルダの一覧に出ないので、`.floe/` はページと並びません。
- `docs/specs` には、floe のデータファイル（名前は `.yaml`）とそのページだけを置きます。下のコマンドは、そこの `.yaml` を全部描き、`.yml` は描きません。
- `.floe/` の 3 つのディレクトリは、空なら `.gitkeep` を入れて、どれも置きます。下のコマンドが 3 つとも floe に指すので、誰かの `~/.config/floe` にあるものは、ページを描くのにも止めるのにも関わりません。無いディレクトリは floe が断り、空のディレクトリは git が残しません。

`.gitattributes` で、ページと比べるものと、ページにそのまま入るものを、どのマシンでも同じバイトにし、GitHub の差分ではページを畳みます。

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

## 最初のテンプレート

リポジトリの根で走らせます：

```sh
mkdir -p .floe/templates .floe/parts .floe/themes docs/specs
touch .floe/parts/.gitkeep .floe/themes/.gitkeep
env FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes" floe template new api-spec
```

パスは絶対パスです（floe は相対のパスを断ります）。[templates.md](agents/templates.md) のとおりに 3 つのファイルを書き、`docs/specs/orders.yaml` を `template: api-spec` で書きます。`schema.json` には `"additionalProperties": false` を書きます（floe が作るものは、どの鍵も通します）。文書は、floe のテンプレート（`template: design`）も名指せます。

## 描く・確かめる

どれもリポジトリの根で走らせます。ページを描きます：

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe render docs/specs/*.yaml
)
```

確かめます（CI がするのもこれです）。データファイルの誤りも言うので、`floe check` を並べて走らせる必要はありません：

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml
)
```

- 括弧が、設定を自分のシェルに残しません（残すと `floe publish` に届きます。[ページを読む](#ページを読む)）。
- 2 つは、プロジェクトのコマンドを走らせる仕組みのどれに入れても構いません。
- 文書を `docs/specs` の下のフォルダに分けるときは、`floe render docs/specs/*.yaml` を `find docs/specs -name '*.yaml' -exec floe render {} +` に、`floe render --check docs/specs/*.yaml` を `find docs/specs -name '*.yaml' -exec floe render --check {} +` に替えます。`+` を `\;` にしないでください（失敗が消え、検査が必ず通ります）。`.yaml` が 1 つも無いと、find は何も走らせずに通り、glob は落ちます。
- Windows では、WSL などの POSIX のシェルで走らせます（リポジトリは WSL の中に置きます）。PowerShell では、3 つを `"$PWD\.floe\templates"` のようなパスにし、プログラムに `*.yaml` を展開しないので、ファイルを自分で並べます（未確認です）。

データファイルの無くなったページは、floe は探しません。次の任意の 1 行が探し、隣に `.yaml` の無い `.html`（手で書いたものも）で落ちます。前の失敗が消えないように、`&&` でつなぎます：

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml &&
  find docs/specs -name '*.html' | while read -r f; do [ -f "${f%.html}.yaml" ] || { echo "no YAML for $f"; exit 1; }; done
)
```

## CI

どちらもサーバー無しで描きます。ページを描いた版の floe を入れ、どのマシンも同じ版にします。検査は、PR の統合先（ここでは `main`）でも走らせます。別々には通る 2 つの PR（片方がテンプレートを変え、もう片方が文書を足す）が、両方マージした後に落ちることがあります。ここでは設定は CI のジョブのもので、CI には、それを受け取る自分のサーバーはありません。

GitHub Actions（`.github/workflows/specs.yml`）：

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
          curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.3.1/install.sh | FLOE_VERSION=0.3.1 sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - name: 'Check the documents'
        run: |
          set -e
          export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
          floe template check
          floe render --check docs/specs/*.yaml
```

AWS CodeBuild（`buildspec.yml`）：

```yaml
version: 0.2
phases:
  install:
    commands:
      - 'curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.3.1/install.sh | FLOE_VERSION=0.3.1 FLOE_INSTALL_DIR=/usr/local/bin sh'
  build:
    commands:
      - |
        set -e
        export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
        floe template check
        floe render --check docs/specs/*.yaml
```

## 文書を直す

1. `.html` でなく `.yaml` を直します（`.html` は、次に描くと上書きされます）。
2. ページを描き（[描く・確かめる](#描く確かめる)）、`.yaml` と `.html` を一緒にコミットします。

- マージがぶつかったら、`.yaml` を直して描き直し、`.html` を add します（`.html` は、ぶつかりの印ごと書き直されます）。
- 文書を消すときは、両方のファイルを消します。名前を変えるときも、古いページを消します。
- テンプレート・部品・テーマを変えたら、描き直した全部のページを、その変更と一緒にコミットします。

## ページを読む

```sh
floe publish --link docs/specs
```

マシンごとに 1 回走らせます。フォルダは管理ページの 1 行になり、ページは木で並びます（[links.ja.md](links.ja.md#フォルダ)）。開いているページは、描き直すと読み直されます。

リポジトリの設定は自分のシェルに置かず、リポジトリのテンプレートで描く `docs/specs` のデータファイルは publish しません。サーバーは、起動したときのテンプレートで描くので、そのデータファイルを断り（`unknown template "api-spec"`）、ほかのテンプレートを指した `floe publish` は `--link` でも断ります。サーバーが動いていなければ、その publish が、自分の全部のページをリポジトリのテンプレートで描くサーバーを起こします。

## エージェントに知らせる

リポジトリの AGENTS.md（か CLAUDE.md）に次を、そのリポジトリの描くコマンドと確かめるコマンドを入れて書くと、文書を直すエージェントに伝わります。

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe guide` first, and commit its .yaml and .html together. Do not edit the .html. Draw with `<command>`, check with `<command>`. Their templates are in .floe/, which some searches leave out.
```

## 気を付けること

- **誰も変えていないページが落ちるとき**：ページを描いたのと別の版の floe か、その間にマージされた PR がテンプレート・部品・テーマを変えたときです。最新を取り込み、描き直して、ページをコミットします。
- **floe の版**：別の版はページを違う形に描くことがあります。版を上げるのは専用の PR にします。CI に新しい版を書き（同じ行に 2 か所）、入れて、ページを全部描き、コミットします。floe のソースからビルドした、リリースとリリースの間の floe は、違う形に描くことがあり、版の番号の後に `+` と commit が付きます（`0.3.1+09feb55824ca`）。ページは、版に `+` の無いリリースの floe で描きます。
- **AWS・Google Cloud・Azure のアイコン**：floe は同梱しないので、描く前に、各マシンと CI が取ってきます（CI では Install floe の次の段。[commands.ja.md](commands.ja.md#floe-icons-fetch-セット-zip-か-url)）。`floe icons fetch aws` と `floe icons fetch gcp` は最新のセットを取り、アイコンが変わるとページも変わります。AWS を 1 つのセットに留めるには、その zip の URL（日付が入る）を、CI でも自分のマシンでも渡します（`floe icons fetch aws https://…/Icon-package_….zip`）。Google の 2 つの zip は、中身が変わっても URL が同じなので、zip を取っておいて両方を渡します。Azure の zip は、条件に同意して手で落とし、`floe icons fetch azure` に渡します。Lucide のアイコンは組み込みです。
- **ページが多いとき**：1 回の `floe render` が使えるメモリは、そのページ全部を合わせて 512 MiB までです（[commands.ja.md](commands.ja.md#floe-render-datayaml)）。超えたら、`docs/specs` のフォルダを 1 つずつ走らせます。
