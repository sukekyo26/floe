# A repository's documents

A repository can keep its specifications, design documents and runbooks as data files (YAML), drawn by templates of its own and checked in CI: floe refuses a key left out or misspelt, a diagram that does not fit, and a page that was not drawn again. The templates, the data files and the pages drawn (HTML) are all committed. Agents edit the data files; people read the pages on floe's server on their own machine.

## Layout

```text
.floe/
  repository.yaml         the setting: floe's version and where the documents are
  templates/api-spec/     page.html, schema.json, example.yaml, …
  parts/                  parts of your own, if any
  themes/                 themes of your own, if any
docs/specs/
  orders.yaml             the data file, which is edited
  orders.html             its page, which floe repository render writes
  invoice.yaml
  invoice.html
.gitattributes
```

`.floe/repository.yaml`, which `floe repository new` writes:

```yaml
version: '0.4.0'
documents:
  - 'docs/specs'
```

- `floe repository` finds the repository by this file, from any directory in it, looking up no higher than the directory that holds `.git`. Its commands read the templates, parts and themes in `.floe/` alone, whatever is in `~/.config/floe` or the `FLOE_*_DIR` settings, and they use no server and no store. A directory of `.floe/` that is not there is none: no `.gitkeep` is needed.
- `version` is the version of floe that draws the pages. Another floe refuses the repository before it reads anything else, telling both versions. A floe built from its source has a `+` and a commit after its version (`0.4.0+67f6e156bc78`), and the part before the `+` is compared: such a floe between two releases may draw a page otherwise, which the check then tells as a page not drawn again.
- `documents` are the directories of the documents, from the root, `/`-separated, without `./` before or `/` after (`docs/specs`). Every `.yaml` and `.yml` under them is a document, in folders too, but not under a name that starts with a dot, nor under a link to a directory (name that directory in `documents` instead). An HTML file with no data file is left alone, so a page written by hand can be kept there; so is a page whose data file was deleted, which the check does not find.
- floe reads and writes nothing out of the repository: a page, a data file or a directory of `.floe/` that is a link out of it is refused, and so is a file in a directory of `.floe/` that is a link out of that directory.
- floe leaves out of a linked folder the names that start with a dot, so that `.floe/` is not listed beside the pages.
- A page is drawn in the theme its data file names with `theme:`, else `default`: the theme picked on the management page does not reach a repository's pages, and a template has no theme of its own.

`.gitattributes` keeps what a page is compared with, and what goes into it as it is, the same on every machine, and folds the pages in GitHub's diffs:

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

For an HTML file written by hand in `docs/specs`, add a line after these that names it with `-linguist-generated` (`/docs/specs/notes.html -linguist-generated`).

## Make the first template

At the repository's root:

```sh
floe repository new docs/specs
floe repository template new api-spec
```

`floe repository new` makes no directory of `documents`: make `docs/specs` too. Fill in the three files as [templates.md](agents/templates.md) tells, with `floe repository` before each command it names (`floe repository template check`), then write `docs/specs/orders.yaml` with `template: api-spec`. Write `"additionalProperties": false` in `schema.json`: the one floe makes takes any key. A document can name one of floe's templates too (`template: design`).

## Draw and check

Draw the pages:

```sh
floe repository render
```

Check them, which is what CI does. It checks the repository's templates, parts and themes, drawing each template's `example.yaml`, then every document, and tells every mistake; once the documents have none, it tells each page missing or not drawn again:

```sh
floe repository check
```

- Both run from any directory of the repository, and name the documents from its root. Give data files to either to draw or check those alone (`floe repository render docs/specs/orders.yaml`).
- Put the two in whatever runs your project's commands.

## CI

Both draw without floe's server. Install the version the pages were drawn with, the one `.floe/repository.yaml` names. Run the check on the branch pull requests go into too (here `main`): two pull requests that pass apart may fail once both are merged, as when one changes a template and the other adds a document.

GitHub Actions, `.github/workflows/specs.yml`:

```yaml
name: specs
on:
  pull_request:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  specs:
    runs-on: ubuntu-latest
    steps:
      - uses: 'actions/checkout@v7'
      - name: 'Install floe'
        run: |
          curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.4.0/install.sh | FLOE_VERSION=0.4.0 sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - name: 'Check the documents'
        run: 'floe repository check'
```

AWS CodeBuild, `buildspec.yml`:

```yaml
version: 0.2
phases:
  install:
    commands:
      - 'curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/v0.4.0/install.sh | FLOE_VERSION=0.4.0 FLOE_INSTALL_DIR=/usr/local/bin sh'
  build:
    commands:
      - 'floe repository check'
```

## Change a document

1. Edit its `.yaml`, not its `.html`, which the next drawing writes over.
2. Draw the pages (`floe repository render`), and commit the `.yaml` and the `.html` together.

- To settle a merge conflict, settle the `.yaml`, draw again and add the `.html` as drawn: it is written over, conflict marks and all.
- To delete a document, delete both files; to rename one, delete the old page too.
- After a change to a template, a part or a theme, commit every page drawn again with it.

## Read the pages

```sh
floe publish --link docs/specs
```

Run it once on each machine. The folder is a row of the management page, its pages in a tree ([links.md](links.md#a-folder)), and an open page reloads when it is drawn again.

Publish no data file of `docs/specs` drawn with the repository's templates: the server draws with your templates, not the repository's, so it refuses the data file (`unknown template "api-spec"`). `floe publish` does not read `.floe/repository.yaml`.

## Tell the agents

Put this in the repository's AGENTS.md (or CLAUDE.md), so that an agent that edits a document knows:

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe repository guide` first, and commit its data file (.yaml or .yml) and its .html together. Do not edit the .html. Draw with `floe repository render`, check with `floe repository check`. Their templates are in .floe/, which some searches leave out.
```

## What to watch

- **Pages nobody changed that fail**: the check says so when floe is of another version than drew them, or when a template, part or theme changed in a pull request merged meanwhile: take in the latest, draw again and commit the pages.
- **floe's version**: another version may draw a page otherwise, so `floe repository` refuses one other than `.floe/repository.yaml` names. Upgrade in a pull request of its own: write the new version in `.floe/repository.yaml` and in the CI (twice on its line), install it, run `floe repository render` and commit the pages. A floe built from its source between two releases may draw otherwise though the part of its version before the `+` is the same: draw the pages with a release, whose version has no `+`.
- **AWS, Google Cloud and Azure icons**: floe does not ship them, so each machine and the CI fetch them before drawing, in a step after Install floe ([commands.md](commands.md#floe-icons-fetch-set-zip-or-url)). `floe icons fetch aws` and `floe icons fetch gcp` take the newest set, and an icon changed in it changes the pages: to hold one AWS set, give its zip's URL, which holds its date (`floe icons fetch aws https://…/Icon-package_….zip`), in the CI and on your machine; Google's two zips have one URL whatever they hold, so keep the zips and give both. Azure's zip is downloaded by hand after agreeing to its terms and given to `floe icons fetch azure`. Lucide's icons are built in.
- **Many pages**: one `floe repository render` may hold 512 MiB of memory, its pages together ([commands.md](commands.md#floe-render-datayaml)). Past it, give it the data files of the folders of `docs/specs` one folder at a time.
