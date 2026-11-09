---
paths:
  - "internal/store/**"
  - "internal/server/**"
  - "internal/cli/**"
  - "internal/ui/**"
---

# タスク

- **タスクは tasks.json 1 つ**：タスクの状態・段・URL は tasks.json に持ち、書くのは store の lock の中だけ。状態の値は `store.States` 1 つから作る。タスクの一覧の置き場所は状態だけで決まり、ページの印から計算しない。ページがどのタスクかは meta.json の `work` だけが持つ。タスクとページのアーカイブは別々で、どの入口も片方しか書かない
- **プロジェクトの比べは ProjectKey 1 つ**：meta.json の空と tasks.json の `/` の対応は `store.ProjectKey` だけ。CLI と画面は `GET /api/artifacts` とページの編集の答えの `taskProject` を使い、自分で写さない
- **親子は 1 段だけ**：確かめるのは `EditTask` で、読むときは断らない。画面の親子は `tasksOf` だけが決める。新しい親は `parentNew` と `parentTitle` で子と一緒に 1 回で書く
- **画面のタスクの書き込みは、見た版を送る**：画面は `PATCH /api/tasks` に、タスクがあると思うか（`new`）と、あるタスクなら見た行の `rev` を送る。ページに描いたボタンは送る時の `tasksRead` の行の `rev`、開いて見せているもの（状態の select・モーダル・取り消しのトースト）は見せている行の `rev` を送る。通った書き込みの答えの行は、`writeTask` 1 か所が `tasksRead` に入れる。親を書くときは `parentNew`（作るなら `parentTitle`）も送り、知らない親には送らない。違えば `EditTask` が 409 で断り、画面は読み直す。CLI はどれも送らない
- **書く順**：ページとタスクの両方を変えるときは、tasks.json を先に、異なるタスクごとに 1 回頼み、ページを後に頼む（`floe work` と「タスクを設定」）。ページだけのフラグでは tasks.json を頼まない
