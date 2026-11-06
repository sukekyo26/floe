---
paths:
  - "internal/store/**"
  - "internal/server/**"
  - "internal/cli/**"
  - "internal/ui/**"
---

# タスク

- **タスクは tasks.json 1 つ**：タスクの状態・段・URL は tasks.json に持ち、書くのは store の lock の中だけ。状態の値は `store.States` 1 つから作る。ページがどのタスクかは meta.json の `work` だけが持つ
- **プロジェクトの比べは ProjectKey 1 つ**：meta.json の空と tasks.json の `/` の対応は `store.ProjectKey` だけ。CLI と画面は `GET /api/artifacts` の `taskProject` を使い、自分で写さない
- **親子は 1 段だけ**：確かめるのは `EditTask` で、読むときは断らない。画面の親子は `tasksOf` だけが決める
- **画面のタスクの書き込みは `new` を言う**：画面から `PATCH /api/tasks` を送るときは、あると思えば `new: false`、無いと思えば `new: true` を送る。「ある」はタスク一覧に出ること（行か、アーカイブも含むページ）。違えば store が `store.Conflict`（409）で断り、画面は tasks を読み直す。CLI は送らない
- **書く順**：ページとタスクの両方を変えるときは、tasks.json を先に、異なるタスクごとに 1 回頼み、ページを後に頼む（`floe work` と「タスクを設定」）。ページだけのフラグでは tasks.json を頼まない
