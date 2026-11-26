# A repository's documents

A repository can keep its specifications, design documents and runbooks as data files (YAML), drawn by templates of its own and checked in CI: floe refuses a key left out or misspelt, a diagram that does not fit, and a page that was not drawn again. The templates, the data files and the pages drawn (HTML) are all committed. Agents edit the data files; people read the pages on floe's server on their own machine.

## Layout

```text
.floe/
  templates/api-spec/     page.html, schema.json, example.yaml, …
  parts/.gitkeep          parts of your own, if any
  themes/.gitkeep         themes of your own, if any
docs/specs/
  orders.yaml             the data file, which is edited
  orders.html             its page, which floe render writes
  invoice.yaml
  invoice.html
.gitattributes
```

- floe leaves out of a linked folder the names that start with a dot, so that `.floe/` is not listed beside the pages.
- Keep only floe's data files, named `.yaml`, and their pages in `docs/specs`: the commands below draw every `.yaml` there, and no `.yml`.
- Keep all three directories of `.floe/`, with a `.gitkeep` in an empty one. The commands below point floe at all three, so that what someone has in `~/.config/floe` neither draws these pages nor stops them, and floe refuses a directory that is not there, as git leaves an empty one out.

`.gitattributes` keeps what a page is compared with, and what goes into it as it is, the same on every machine, and folds the pages in GitHub's diffs:

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

## Make the first template

At the repository's root:

```sh
mkdir -p .floe/templates .floe/parts .floe/themes docs/specs
touch .floe/parts/.gitkeep .floe/themes/.gitkeep
env FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes" floe template new api-spec
```

The paths are absolute ones, as floe refuses a relative one. Fill in the three files as [templates.md](agents/templates.md) tells, then write `docs/specs/orders.yaml` with `template: api-spec`. Write `"additionalProperties": false` in `schema.json`: the one floe makes takes any key. A document can name one of floe's templates too (`template: design`).

## Draw and check

Run these at the repository's root. Draw the pages:

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe render docs/specs/*.yaml
)
```

Check them, which is what CI does. It tells the mistakes of a data file too, so that `floe check` need not run beside it:

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml
)
```

- The parentheses keep the settings out of your shell, where they would reach `floe publish` ([Read the pages](#read-the-pages)).
- Put the two in whatever runs your project's commands.
- For documents in folders of `docs/specs`, give `find docs/specs -name '*.yaml' -exec floe render {} +` and `find docs/specs -name '*.yaml' -exec floe render --check {} +` in place of `floe render docs/specs/*.yaml` and `floe render --check docs/specs/*.yaml`. Do not write `\;` for `+`: a failure is lost, and the check passes whatever it finds. With no `.yaml` at all it runs nothing and passes, where the glob fails.
- On Windows, run these in a POSIX shell such as WSL's, with the repository inside WSL. In PowerShell, set the three to paths such as `"$PWD\.floe\templates"` and name the files yourself, as it does not expand `*.yaml` for a program; this is not tried.

floe does not look for a page whose data file is gone. This optional line does, and fails for an `.html` with no `.yaml` beside it, one you wrote by hand too. Join it to the check with `&&`, so that a failure before it is not lost:

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml &&
  find docs/specs -name '*.html' | while read -r f; do [ -f "${f%.html}.yaml" ] || { echo "no YAML for $f"; exit 1; }; done
)
```

## CI

Both draw without floe's server. Install the version the pages were drawn with, the same on every machine. Run the check on the branch pull requests go into too (here `main`): two pull requests that pass apart may fail once both are merged, as when one changes a template and the other adds a document. Here the settings are the job's own: in CI, no server of yours takes them.

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
        run: |
          set -e
          export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
          floe template check
          floe render --check docs/specs/*.yaml
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
      - |
        set -e
        export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
        floe template check
        floe render --check docs/specs/*.yaml
```

## Change a document

1. Edit its `.yaml`, not its `.html`, which the next drawing writes over.
2. Draw the pages ([Draw and check](#draw-and-check)), and commit the `.yaml` and the `.html` together.

- To settle a merge conflict, settle the `.yaml`, draw again and add the `.html` as drawn: it is written over, conflict marks and all.
- To delete a document, delete both files; to rename one, delete the old page too.
- After a change to a template, a part or a theme, commit every page drawn again with it.

## Read the pages

```sh
floe publish --link docs/specs
```

Run it once on each machine. The folder is a row of the management page, its pages in a tree ([links.md](links.md#a-folder)), and an open page reloads when it is drawn again.

Keep the repository's settings out of your shell, and publish no data file of `docs/specs` drawn with the repository's templates: the server draws with the templates it was started with, so it refuses the data file (`unknown template "api-spec"`), and a `floe publish` of other templates, `--link` too. With no server running, such a publish starts one that draws all your pages with the repository's templates.

## Tell the agents

Put this in the repository's AGENTS.md (or CLAUDE.md), with the commands that draw and check in your repository, so that an agent that edits a document knows:

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe guide` first, and commit its .yaml and .html together. Do not edit the .html. Draw with `<command>`, check with `<command>`. Their templates are in .floe/, which some searches leave out.
```

## What to watch

- **Pages nobody changed that fail**: the check says so when floe is of another version than drew them, or when a template, part or theme changed in a pull request merged meanwhile: take in the latest, draw again and commit the pages.
- **floe's version**: another version may draw a page otherwise. Upgrade in a pull request of its own: write the new version in the CI (twice on its line), install it, draw every page and commit them. A floe built from its source between two releases may draw otherwise, and its version has a `+` and a commit after the number (`<version>+<commit>`): draw the pages with a release, whose version has none.
- **AWS, Google Cloud and Azure icons**: floe does not ship them, so each machine and the CI fetch them before drawing, in a step after Install floe ([commands.md](commands.md#floe-icons-fetch-set-zip-or-url)). `floe icons fetch aws` and `floe icons fetch gcp` take the newest set, and an icon changed in it changes the pages: to hold one AWS set, give its zip's URL, which holds its date (`floe icons fetch aws https://…/Icon-package_….zip`), in the CI and on your machine; Google's two zips have one URL whatever they hold, so keep the zips and give both. Azure's zip is downloaded by hand after agreeing to its terms and given to `floe icons fetch azure`. Lucide's icons are built in.
- **Many pages**: one `floe render` may hold 512 MiB of memory, its pages together ([commands.md](commands.md#floe-render-datayaml)). Past it, run it on the folders of `docs/specs` one by one.
