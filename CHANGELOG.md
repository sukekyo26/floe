# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add the `floe` command with `floe version` (also `floe --version`), which
  prints the installed version.
- Add prebuilt binaries for linux and macOS (amd64 / arm64) on GitHub Releases
  and `install.sh`, which installs the latest release (or `FLOE_VERSION`) into
  `FLOE_INSTALL_DIR` (default `~/.local/bin`) after verifying its SHA-256.
