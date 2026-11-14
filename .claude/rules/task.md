---
paths:
  - "internal/store/**"
  - "internal/server/**"
  - "internal/cli/**"
  - "internal/ui/**"
---

# タスク

- **タスクは tasks.json 1 つ**：タスクの状態・カテゴリの順・URL は tasks.json に持ち、書くのは store の lock の中だけ。状態の値は `store.States` 1 つから作る。タスクの一覧の置き場所は状態だけで決まり、ページの印から計算しない。ページがどのタスクのどのカテゴリにあるかは meta.json の `task` だけが持つ。ページが名指すタスクとカテゴリには必ず記録があり、記録の無いものを推して扱わない（読むときは断らない）。タスクとページのアーカイブは別々で、どの入口も片方しか書かない
- **タスクの URL は足す・外す・並べ替えで書く**：行の `urls` は `URLsEdit` の `add`・`remove`・`order` だけで変える。足す・外すは変えた URL だけを送る。`order` は前に置く順で、書かない URL は今の順のまま後ろに残る。画面は順を変えたときだけ `order` を送る。上限は、足して増える書き込みだけを断る（`checkURLCount`）
- **ページとタスクは 1 つの操作で書く**：ページの `task` とタスクの記録を書くのは `Store.ChangeTasks`（API は `PATCH /api/tasks`）1 つで、同じ lock の中で全部を確かめてから、tasks.json、meta.json の順に書く。名指すタスクの記録が無ければ作り、カテゴリが無ければ `categories` の末尾に足し、記録が変われば `rev` を進め、答えで触ったタスクを全部返す。ページのプロジェクトが変わる書き込み（`Store.Edit`・publish の `commit`）は、同じ lock の中でページをタスクから外し、タスクは書かない。ページの編集（`PATCH /api/pages`）は `task` を持たない
- **プロジェクトの比べは ProjectKey 1 つ**：meta.json の空と tasks.json の `/` の対応は `store.ProjectKey` だけ。CLI と画面は `GET /api/pages`・`PATCH /api/tasks` の答え・publish の答えの `taskProject` を使い、自分で写さない。API の本文・答え・query では、tasks.json の鍵の形のプロジェクトは `taskProject` で、`project` は書いたままの名前（ページの編集と publish の本文、`GET /api/pages` の 1 件の `project`）だけ
- **親子は 1 段だけ**：確かめるのは `Store.ChangeTasks` で、読むときは断らない。画面の親子は `tasksOf` だけが決める。新しい親は `parentNew` と `parentTitle` で子と一緒に 1 回で書く
- **画面のタスクの書き込みは、見た版を送る**：画面は `PATCH /api/tasks` に、名指す 1 つのタスクがあると思うか（`new`）と、あるタスクなら見た行の `rev` を送る。ページに描いたボタンは送る時の `tasksRead` の行の `rev`、開いて見せているもの（状態の select・モーダル・取り消しのトースト）は見せている行の `rev` を送る。通った書き込みの答えの `tasks` は、`changeTasks` 1 か所が `tasksRead` に入れる。親を書くときは `parentNew`（作るなら `parentTitle`）も送り、知らない親には送らない。違えば `Store.ChangeTasks` が 409 で断り、画面は読み直す。CLI はどれも送らない
