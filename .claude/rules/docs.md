---
paths:
  - "docs/agents/*.md"
  - "docs/docs.go"
---

# floe が出す docs

- **CLI が出す docs は `docs/agents/` にだけ置き、英語だけにする**：日本語版（`.ja.md`）は作らない（CLI は英語だけで答えるので、日本語版は誰にも出ない）。`docs/agents/` の md は全部 `docs/docs.go` が埋め込む。`TestAgentDocs`
- **floe が出す docs は、ほかの docs へリンクしない**：`floe guide`・`floe diagram|part|template|theme guide` は docs の md をそのまま出すので、リンクは自分の見出しへだけにし、ほかの docs はそれを出すコマンドと節の名前で書く。`TestGuideDocLinks`
