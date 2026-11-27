# Documents in a repository

A repository may keep documents as data files with the pages drawn of them, both committed (`docs/specs/orders.yaml` and `docs/specs/orders.html`), often drawn with templates of its own. Such a repository has `.floe/repository.yaml` at its root. `floe repository render` draws its pages and `floe repository check` checks them, without the server, and CI runs the check; neither the data files nor the pages are published. This doc says how to change one, how to set a repository up to keep them, and what makes the check fail.

## Change a document

1. Find `.floe/repository.yaml` at the repository's root (some searches leave out names that start with a dot: list `.floe` by name).
   - When there is none but pages are committed beside their data files, the repository draws them its own way: read AGENTS.md, CLAUDE.md, README, the CI and the scripts (search for `floe` in any case), and use the commands they name. When it has neither, the document is not kept in the repository: follow *Writing a page* of `floe guide`.
2. In a repository that has `.floe/repository.yaml` (to set one up, see *Set up a repository* instead), run `floe repository check` before you edit, and before you write a new document too. When it fails for anything you were not asked to mend, change nothing and tell the user (*When the check fails*).
3. Read the template the document names: `floe repository template show <template> schema.json`, and its `example.yaml` for a new document. `floe repository part list`, `floe repository part show <part> schema.json` and `floe repository theme list` tell the parts and themes it may take. Put `floe repository` before every command of `template`, `part` and `theme` in this repository: without it, floe reads and writes your own (`~/.config/floe`), not the repository's.
4. Edit the `.yaml` as `floe guide` says (*The data file*, *YAML that floe reads as you meant*). Do not edit the `.html` (drawing writes over it).
5. Run `floe repository render`, fix every mistake (*Reading mistakes* of `floe guide`), and run it again until none is left.
6. Run `floe repository check`. When pages of documents you did not change came out different, and you changed no template, part or theme, do not commit those pages, and tell the user (*When the check fails*). Else, when you commit, commit the `.yaml` and its page together (the check fails for a page that its `.yaml` does not draw).

- Run the commands anywhere in the repository: they find its root, and name the documents they find from it. A data file you give them is a path from where you are, as with `floe render` (`floe repository render specs/orders.yaml` in `docs`).
- To change a template of the repository's (a new required key, say), change its `schema.json`, `page.html` and `example.yaml` as `floe repository template guide` says, then every data file that names it (the check tells those left), draw, and commit it all in one change.
- In a merge conflict, settle the `.yaml` and draw again, which writes the page whole.
- To delete a document, delete its `.yaml` and its page; to rename one, delete the old page too (*What the check does not find*).

## Show the documents

Run `floe publish --link docs/specs`: it lists the pages of the folder, as drawn. Do not publish a data file that names a template of the repository's (`unknown template "api-spec"`): the server draws with the user's templates, not the repository's.

A page of a repository is drawn in the theme its data file names with `theme:`, else floe's `default`: the theme picked on the management page does not reach it, and a template has no theme of its own. When the user names a theme for the repository's documents, write `theme:` in each data file.

## Set up a repository

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

### .floe/repository.yaml

`floe repository new` writes it:

```yaml
version: '0.4.0'
documents:
  - 'docs/specs'
```

| Key | Required | What it is | How to write it |
|---|---|---|---|
| `version` | yes | The version of floe that draws the pages; another floe refuses the repository (*floe's version*) | The number `floe --version` prints before any `+`, as a string |
| `documents` | yes | The directories of the documents, every data file under them drawn | A list of one or more directories that are there, from the root, `/`-separated, without `./` before, `/` after or `..` (`docs/specs`) |

Any other key is refused.

### Steps

Run these at the repository's root, the directory that holds `.git`: `floe repository new` makes the repository where it runs.

1. Make the setting, naming the directories of the documents from the root (`docs/specs`, without `./` before or `/` after), and the first template. `floe repository new` makes no directory of `documents`: make each, as `mkdir` does below, or the check fails with `docs/specs: no such file or directory`. With floe's templates alone (`template: design`), leave out `template new`.

```sh
floe repository new docs/specs
mkdir -p docs/specs
floe repository template new api-spec
```

2. Fill in the template as `floe repository template guide` says, with `floe repository` before each command it names, until `floe repository template check` tells no mistake. Write the documents, then draw them (until they are drawn, the check tells each page `is missing`):

```sh
floe repository render
```

3. Check them:

```sh
floe repository check
```

4. Make CI run the check on every pull request and on every push to the branch that pull requests merge into (two pull requests that each pass may fail once both are merged).
   - Find the command that installs floe in the repository's CI or README. If neither has it, ask the user for it before you write the install step, and do not make up its URL.
   - Install the version `.floe/repository.yaml` names: set `FLOE_VERSION=<version>` for the `sh` that runs the installer, with no `v`.
   - Then run `floe repository check`. Where a page has icons of a cloud, fetch them before the check (*Icons*).
5. Add to `.gitattributes` (the check compares bytes):

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

   For an HTML file written by hand in `docs/specs`, add a line after these that names it with `-linguist-generated` (`/docs/specs/notes.html -linguist-generated`), so that its diffs are shown.

6. Add to the repository's AGENTS.md or CLAUDE.md, so that whoever edits a document knows:

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe repository guide` first, and commit its data file (.yaml or .yml) and its .html together. Do not edit the .html. Draw with `floe repository render`, check with `floe repository check`. Their templates are in .floe/, which some searches leave out.
```

### What floe repository reads

- The templates, parts and themes in `.floe/` alone. A directory of them that is not there is none (no `.gitkeep` is needed). `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR` and `FLOE_THEMES_DIR` are not read: do not set them for this repository.
- Every `.yaml` and `.yml` under the directories of `documents`, in folders too, but not under a name that starts with a dot, nor under a link to a directory: name such a directory in `documents` instead. An HTML file with no data file is left alone, a page written by hand too.
- Nothing out of the repository but the icon sets of `floe icons` (*Icons*): a page, a data file or a directory of `.floe/` that is a link out of it is refused.

### What the check does not find

- A page whose data file was deleted or renamed: delete it with its data file.
- An HTML file with no data file beside it, one written by hand too: the check leaves it alone.
- A data file under a name that starts with a dot, or under a link to a directory: name such a directory in `documents`.
- A page that a floe built from its source draws otherwise than the release of the same version (*floe's version*).

### floe's version

- `version` in `.floe/repository.yaml` is the version that draws the pages; floe refuses the repository when it is of another. A floe built from its source has a `+` and a commit after its version (`<version>+<commit>`), and the part before the `+` is compared: such a floe may still draw a page otherwise. Draw the pages with a release.

### Icons

- A page drawn with AWS, Google Cloud or Azure icons needs them fetched in CI too, before the check. `floe icons fetch aws` and `floe icons fetch gcp` take the newest set, which may change the pages: give `aws` its zip's URL, which holds its date, the same in CI and on each machine; keep the zips of `gcp`, whose URLs hold none, and give both zips. `floe icons fetch azure` takes the zip the user downloads after agreeing to its terms.

## When the check fails

The check names each mistake of a template, part, theme or document. Once the documents have none, it names each page that its data file does not draw or that is missing: after you fix the mistakes, run the check again. A directory of `documents` that is not there is told before the documents are checked: make it, and run the check again. `floe repository render` writes no page while any data file has a mistake.

- For a document you changed, added or took away: draw again, and delete the page of one taken away or renamed.
- For one you did not touch, tell the user what the check printed, with the version of `floe --version` and the one `.floe/repository.yaml` names. The reasons may be:
  - floe is of another version than drew them (`floe repository` refuses another version before it checks; a `+` after the version is a floe built from its source, not a release).
  - A template, part, theme or icon set changed in a pull request merged meanwhile (once the user says it is this, take in the latest and draw again).

  Do not guess which.

To upgrade floe, write the new version in `.floe/repository.yaml` and in CI, install it, run `floe repository render` and commit the pages, in one change.
