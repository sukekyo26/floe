# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- 追加：`install.sh` と `install.ps1` に `FLOE_INSTALL_TIMEOUT`。何も届かない状態を、1 回のリクエストごとに何秒まで待つかを指定する（既定は 120）。

### 変更

- 変更：schema が `items` を持つ並び（draft 2020-12）の、`prefixItems` が受け持つ先頭の項目。`prefixItems` だけの並びの項目と同じく、書いたまま `page.html`・`part.html` に渡り、schema が持つ鍵を項目が省いても nil にならず、鍵が無い。その鍵は `index` で読む（`{{with index . "more"}}…{{end}}`）。`.more` で読んでいたページは `map has no entry` で断られるようになる。
- 変更：`|` や `>` の文の直後の行に出る YAML の誤り（文の最初の行が後の行より深いときなど）。YAML のライブラリの言葉の後に、文がその行の前で終わっていることと直し方（文の行の字下げを揃えるか、`|2` のように字下げの数を書く）を表示する。前半の言葉と場所は変わらない。
- 変更：`floe part check` と `floe template check` が、`$ref` だけをたどって自分に戻る `$ref`（部品の根が、根でその部品を指す別の部品を指す。`$defs` が自分を指す）を、schema のファイルの、その `$ref` の所で断る。そういう `$ref` を通る値は無いが、今まではデータファイルに値を書いたときにだけ、データファイルの行で断られていた。値が届かない所や、`not`・`anyOf`・`if` の中にそれを持つ自分の schema は、今までは描けたが、自分の部品とテンプレートを読むどのコマンドでも断られるようになる。

### 修正

- 修正：`prefixItems` の横の `items` が、`prefixItems` が受け持つ項目まで描いていた。`items` の部品や floe の format が並びの全部の項目を描いていたが、JSON Schema では `items` が当たるのは `prefixItems` の後ろの項目だけで、検証していない値で描く段がデータファイルの行で止まることがあった。後ろの項目だけを描くようにした。
- 修正：テンプレートや部品の `checks.yaml` の `for` が、`prefixItems` が受け持ち `items` が受け持たない先頭の項目を持つ並びを通るとき、schema が許さない鍵の道として断られていた。そういう道は断らなくなった。どの項目も持てない鍵の道も、そこでは断らない。
- 修正：`install.sh` と `install.ps1` が、サーバーや途中のプロキシが応答しなくなると、いつまでも待ち続けていた。応答が無いまま 120 秒たつと打ち切り、取得しようとしていた URL と対処を表示する。ダウンロード全体の時間に上限は無いので、遅いダウンロードは最後まで続く。ダウンロードが途中で止まった場合、Windows PowerShell 5.1 は 5 分で打ち切り、PowerShell 7.0〜7.3 は打ち切らない。
- 修正：管理ページのモーダルが閉じた直後に押したキーが、閉じたモーダルに届いていた。編集のモーダルで Esc の直後に Enter を押すと、Esc で取り消した変更が保存されることがあり、検索のモーダルでは選んでいたページへ移ることがあった。モーダルが閉じると同時にフォーカスがモーダルの外へ出るようにした。
- 修正：ページを別のタスクへ移すかの確かめで「はい」の直後に Enter を押すと、「はい」が捨てられ、ページが移らずにモーダルが開いたまま残ることがあった。
- 修正：管理ページのモーダルを保存中に閉じると、保存は続き、その後の処理がその時に開いているモーダルを相手にしていた。別のページで開き直したモーダルを閉じたり、開き直した編集の次の保存を、前の保存が改名したページへ送ってそのページの URL 名と題を書き換えたり、見えない所に失敗を書いたりした。保存とその後の一覧の読みの間、モーダルを止めるようにした（Esc・キャンセル・外側の押し・中のボタンは、保存の答えまで効かず、失敗はモーダルの中に出る）。保存の答えが返らないときは、ページを読み直す（F5・Ctrl+R）まで止まったままになる。
- 修正：モーダルの欄で押し下げて外で離すと、外側の押しと同じくモーダルが閉じていた。外で押し下げた押しだけで閉じるようにした。
- 修正：schema が `prefixItems` の横の `items` で自分の部品を指す並びを持つページを描くと、その部品の根と別の部品の根が `$ref` で互いに戻るとき、floe が `fatal error: stack overflow` で落ちていた。そういう部品は、読むときに断られる。

## [0.2.0] - 2026-11-17

### 追加

- 追加：Windows 11（amd64・arm64）のビルドと、それを入れる `install.ps1`（`irm https://raw.githubusercontent.com/sukekyo26/floe/main/install.ps1 | iex` で `%USERPROFILE%\.local\bin` に入る）。floe の置き場所は、Linux の `~` の下と同じく `%USERPROFILE%` の下。開くのは手元のドライブの上のふつうの名前のものだけで、ネットワークの共有・ネットワークドライブ・デバイス・パイプ・ストリームの名前を断り、`AppData` の下（`%TEMP%` も）のファイルのリンクも断る。ほかに違うことは docs/commands.ja.md の Windows の節にある。

### 変更

- 変更：ポートのサーバーが配信する保存先がこのシステムの絶対パスでない（Windows から見た WSL の floe など、別のシステムのサーバー）ときのコマンドの断りが、`FLOE_PORT` を別のポートにすることだけを言い、`FLOE_DIR` をその保存先にすることと `floe stop` を勧めないようにした。
- 変更：`floe licenses` が、どの OS でも、Windows のビルドが持つ `github.com/inconshreveable/mousetrap` のライセンスも出すようにした。
- 変更：Linux・macOS でない OS で `install.sh` を走らせると、Windows では `install.ps1` で入れることと、その入れ方を言うようにした。
- 変更：データファイルが名指す画像のパスを断る文が、Windows では `\`、`:`、`NUL`・`CON` などのデバイスの名前も書けないと言うようにした。NUL の字を含む画像の名前も、この文で断るようにした。
- 変更：リンクしたフォルダとリンクしたファイルのディレクトリで、名前が UTF-8 でないファイルとディレクトリを配信しないようにした。その URL は 404 になる。
- 変更：ホームの隠しディレクトリの中のリンクを断る文が、アプリのデータの場所も言い、ファイルをどこに置けばよいかを言うようにした。
- 変更：実のパスでないリンクを断る文が、呼ぶ Go の関数を名指していたのをやめ、リンクと junction をたどった、システムの言う全体のパスを渡すよう言うようにした。
- 変更：置き場所が `PATH` に無いときに `install.sh` が言うこと。足すコマンドを、写して貼ればそのまま動く形で見せる（zsh は `~/.zshrc`、Linux の bash は `~/.bashrc`、fish は `fish_add_path`。sh の仲間のほかのシェルには足す行だけ）。`PATH` もファイルも、今までどおり変えない。相対パスで渡した置き場所は絶対パスで言い、絶対パスで比べるので、もう `PATH` にある置き場所を案内しなくなった。
- 変更：`floe guide` が、場所の無い断りのうちコマンドに渡したもの（パス・フラグ・URL 名・タスク・自分で置いた `FLOE_…`）についてのものはエージェントが直し、ほかはユーザーに伝え、ユーザーの設定を変えずサーバーを止めない、と言うようにした。前は場所の無い断りを全部、ユーザーに伝える故障と言っていた。

### 修正

- 修正：ページの URL 名の変更でページを動かせなかったとき、説明・タグ・プロジェクト・題だけが保存され、ほかは失われていた。ページを先に動かし、動かせなければ何も保存しないようにした。動かした後に `meta.json` を書けなければ元に戻し、戻せないときは、URL 名だけが変わり、ほかの変更はもう一度保存すると言う誤りで断る。
- 修正：1 MiB を超えたログを `server.log.1` へ動かせないとき（そこにディレクトリがある、前のサーバーが開いたまま）、サーバーが起動しなかった。今のログへ書き続け、動かせなかった理由を 1 行書き、次の起動でまた試すようにした。

## [0.1.1] - 2026-11-16

### 変更

- **BREAKING**: 変更：`floe task --categories` を、`--url` と同じくカテゴリ 1 つごとに繰り返す旗にした（`--categories designs --categories samples`）。名にカンマを使える。`--categories ""` で全部外すのは今のまま。`floe task` はカテゴリを URL と同じく 1 つずつ引用して出す：`categories "a,b"` でなく `categories "a" "b"`、書き込みで最後の 1 つを外した後は `categories ""` でなく `no categories`。ページのあるカテゴリの断りは名を引用符で囲む（`category "designs" of task …`）。その行と `floe info` の値（題・タスク・カテゴリ・タグなど）は、全角空白を `\u3000` でなくそのまま出す。
- 変更：`floe guide` のタスクモードの問い。エージェントは新しいページを publish するたびに、どのタスク（開いたタスクの先頭 2 件・新しいタスク・無し）に置くかと、そのタスクのどのカテゴリに置くかを、質問ダイアログで（無ければ問いだけの返事で）ユーザーに聞く。
- 変更：`floe task` とタスク一覧の並び。状態だけでなく、欄を変えたときや、ページを載せたか、そのタスクの別のカテゴリへ移したときも数え、最後に変えたタスクを先に並べる。タスクに出る時刻もその変更の時刻。ページだけを書いた `floe task --slug … --task …` も、そのタスクを `changed` の行で言う。

### 修正

- 修正：プロジェクトの無いタスクの断りが `project /` と言っていた。`floe task` と管理ページは、`no project`（日本語は、タスク一覧の「未設定」に合わせて `プロジェクト未設定`）と言う。管理ページの日本語の断りでは、名のあるプロジェクトも `プロジェクト「floe」` と囲む。無いタスクを `--parent` で名指したときは `no task #42 in project app can be a parent` と言う。
- 修正：`floe publish` に `--link` と `--move-project` を一緒に渡したときの断りに `run floe --help` が付かず、終了コードが 1 だった。ほかのコマンド行の誤りと同じく、断りの末尾に `run floe --help` を付け、終了コードを 2 にした。

## [0.1.0] - 2026-11-16

### 追加

- 追加：`floe publish`。データファイル（YAML）からテンプレートでページを描き、ローカルのサーバー（`http://127.0.0.1:7325`、動いていなければ起動する）で見せる。`--link` は自分の HTML のファイルやフォルダをそのまま見せる。
- 追加：データファイルを確かめる `floe check`、リポジトリに置くページを書く `floe render`、保存したページを扱う `floe list`・`floe info`・`floe tag`・`floe archive`・`floe rm`・`floe rebuild`・`floe serve`・`floe stop`。
- 追加：誤りを、誤った所のファイル・行・列で、直し方と一緒に言う。より良い書き方を勧める警告。
- 追加：floe のテンプレート `report`・`design`・`handoff`・`adr`・`postmortem`・`runbook`・`slides` と、自分のテンプレート（`floe template new`）。
- 追加：floe の部品（text・code・terminal・diff・table・entries・steps・stats・bars・chart・checks・asks・options・columns・image など）と、自分の部品（`floe part new`）。
- 追加：12 のテーマと、自分のテーマ（`floe theme new`）。既定のテーマとページのテーマは、人が管理ページで選ぶ。
- 追加：検査付きの inline SVG の図。枠・箱・線・表・木、線から並べる `graph`、見本（`floe diagram example`）。
- 追加：図のアイコン。Lucide を組み込み、AWS・Google Cloud・Azure のアイコン集を `floe icons fetch` で取り、`floe icons search` で探す。
- 追加：管理ページ。プロジェクトごとのページ、検索（`Ctrl+K`）、タグ、ピン、アーカイブ、ページの版と版の違い、未読、設定。
- 追加：プロジェクト。ページを初めて publish した git リポジトリの名前で決まる（`--move-project` で移す）。
- 追加：タスク。プロジェクトと状態ごとのタスク一覧、子のタスク、並べられる外のリンク、タスクの中のページのカテゴリ。`floe task` で読み書きする。
- 追加：タスクモード。入なら `floe guide` が、新しいページを publish した後に、タスクに載せるかをユーザーに聞くようエージェントに言う。
- 追加：エージェントの手引き `floe guide` と、`floe task guide`・`floe diagram guide`・`floe part guide`・`floe template guide`・`floe theme guide`、エージェントのスキルからそれを指す `floe skill install`。
- 追加：ページの CSP とサンドボックス。ページの script を floe のものだけにする。
- 追加：`floe licenses`。floe と、floe に入っているもののライセンスを出す。

[Unreleased]: https://github.com/sukekyo26/floe/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/sukekyo26/floe/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/sukekyo26/floe/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/sukekyo26/floe/releases/tag/v0.1.0
