---
paths:
  - "internal/store/**"
  - "internal/server/**"
  - "internal/cli/**"
  - "internal/ui/**"
---

# 開発の画面のタスク

- **タスクは work.json 1 つ**：タスクの状態・段・URL は work.json に持ち、書くのは store の lock の中だけ。状態の値は `store.States` 1 つから作る。ページがどのタスクかは meta.json の `work` だけが持つ
- **プロジェクトの比べは ProjectKey 1 つ**：meta.json の空と work.json の `/` の対応は `store.ProjectKey` だけ。CLI と画面は `GET /api/artifacts` の `taskProject` を使い、自分で写さない
- **書く順**：ページとタスクの両方を変えるときは、work.json を先に、異なるタスクごとに 1 回頼み、ページを後に頼む（`floe work` と「タスクを設定」）。ページだけのフラグでは work.json を頼まない
