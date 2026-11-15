# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 変更

- **BREAKING**: 変更：`floe task --categories` を、`--url` と同じくカテゴリ 1 つごとに繰り返す旗にした（`--categories designs --categories samples`）。名にカンマを使える。`--categories ""` で全部外すのは今のまま。`floe task` はカテゴリを URL と同じく 1 つずつ引用して出す：`categories "a,b"` でなく `categories "a" "b"`、書き込みで最後の 1 つを外した後は `categories ""` でなく `no categories`。ページのあるカテゴリの断りは名を引用符で囲む（`category "designs" of task …`）。その行と `floe info` の値（題・タスク・カテゴリ・タグなど）は、全角空白を `\u3000` でなくそのまま出す。
- 変更：`floe guide` のタスクモードの問い。エージェントは新しいページを publish するたびに、どのタスク（開いたタスクの先頭 2 件・新しいタスク・無し）に置くかと、そのタスクのどのカテゴリに置くかを、質問ダイアログで（無ければ問いだけの返事で）ユーザーに聞く。

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

[Unreleased]: https://github.com/sukekyo26/floe/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sukekyo26/floe/releases/tag/v0.1.0
