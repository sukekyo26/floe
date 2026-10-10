# CLAUDE.md

floe はエージェントが書いた HTML ページをローカルサーバー（`http://127.0.0.1:4317`）で表示・管理する Go の単一バイナリです。dotfiles（[azarashi](https://github.com/sukekyo26/azarashi)）の `profiles/bedrock/.agents/skills/artifact` にある JS 実装を移植し、データ（YAML）とテンプレートから文書を組み立てる機能と、検査付きの inline SVG 図を足します。

- 検証は `just ci`（CI と同じ内容）。golangci-lint は `just setup` で入る。
- 対応 OS は linux と darwin（amd64・arm64）。Windows はリンクのパス検査などの信頼境界を検証できないので対応しない。
