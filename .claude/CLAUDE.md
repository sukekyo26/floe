# CLAUDE.md

## Repository Context

floe はエージェントが書いた HTML ページをローカルサーバー（`http://127.0.0.1:4317`）で表示・管理する Go の単一バイナリです。dotfiles（[azarashi](https://github.com/sukekyo26/azarashi)）の `profiles/bedrock/.agents/skills/artifact` にある JS 実装を移植し、データ（YAML）とテンプレートから文書を組み立てる機能と、検査付きの inline SVG 図を足します。

## Key Paths

- `cmd/floe/`: エントリポイント。`internal/cli.Run` を呼ぶだけ。
- `internal/cli/`: cobra のコマンド定義。出力は注入された `io.Writer` に書く。
- `internal/version/`: リリース版。`VERSION` と同時に上げる。

## Commands

- `just setup`: `just ci` が使うツール（golangci-lint・govulncheck・shfmt）を CI と同じ版で入れる。
- `just build`: `bin/floe` をビルドする。
- `just ci`: push 前の総合チェック（CI と同じ内容）。コード変更後はこれをグリーンにしてからコミットする。

## Project-Specific Rules

- 対応 OS は linux と darwin（amd64・arm64）。Windows はリンクのパス検査などの信頼境界を検証できないので対応しない。
- 版は `VERSION` と `internal/version/version.go` を lockstep で更新する。リリースは `develop` → `main` の PR で `VERSION` が変わったときに `release.yml` が行う。
- カバレッジは `internal/` 全体で 90% 以上（`just cover-check`）。
