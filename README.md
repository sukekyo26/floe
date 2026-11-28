# floe

[日本語版 README](docs/README.ja.md)

floe makes documents out of YAML data that AI agents write: a template draws each as HTML, and floe refuses data that its schema does not allow. Keep the templates, the data and the pages of a project in its repository and check them in CI ([docs/repository.md](docs/repository.md)), and read them on floe's local server, which also shows the pages an agent publishes, at `http://127.0.0.1:7325`.

> **Early release.** floe is still changing a lot: commands, data files and templates may change between versions without a migration path. You are welcome to try it, but it is not ready for wide use yet.

## Install

linux and macOS (amd64 / arm64):

```sh
curl -fsSL https://floe-3cf.pages.dev/install.sh | sh
```

Windows 11 (amd64 / arm64), in PowerShell:

```powershell
irm https://floe-3cf.pages.dev/install.ps1 | iex
```

It puts `floe.exe` in `%USERPROFILE%\.local\bin`; [docs/commands.md](docs/commands.md#windows) tells what differs on Windows.

## Use

```sh
floe skill install <agent>   # floe skill install --help lists the agents
```

For an agent not listed, put this line in its instructions (AGENTS.md or the like):

```markdown
To show the user a page, or to write or change a document that is a YAML file with `template` and `lang` at its top, use floe: run `floe guide` first and follow it. It is the guide of the floe installed, so read it each time rather than from memory.
```

`floe --help` lists the commands; [docs/commands.md](docs/commands.md) covers commands and settings.

## License

[MIT](LICENSE). A floe binary holds the Go standard library and Go modules of others under their own licenses: `floe licenses` prints floe's license and theirs, kept in [internal/licenses/NOTICES.txt](internal/licenses/NOTICES.txt). The [Lucide](https://lucide.dev) icons floe ships for diagrams (`lucide/…`) are under the ISC License, some of them under the MIT License too: [internal/icons/lucide/LICENSE](internal/icons/lucide/LICENSE).
