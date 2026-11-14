# Using floe

floe draws a page from a YAML data file you write, checks it, and serves it at the URL `floe publish` prints. This guide says how to write, check and publish a page, which part fits which content, and what floe refuses or misreads. `floe <command> --help` and the `list` and `show` commands say the rest.

## Writing a page

1. Pick a template from `floe template list`: the one made for the page's purpose if there is one (`design` for a design document, `handoff`, `runbook`, …), else `report`. Read its example and schema before writing: `floe template show <template> example.yaml` and `floe template show <template> schema.json`. The example shows the file's structure, not a fixed set of parts.
2. Name the file for the page, as `db-choice.yaml` (*The data file*).
3. Choose parts from `floe part list` that the template's schema allows where you put them (*Picking parts*). A part MUST NOT hold a key its schema does not list: look up the keys of each part the example does not show.
4. Run `floe check db-choice.yaml`, fix every mistake, and run it again until none is left (*Reading mistakes*). `warning:` lines are advice: a file with only those is done.
5. Run `floe publish db-choice.yaml` from the repository the page is about, and give the user the URL it prints (*Publishing*). You need not fetch the page.

```yaml
template: report
lang: en
title: 'Moving the team wiki'
lead: 'The wiki moves in two weekends.'
sections:
  - heading: Steps
    parts:
      - text: 'Freeze edits on **Friday evening**, then copy the pages.'
```

## The data file

| Key | Takes | What it does |
|---|---|---|
| `template` | a name `floe template list` prints, required | The template that draws the page |
| `lang` | a BCP 47 tag such as `en` or `ja`, required | The page's language: the words floe draws and the diagrams' text |
| `title` | text | The page's name |
| `theme` | a name `floe theme list` prints | Leave it out (*Themes*) |

- The other top keys are the template's: its `schema.json` lists them (a `report`'s `lead` sums the page up under the title).
- The file's name without `.yaml` becomes the page's slug in its URL, in lower case, with each run of characters other than ASCII letters and digits made a `-`, cut to 64 characters, with no `-` at either end. A name with no ASCII letter or digit (`設計.yaml`) gives no slug: give `floe publish --slug <slug>`.
- A `--slug` MUST be 1 to 64 lower case ASCII letters, digits and `-`, starting and ending with a letter or digit.

## Publishing

- A new page goes in a project, which groups pages on the management page and the task list. floe names the project after the git repository `floe publish` runs in: its `origin`'s name, else the repository's directory name; outside a repository, the directory's name. Where the data file is does not matter (`cd ~/work/app && floe publish ~/notes/db-choice.yaml`).
- When a page is new or its project changes (it had none, it replaced another file's page, or `--move-project` moved it), `floe publish` prints the project on stderr after the URL (`page in project app`). If it is not the name of the repository the page is about, move to that repository and publish again with `--move-project`.
- `--move-project` takes the page off its task, as a task is of its project, and says so (`page in project app, off task #9`); the task stays in the old project without the page. Tell the user, who decides whether to put the page on a task (*Tasks*). The user can also change the project on the management page (the page's Edit, Slug and project).
- floe knows a page by its data file, not by the directory floe runs in: publish the same file again, from any directory and without `--slug`, and it replaces its page, even after the user renamed the page on the management page.
- A slug MUST NOT be that of another file's page (`slug "db-choice" was published from …: choose another slug`): rename your file (`db-choice-2.yaml`) or give another `--slug`, and publish again.

## Tasks

The user may put pages on the tasks of the task list, and group a task's pages by category. Change them only when the user asks; `floe work --help` says how.

- `floe work <slug>` prints the page's task with the task's project, its category and whether it is pinned, and changes nothing. A page on no task prints `page <slug>: no task`, without its project: `floe publish` tells the project.
- `floe work` prints the todo and doing tasks of the working directory's project, the latest change first, a line each. `--project` names another project (`--project ""` for pages of no project).
- Do not give `--category`, or change a page's category, unless the user asks: the user decides where a page stands among its task's pages.

## Themes

- Do not write `theme` unless the user names a theme for the page: the user picks themes on the management page, at the root of the page's URL.
- When you give the user a URL, run `floe theme default`. If it prints a name, say nothing of themes. If it prints nothing, tell the user once in the conversation that the default theme for new pages can be chosen in the management page's settings (the gear in the page list's bar).

## Picking parts

Each item of `parts` is one key naming the part. `floe part list` gives each part's keys and what it is for; for a part whose keys it does not list (`chart`, `stats`, `diagram`, …), `floe part show <part> schema.json` gives its keys, types and details. The template's `schema.json` says which parts each section or field takes.

| Content | Part |
|---|---|
| Prose, or a short list | `text` (Markdown) |
| Steps in order, to do or done | `steps`; floe numbers them, so write no number in a title |
| Items with longer explanations, such as risks, requirements or terms | `entries`; short states or labels go in each item's `tags` (`tags: [done]`) |
| Options weighed with pros and cons | `options` |
| One decision with its reasons | `decision` |
| Questions for the reader to decide | `asks` |
| Results of checks or tests | `checks`, each with `result: ok`, `ng` or `unchecked`; floe counts the results at the top. Not the conditions of a choice (`table`, `options`) nor what is still to do (`checklist`) |
| Events at dates | `timeline`; stages without dates are `steps` |
| Short values compared in rows and columns; before-and-after values in separate columns (the first column's heading empty) | `table` |
| Independent blocks side by side, each with its own parts, such as before-and-after screens or code | `columns` |
| A few key values at the top | `facts` |
| Numbers over time, or compared | `chart` |
| Structure or flow: boxes and arrows | `diagram` (*Diagrams*) |
| Code; commands with what they printed | `code`; `terminal` (write no `$ `) |

- Do not draw a table or a diagram with characters in `text` or `code`: use `table` or `diagram`.

```yaml
- text: |
    One paragraph of **Markdown**.

    A second paragraph, with a `code` word.
- note: { tone: warn, text: 'Edits during the freeze are lost.' }
- facts:
    - { name: Owner, value: Ana }
- steps:
    - { title: 'Freeze edits', state: done }
- entries:
    - { title: 'Broken links', tags: ['120 pages'], text: 'A link check runs after the copy.' }
- checks:
    - { title: 'Pages copied', result: ok, actual: 'All 1840 pages.' }
    - { title: Search, result: ng, actual: 'The index is empty.' }
    - { title: 'Old links', result: unchecked, actual: 'Waits for the copy.' }
- table:
    columns: [Page, { title: Links, num: true }]
    rows:
      - [Home, '12']
```

## Diagrams

Read `floe diagram guide` before writing a diagram: it says how to write one, what floe refuses, and the icons to draw with: `lucide`, built in, for users, servers, databases and the other things any diagram shows, and those of AWS, Google Cloud and Azure for their services.

## YAML that floe reads as you meant

Write so:

- Write a text value in `'…'`, or over several lines as a `|` block, without judging whether it needs quotes: `heading: 'Step 1: plan'`, ``text: '`algorithms` fixes the methods.'``. In `'…'`, write a `'` as `''`; nothing else is special, a `\` and a `"` included. In a `|` block, write the text as it is. Leave bare only a number, `true` or `false`, and a word that starts with an ASCII letter or digit, holds only ASCII letters, digits, `_`, `.`, `/` and `-`, and does not read as a number (`template: report`, `tone: warn`; not `1.10`, `08` or `1_000`, as below).
- Write a comment on a line of its own, no deeper than the key of the value above it (or the `-` of a list item).
- In a `|` block, indent the first line as the lines after it, with spaces only.

floe refuses these:

- A text MUST NOT be left bare where YAML reads it as something else: with a `: ` (a key) or a ` #` (a comment) in it, starting with a backtick, or reading as a number, `null` or `~` where only text goes (`'42'` in a table's cell, `'0x1F'`, `'null'`, `'-'`).
- A comment MUST NOT stand in `{ }` or `[ ]`. The first line of a `|` block MUST NOT be deeper than the next, nor hold a tab after its spaces.
- A file MUST be one document (no `---`), with no anchors, aliases, merge keys (`<<`) or tags.

These pass `floe check` and change what you wrote:

- Do not leave bare a text that reads as a number where a number goes too: `default: 1.10` of `fields` is drawn `1.1`. Write `'1.10'`.
- Do not write a `,` in a bare item of `[ ]`: it ends the item (`tags: [Tokyo, Osaka office]` is two tags).
- Do not use `"…"`: a `\` in it is an escape, mostly with no mistake (`\_` is a no-break space).
- Beside Japanese, Chinese or Korean letters, `**`, `*` and `~~` mark text across punctuation too (`**「題」**に` is bold): write a `*` meant as a character there as `\*` or in backticks.

`heading: Step 1: plan`, `lead: PR #100` and `42` in a table's cell are refused so:

```text
h.yaml:5:14: mapping value is not allowed in this context
c.yaml:4:10: a # began a YAML comment, which may have cut the value short, to PR: if the # is part of the value, put the value in quotes, or, in a | or > text, indent all its lines alike, the # line too, or write its indentation after the | or > (like |2); else write the comment on a line of its own, no deeper than the value's key (or the - of an item)
t.yaml:10:16: got number, want string or object
```

## Reading mistakes

- A mistake names a place in what you wrote, `file:line:column:` (or `file:` for the whole file): fix it there. Anything else is a failure, of floe or of the machine (a port in use, …): tell the user what floe printed.
- `floe check` prints one mistake a line and exits with code 1. It checks in stages, so fixing the mistakes may bring out those of the next stage.
- A key a part does not take, a key it needs left out, or a value of the wrong type is a mistake of the schema: read the schema (*Picking parts*).

```text
plan.yaml:8:19: additional properties 'colour' not allowed
plan.yaml:9:17: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

## More

- `floe render db-choice.yaml` writes `db-choice.html` beside it, to keep in a repository.
- `floe publish --link file.html` shows an HTML file of your own as it is.
- Templates, parts and themes of your own: `floe template --help`, `floe part --help` and `floe theme --help`.
- To change a data file from a program, write it with a YAML library set to write no anchors or aliases, to keep non-ASCII text as it is, and to keep the keys' order.
