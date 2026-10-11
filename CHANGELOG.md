# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add `floe publish <file>`, which stores an HTML page and prints its URL on
  `http://127.0.0.1:7325`. It starts the server in the background when needed,
  opens a new page in the browser and wraps body-only HTML in the built-in
  template. `--link` registers the file in place, served with the files under
  its directory. `--slug`, `--title`, `--description` and `--force` are also
  available. Republishing a slug says so on stderr, with a warning when the
  title changed.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, as well as the
  management page with live reload, which reads and writes the same store
  format as the artifact skill. They are configured with `FLOE_DIR`,
  `FLOE_PORT` (default 7325) and `FLOE_IDLE_MINUTES` (default 30).
- Add the `floe` command with `floe version` (also `floe --version`), which
  prints the installed version.
- Add prebuilt binaries for linux and macOS (amd64 / arm64) on GitHub Releases
  and `install.sh`, which installs the latest release (or `FLOE_VERSION`) into
  `FLOE_INSTALL_DIR` (default `~/.local/bin`) after verifying its SHA-256.
