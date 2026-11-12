---
paths:
  - "docs/agents/*.md"
  - "docs/docs.go"
---

# floe が出す docs

- **CLI が出す docs は `docs/agents/` にだけ置き、英語だけにする**：日本語版（`.ja.md`）は作らない（CLI は英語だけで答えるので、日本語版は誰にも出ない）。`docs/agents/` の md は全部 `docs/docs.go` が埋め込む。`TestAgentDocs`
- **floe が出す docs は、ほかの docs へリンクしない**：`floe guide`・`floe diagram|part|template|theme guide` は docs の md をそのまま出すので、リンクは自分の見出しへだけにし、ほかの docs はそれを出すコマンドと節の名前で書く。`TestGuideDocLinks`
- **`docs/agents/` の md は、エージェントが読むリファレンスとして書く**：変数・ファイル・コマンドは、名前・置く所・何が変わるか・既定の値の表にする。`MUST` / `MUST NOT` は floe が断るものにだけ使い、壊れる仕組みは行動を先に、理由をかっこで書く。floe のテーマや部品の好みの決まり、一般知識、例え話は書かない。組み込みの参照は本当に要る所だけにする。
