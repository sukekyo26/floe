# floe

[日本語版 README](docs/README.ja.md)

A local page server for AI agents. An agent writes a page as YAML data, and floe draws it with a template and shows it at `http://127.0.0.1:7325`.

> **Early release.** floe is still changing a lot: commands, data files and templates may change between versions without a migration path. You are welcome to try it, but it is not ready for wide use yet.

## Install

linux and macOS (amd64 / arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

## Use

```sh
floe skill install <agent>   # floe skill install --help lists the agents
```

For an agent not listed, put this line in its instructions (AGENTS.md or the like):

```markdown
To show the user a page, use floe: run `floe guide` first and follow it. It is the guide of the floe installed, so read it each time rather than from memory.
```

`floe --help` lists the commands. More in [docs](docs/).

## License

[MIT](LICENSE). A floe binary holds the Go standard library and Go modules of others under their own licenses: `floe licenses` prints floe's license and theirs, kept in [internal/licenses/NOTICES.txt](internal/licenses/NOTICES.txt). The [Lucide](https://lucide.dev) icons floe ships for diagrams (`lucide/…`) are under the ISC License, some of them under the MIT License too: [internal/icons/lucide/LICENSE](internal/icons/lucide/LICENSE).
