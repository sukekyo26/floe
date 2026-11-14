# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

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
