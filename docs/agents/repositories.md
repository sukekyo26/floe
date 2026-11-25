# Documents in a repository

A repository may keep documents as data files with the pages drawn of them, both committed (`docs/specs/orders.yaml` and `docs/specs/orders.html`), often drawn with templates of its own. `floe render` draws the pages, without the server, and CI checks them; neither the data files nor the pages are published. This doc says how to change one, how to set a repository up to keep them, and what makes the check fail.

## Change a document

1. Find the command that draws the documents and the one that checks them. Read AGENTS.md, CLAUDE.md, README, the CI and the scripts (search for `floe` in any case, and in names that start with a dot: some tools leave out `.floe/` and `.github/`). Below, these two are the draw command and the check command.
   - When the repository has no such command but pages are committed beside its data files, draw with `floe render` and check with `floe render --check` (*Settings*).
   - When it has neither, the document is not kept in the repository: follow *Writing a page* of `floe guide`.
2. Run the check command before you edit, and before you write a new document too. When it fails for anything you were not asked to mend, change nothing and tell the user (*When the check fails*).
3. Read the template the document names, with every setting the repository's commands set put before the command (a template may take the repository's parts, and what is not set is read from the user's home). For a new document, read `example.yaml` too. Run these commands, and those below, at the repository's root, which `$PWD` is. Put the same settings before `floe part list`, `floe part show <part> schema.json` and `floe theme list`. Where the repository has no such directories (*Settings*), run these commands with no settings. Where they are in `.floe/`:

```sh
env FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes" floe template show api-spec schema.json
```

4. Edit the `.yaml` as `floe guide` says (*The data file*, *YAML that floe reads as you meant*). Do not edit the `.html` (drawing writes over it).
5. Run the draw command, fix every mistake (*Reading mistakes* of `floe guide`), and run it again until none is left.
6. Run the check command. When pages of documents you did not change came out different, and you changed no template, part or theme, do not commit those pages, and tell the user (*When the check fails*). Else, when you commit, commit the `.yaml` and its page together (the check fails for a page that its `.yaml` does not draw).

- To change a template of the repository's (a new required key, say), change its `schema.json`, `page.html` and `example.yaml` as `floe template guide` says, then every data file that names it (the check tells those left), draw, and commit it all in one change.
- In a merge conflict, settle the `.yaml` and draw again, which writes the page whole.
- To delete a document, delete its `.yaml` and its page; to rename one, delete the old page too.

## Settings

| Setting | Where to set it | What it changes | Default |
|---|---|---|---|
| `FLOE_TEMPLATES_DIR` | In the repository's commands, or before one command | Where floe reads templates of your own | `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates` |
| `FLOE_PARTS_DIR` | The same | Where floe reads parts, looks and shared schemas of your own | `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts` |
| `FLOE_THEMES_DIR` | The same | Where floe reads themes of your own | `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes` |

- Where the repository has directories of its own (`.floe/`), set all three to them (one not set is read from the user's home, and a mistake in it stops these pages). Where it has none, set none.
- Each MUST be the absolute path of a directory that is there (keep a `.gitkeep` in an empty one, as git leaves it out).
- Do not export them outside a subshell (`( … )`, as below), nor put them before `floe publish`, `--link` too (a server running draws with its own directories and refuses the command; with none running, the publish starts one that draws the user's every page with the repository's). When floe says `the server on port … draws pages with …`, run the command again without the settings you put before it; with none, tell the user, and change none of theirs. Do not run `floe stop`.

## Show the documents

Run `floe publish --link docs/specs`, with none of these settings: it lists the pages of the folder, as drawn. Do not publish a data file that names a template of the repository's (`unknown template "api-spec"`).

## Set up a repository

```text
.floe/templates/api-spec/   page.html, schema.json, example.yaml
.floe/parts/.gitkeep        parts of its own, if any
.floe/themes/.gitkeep       themes of its own, if any
docs/specs/orders.yaml      a data file of template: api-spec
docs/specs/orders.html      its page, which floe render writes
```

Run all of these at the repository's root.

1. Make the directories, `.floe/parts` and `.floe/themes` too while they are empty (step 2 sets all three), and the template:

```sh
mkdir -p .floe/templates .floe/parts .floe/themes docs/specs
touch .floe/parts/.gitkeep .floe/themes/.gitkeep
env FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes" floe template new api-spec
```

2. Put these two, which draw and check, in whatever the project runs its commands with:

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe render docs/specs/*.yaml
)
```

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml
)
```

   For documents in folders of `docs/specs`, write `find docs/specs -name '*.yaml' -exec floe render {} +` for `floe render docs/specs/*.yaml`, and `find docs/specs -name '*.yaml' -exec floe render --check {} +` for `floe render --check docs/specs/*.yaml`. Do not write `\;` for `+` (a failure is lost, and the check passes whatever it finds).
   floe does not look for a page whose data file is gone. To find such a page, end the check with this, joined with `&&` (without `&&`, a failure of the check before the find is lost; the find fails for an `.html` with no `.yaml` beside it, one you wrote by hand too):

```sh
(
  export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
  floe template check &&
  floe render --check docs/specs/*.yaml &&
  find docs/specs -name '*.html' | while read -r f; do [ -f "${f%.html}.yaml" ] || { echo "no YAML for $f"; exit 1; }; done
)
```

3. Fill in the template as `floe template guide` says until the check tells no mistake, write the documents, and draw.
4. Make CI run the check on every pull request and on every push to the branch that pull requests merge into (two pull requests that each pass may fail once both are merged).
   - Find the command that installs floe in the repository's CI or README. If neither has it, ask the user for it before you write the install step, and do not make up its URL.
   - Install the version the pages were drawn with: set `FLOE_VERSION=<version>` for the `sh` that runs the installer, `<version>` the number after `floe` that `floe --version` prints, with no `v`.
   - Check that `floe --version` says that version, then run:

```sh
set -e
export FLOE_TEMPLATES_DIR="$PWD/.floe/templates" FLOE_PARTS_DIR="$PWD/.floe/parts" FLOE_THEMES_DIR="$PWD/.floe/themes"
floe template check
floe render --check docs/specs/*.yaml
```

   Where step 2 has the `find … {} +` form, use it here too, in place of `floe render --check docs/specs/*.yaml`. Under `set -e` the find that looks for pages without a data file stands on a line of its own, last. Where a page has icons of a cloud, fetch them before the check (*Icons*).
5. Add to `.gitattributes` (the check compares bytes):

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

6. Add to the repository's AGENTS.md or CLAUDE.md, so that whoever edits a document knows. In place of `<command>`, write the project's own names for the two of step 2, a script or a task; they are of several lines, so make a script of each first where there is none:

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe guide` first, and commit its .yaml and .html together. Do not edit the .html. Draw with `<command>`, check with `<command>`. Their templates are in .floe/, which some searches leave out.
```

### The folder

- Keep in `docs/specs` only floe's data files, named `.yaml`, and their pages (the commands draw every `.yaml`, and no `.yml`).

### floe's version

- Draw the pages with a release (a floe built from its source between two releases says the earlier one's version).

### Icons

- A page drawn with AWS, Google Cloud or Azure icons needs them fetched in CI too, before the check. `floe icons fetch aws` and `floe icons fetch gcp` take the newest set, which may change the pages: give `aws` its zip's URL, which holds its date, the same in CI and on each machine; keep the zips of `gcp`, whose URLs hold none, and give both zips. `floe icons fetch azure` takes the zip the user downloads after agreeing to its terms.

## When the check fails

The check names each page that its data file does not draw and each page missing (and each page with no data file, with the find of step 2).

- For a document you changed, added or took away: draw again, and delete the page of one taken away or renamed.
- For one you did not touch, tell the user what the check printed, with the version of `floe --version` and the one the repository's CI installs. The reasons may be:
  - floe is of another version than drew them (`floe --version` and the version the CI installs; the version a floe built from its source says is not enough to tell).
  - A template, part, theme or icon set changed in a pull request merged meanwhile (once the user says it is this, take in the latest and draw again).

  Do not guess which.

To upgrade floe, write the new version in CI, install it, draw every page and commit them, in one change.
