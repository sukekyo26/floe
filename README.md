# floe

A local page server for agent-written HTML, shipped as a single Go binary.
Agents publish a page (or structured data rendered through a template), and
floe serves it at `http://127.0.0.1:4317` with live reload and a management UI.
Diagrams are drawn as inline SVG from a declarative spec and checked for
overflow, overlaps and crossing edges before they render.

> **Status:** under construction; only `floe version` exists so far.

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
