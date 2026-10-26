# floe

[日本語版 README](docs/README.ja.md)

A local page server for agents, shipped as a single Go binary. Agents write a
page as data (YAML) and floe draws it with a template, or link an HTML file to
show it as it is; floe serves them at `http://127.0.0.1:7325`.

> **Status:** being rebuilt. So far `floe check` and `floe render` draw data files
> with floe's parts, themes and report, and with your own under `FLOE_PARTS_DIR`,
> `FLOE_THEMES_DIR` and `FLOE_TEMPLATES_DIR` (else `~/.config/floe/parts`, `themes`
> and `templates`), which `floe part check` and `floe template check` check; `floe
> version` prints the version.

## Install

linux and macOS (amd64 / arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

Set `FLOE_VERSION` to pin a release and `FLOE_INSTALL_DIR` to change the
destination (default `~/.local/bin`). The binary is verified against the
release's `SHA256SUMS`.

## License

[MIT](LICENSE)
