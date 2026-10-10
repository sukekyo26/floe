# Changelog

floe の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- `floe` コマンドと、インストール済みの版を表示する `floe version`
  （`floe --version` でも可）を追加。
- linux と macOS（amd64 / arm64）向けのビルド済みバイナリを GitHub Releases で配布し、
  最新のリリース（または `FLOE_VERSION`）を SHA-256 を検証してから
  `FLOE_INSTALL_DIR`（既定は `~/.local/bin`）に入れる `install.sh` を追加。
