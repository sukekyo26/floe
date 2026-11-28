# Documents in a repository

A repository with `.floe/repository.yaml` at its root keeps documents as data files and their pages, both committed (`docs/specs/orders.yaml`, `docs/specs/orders.html`). `floe repository render` draws the pages and `floe repository check` checks them, without the server; CI runs the check.

- Put `floe repository` before every `template`, `part` and `theme` command in such a repository: without it, floe reads and writes your own (`~/.config/floe`).
- The commands run anywhere in the repository and name the documents from its root; a data file you give them is a path from where you are.

## Change a document

1. Find `.floe/repository.yaml` at the repository's root (some searches leave out names that start with a dot).
   - None, but pages committed beside their data files: use the commands the repository's AGENTS.md, CLAUDE.md, README, CI or scripts name.
   - Neither: the document is not kept in a repository; follow *Writing a page* of `floe guide`, or *Set up a repository* when the user asks for one.
2. Run `floe repository check`. When it fails for anything you were not asked to mend, change nothing and tell the user (*When the check fails*).
3. Read the template: `floe repository template show <template> schema.json`, and `example.yaml` for a new document; `floe repository part list`, `floe repository part show <part> schema.json` and `floe repository theme list` tell what it may take.
4. Edit the `.yaml` as `floe guide` says (*The data file*, *YAML that floe reads as you meant*). Do not edit the `.html`.
5. Run `floe repository render` until it tells no mistake (*Reading mistakes* of `floe guide`).
6. Run `floe repository check`, and commit each `.yaml` with its page. When pages you did not change came out different and you changed no template, part or theme, do not commit them, and tell the user (*When the check fails*).

- To change a template's keys, change its `schema.json`, `page.html` and `example.yaml` (`floe repository template guide`) and every data file that names it, draw, and commit it all in one change.
- In a merge conflict, settle the `.yaml` and draw again; do not mend the page by hand.
- To delete or rename a document, delete its old page too: the check does not find it.
- A page is drawn in the theme its data file names with `theme:`, else `default`: the theme picked on the management page does not reach it, and a template has none. When the user names a theme, write `theme:` in each data file.

## Show the documents

`floe publish --link docs/specs` lists the pages. Do not publish a data file that names a template of the repository's: the server has the user's templates, not the repository's (`unknown template "api-spec"`).

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

```yaml
version: '0.5.0'
documents:
  - 'docs/specs'
```

| Key | What it is | How to write it |
|---|---|---|
| `version` | The version of floe that draws the pages: another floe refuses the repository before it reads anything else | What `floe --version` prints before any `+`, as a string. A floe built from its source (`0.5.0+<commit>`) is taken, and may draw a page otherwise: draw the pages with a release |
| `documents` | The directories whose `.yaml` and `.yml`, in folders too, are the documents | One or more, from the root, `/`-separated, without `./`, a last `/` or `..` (`docs/specs`). `floe repository new` does not make them. Not read: a name that starts with a dot, or a link to a directory, and what is under them (name that directory here instead) |

Both are required, and any other key is refused.

### Steps

Run these at the repository's root, where `.git` is: `floe repository new` makes the repository where it runs.

1. Make the setting, the directory of the documents and a template (with floe's templates alone, as `template: tech-spec`, leave out `template new`):

```sh
floe repository new docs/specs
mkdir -p docs/specs
floe repository template new api-spec
```

2. Fill in the template as `floe repository template guide` says until `floe repository template check` tells no mistake, write the documents, and draw them:

```sh
floe repository render
```

3. Check them:

```sh
floe repository check
```

4. Make CI run the check on every pull request and on every push to the branch they merge into.
   - Install floe with the command the repository's CI or README has. With none, use `curl -fsSL https://floe-3cf.pages.dev/install.sh | FLOE_VERSION=<version> sh`.
   - `<version>` is the one `.floe/repository.yaml` names, with no `v`.
   - The installer puts floe in `$HOME/.local/bin`: add it to the `PATH` of the later steps, or set `FLOE_INSTALL_DIR` to a directory on it.
   - Fetch the icons of a cloud before the check (*Icons*).
5. Add to `.gitattributes`:

```text
/docs/specs/**/*.html text eol=lf linguist-generated
/docs/specs/**/*.svg  text eol=lf
/.floe/**             text=auto eol=lf
```

   For an HTML file written by hand in `docs/specs`, add a line after these: `/docs/specs/notes.html -linguist-generated`.

6. Add to the repository's AGENTS.md or CLAUDE.md:

```markdown
The documents in docs/specs are drawn by floe. To change one, run `floe repository guide` first, and commit its data file (.yaml or .yml) and its .html together. Do not edit the .html. Draw with `floe repository render`, check with `floe repository check`. Their templates are in .floe/, which some searches leave out.
```

### What floe repository reads

| Reads | Does not read |
|---|---|
| The templates, parts and themes in `.floe/`; one of these directories not there is none | `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR`, `FLOE_THEMES_DIR`, the store, the server |
| The data files under `documents` | An HTML file with no data file, as one written by hand: it is left alone |
| The icon sets of `floe icons` (*Icons*) | Anything else out of the repository: a link out of it is refused |

### Icons

- A page drawn with AWS, Google Cloud or Azure icons needs them fetched in CI too, before the check. `floe icons fetch aws` and `floe icons fetch gcp` take the newest set, which may change the pages: give `aws` its zip's URL, which holds its date, the same in CI and on each machine; keep the zips of `gcp`, whose URLs hold none, and give both zips. `floe icons fetch azure` takes the zip the user downloads after agreeing to its terms.

## When the check fails

The check tells every mistake of the templates, parts, themes and documents, and a directory of `documents` that is not there in place of the documents (make it, and run the check again). Once there is no mistake, it tells each page missing or not drawn again: fix, and run it again. `floe repository render` writes no page while a data file has a mistake.

- For a document you changed, added or took away: draw again, and delete the page of one taken away.
- For one you did not touch: tell the user what the check printed, with `floe --version` and the version `.floe/repository.yaml` names, and do not guess why. It may be a floe of another version than drew the pages, or a template, part, theme or icon set changed in a pull request merged meanwhile (when the user says so, take in the latest and draw again).

To upgrade floe, write the new version in `.floe/repository.yaml` and in CI, install it, run `floe repository render` and commit the pages, in one change.
