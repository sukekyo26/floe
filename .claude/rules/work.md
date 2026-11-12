---
paths:
  - "internal/store/**"
  - "internal/server/**"
  - "internal/cli/**"
  - "internal/ui/**"
---

# タスク

- **タスクは tasks.json 1 つ**：タスクの状態・カテゴリの順・URL は tasks.json に持ち、書くのは store の lock の中だけ。状態の値は `store.States` 1 つから作る。タスクの一覧の置き場所は状態だけで決まり、ページの印から計算しない。ページがどのタスクのどのカテゴリにあるかは meta.json の `work` だけが持つ。ページが名指すタスクとカテゴリには必ず記録があり、記録の無いものを推して扱わない（読むときは断らない）。タスクとページのアーカイブは別々で、どの入口も片方しか書かない
- **ページとタスクは 1 つの操作で書く**：ページの `work` とタスクの記録を書くのは `Store.Work`（API は `PATCH /api/work`）1 つで、同じ lock の中で全部を確かめてから、tasks.json、meta.json の順に書く。名指すタスクの記録が無ければ作り、カテゴリが無ければ `categories` の末尾に足し、記録が変われば `rev` を進め、答えで触ったタスクを全部返す。ページのプロジェクトが変わる書き込み（`Store.Edit`・publish の `commit`）は、同じ lock の中でページをタスクから外し、タスクは書かない。ページの編集（`PATCH /api/pages`）は `work` を持たない
- **プロジェクトの比べは ProjectKey 1 つ**：meta.json の空と tasks.json の `/` の対応は `store.ProjectKey` だけ。CLI と画面は `GET /api/pages`・`PATCH /api/work` の答え・publish の答えの `taskProject` を使い、自分で写さない。API の本文・答え・query では、tasks.json の鍵の形のプロジェクトは `taskProject` で、`project` は書いたままの名前（ページの編集と publish の本文、`GET /api/pages` の 1 件の `project`）だけ
- **親子は 1 段だけ**：確かめるのは `Store.Work` で、読むときは断らない。画面の親子は `tasksOf` だけが決める。新しい親は `parentNew` と `parentTitle` で子と一緒に 1 回で書く
- **画面のタスクの書き込みは、見た版を送る**：画面は `PATCH /api/work` に、名指す 1 つのタスクがあると思うか（`new`）と、あるタスクなら見た行の `rev` を送る。ページに描いたボタンは送る時の `tasksRead` の行の `rev`、開いて見せているもの（状態の select・モーダル・取り消しのトースト）は見せている行の `rev` を送る。通った書き込みの答えの `tasks` は、`writeWork` 1 か所が `tasksRead` に入れる。親を書くときは `parentNew`（作るなら `parentTitle`）も送り、知らない親には送らない。違えば `Store.Work` が 409 で断り、画面は読み直す。CLI はどれも送らない
