# floe

[日本語版 README](docs/README.ja.md)

A local page server for agents, shipped as a single Go binary. Agents write a
page as data (YAML) and floe draws it with a template, or link an HTML file to
show it as it is; floe serves them at `http://127.0.0.1:7325` with live reload
and a management UI.
Diagrams are drawn as inline SVG from a declarative spec and checked for
overflow, overlaps and crossing edges before they render.

> **Status:** under construction. Publishing pages from data files with the
> built-in `report` template, linking HTML files and drawing diagrams
> (`floe diagram render`, with AWS icons from `floe icons fetch aws`) and
> checking data files (`floe check`) and templates of your own work.

## Install

linux and macOS (amd64 / arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

Set `FLOE_VERSION` to pin a release and `FLOE_INSTALL_DIR` to change the
destination (default `~/.local/bin`). The binary is verified against the
release's `SHA256SUMS`.

## Quick start

```sh
floe publish plan.yaml     # draw the page from its data, store it, print its URL and open it
floe publish --link out/index.html   # show an HTML file as it is, read on every request
floe check plan.yaml       # check a data file without publishing it
floe list                  # pages, newest first
floe stop                  # stop the background server
```

The server starts on the first publish and keeps running until `floe stop`.
Open `http://127.0.0.1:7325` for the management page.

## i18n

Messages are in English or Japanese. The command line follows `FLOE_LANG`,
then `LC_ALL`, `LC_MESSAGES` and `LANG`; the management page follows the
browser's language. English is the default.

## Documentation

| Topic | English | 日本語 |
|---|---|---|
| Commands and environment variables | [commands.md](docs/commands.md) | [commands.ja.md](docs/commands.ja.md) |
| Writing templates | [templates.md](docs/templates.md) | [templates.ja.md](docs/templates.ja.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## License

[MIT](LICENSE)
