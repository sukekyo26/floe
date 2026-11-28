# Using floe

floe draws a page from a YAML data file you write, checks it, and serves it at the URL `floe publish` prints or keeps it in a repository as HTML. To show the user a page, start at *Writing a page*; to write or change a document a repository keeps, at *Documents in a repository*. `floe <command> --help` and the `list` and `show` commands say the rest.

## Guides to read first

| Before you | Run, and follow |
|---|---|
| write or change a document a repository keeps (*Documents in a repository*) | `floe repository guide` |
| set a repository up to keep a project's documents, or to check them in CI, when the user asks | `floe repository guide` |
| write a diagram | `floe diagram guide`; it tells of the icons too (`lucide`, built in, for users, servers, databases and other common things, and those of AWS, Google Cloud and Azure) |
| put a page on a task, or read or change the user's tasks, when the user asks | `floe task guide` |
| make or change a template of your own | `floe template guide` (its commands: `floe template --help`) |
| make or change a part, a look or a shared schema of your own | `floe part guide` (`floe part --help`) |
| make or change a theme of your own | `floe theme guide` (`floe theme --help`) |

## Writing a page

1. Pick a template from `floe template list`: the one made for the page's purpose if there is one (`prd` for what to build and how it is known to be done, `tech-spec` for how it is built, `design` for how the screens look, `meeting` for minutes, `handoff`, `runbook`, …), else `report`. Read its example and schema before writing: `floe template show <template> example.yaml` and `floe template show <template> schema.json`. The example shows the file's structure, not a fixed set of parts.
2. Name the file for the page, as `db-choice.yaml` (*The data file*).
3. Choose parts from `floe part list` that the template's schema allows where you put them (*Picking parts*). A part MUST NOT hold a key its schema does not list: look up the keys of each part the example does not show, in `floe part list` or, where it lists none (`chart`, `stats`, `diagram`, …), in `floe part show <part> schema.json`.
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
| `theme` | a name `floe theme list` prints | The theme the page is drawn with; leave it out (*Themes*) |

- The other top keys are the template's: its `schema.json` lists them (a `report`'s `lead` sums the page up under the title).
- The file's name without `.yaml` becomes the page's slug in its URL, in lower case, with each run of characters other than ASCII letters and digits made a `-`, cut to 64 characters, with no `-` at either end. A name with no ASCII letter or digit (`設計.yaml`) gives no slug: give `floe publish --slug <slug>`.
- A `--slug` MUST be 1 to 64 lower case ASCII letters, digits and `-`, starting and ending with a letter or digit.

## Documents in a repository

A document a repository keeps is a YAML file with `template` and `lang` at its top, in a repository that draws its pages: it has `.floe/repository.yaml` at its root, its AGENTS.md, CLAUDE.md, README, CI or scripts speak of floe, or it has a page committed beside the file. Run `floe repository guide` first, and follow it. Do not edit the `.html` there, and do not publish such a file.

## Publishing

`floe publish db-choice.yaml` serves the page and prints its URL. To have the page as an HTML file instead, without publishing it, `floe render db-choice.yaml` writes `db-choice.html` beside it (a document a repository keeps is drawn with `floe repository render`, which draws with the repository's templates, as `floe repository guide` says).

### The page's project

- A project groups pages on the management page and the task list. A new page's project has the name of the git repository `floe publish` runs in: the last part of its `origin`'s URL without `.git`, else its root directory's name (in a worktree, the main checkout's); outside a repository, the directory's name. Where the data file is does not matter (`cd ~/work/app && floe publish ~/notes/db-choice.yaml`).
- When a page is new or its project changes (it had none, it replaced another file's page, or `--move-project` moved it), `floe publish` prints the project on stderr after the URL (`page in project app`). If it is not the name of the repository the page is about, move to that repository and publish again with `--move-project` (not for a `--link` page: *Linked files*).
- `--move-project` takes the page off its task, as a task is of its project, and says so (`page in project app, off task #9`); the task stays in the old project without the page. Tell the user, who decides whether to put the page on a task.
- A page in the wrong project, of any kind, can have its project changed on the management page. Tell the user how: the page's `…` menu (on its row in the list, or over the page when it is open), Edit, open Slug and project, which is folded, type the project's name in Project, and Save. The screen is in the browser's language: in Japanese these are 編集, URL 名とプロジェクト, プロジェクト and 保存.
- `floe info <slug>` prints a page's project, task, data file and the rest floe keeps of it, a name, a tab and a value a line, an empty value for none, and changes nothing: run it to check the project of a page you did not just publish.

### Publishing a file again

- floe knows a page by its data file, not by the directory floe runs in: publish the same file again, from any directory and without `--slug`, and it replaces its page, even after the user renamed the page on the management page.
- A slug MUST NOT be that of another file's page (`slug "db-choice" was published from …: choose another slug`): rename your file (`db-choice-2.yaml`) or give another `--slug`, and publish again.
- A page stored before floe 0.1.0 does not say which file it is of, and is refused from its own file too: ask the user before you pass `--force`, which drops its versions, tags, favourite and task.

### Linked files

`floe publish --link file.html` shows an HTML file, or a folder, of your own as it is.

- Its page is in the project of the linked file's directory (for a folder, the folder itself), not of where `floe publish` runs: link a file inside the repository the page is about.
- Linking it again keeps that project, `--move-project` does not go with `--link`, and only the management page changes it (*The page's project*).
- On Windows, the file or folder MUST be on a local drive (*Limits*) and, in addition, MUST NOT be in `AppData`, `%TEMP%` included: write it in the repository the page is about.

### Limits

- Drawing a page, and the page drawn, MUST stay within these limits: past any, floe refuses the page at its data file.
  - 512 MiB of memory to draw it (`drawing it needed more than 512 MiB of memory`)
  - 30 seconds to draw it on the server (`drawing it took more than 30 seconds`)
  - 80 MiB of HTML drawn (`the page drawn is … MiB, over the 80 MiB a page may be`)
- Do not repeat a long text for each item of a list, in an expression or in `page.html` (`floe template guide`).
- On Windows, what floe reads and writes MUST be on a local drive: the data file, a file or folder to link, the repository, the store, and the directories of your own templates, parts and themes. Any local folder will do; a network share or a network drive will not, nor a symlink or junction that leads to one.

## Tasks

When the user asks you to put a page on a task, or to read or change their tasks, read `floe task guide` first: it says how.

### Ask after publishing

The user keeps a task list, and wants each new page put on a task, under one of its categories.

#### How to ask

- When `floe publish` prints the page's project (`page in project app`, or `page in no project`), after `--move-project` too (`page in project app, off task #9`), ask which task to put the page on, then, if it goes on one, which of its categories.
- Below, `<project>` is the name after `page in project`, up to a comma if any, or `""` for no project.
- Ask in a dialog of choices if you have one (Claude Code's `AskUserQuestion`), not in your reply's text: the user does not see your reply while the dialog is open. With no such dialog, ask in a reply that holds nothing but the question, its choices listed last.
- Either way, write in the question all the user needs, the page's title and URL first, and offer four choices at most.
- When the user picks `A new task` or `A new category` without typing its ID and title or its name, ask for them in a reply that holds nothing but the question, the page's title and URL first: a dialog needs two choices or more.
- When the user says not to use tasks, do not ask again in the conversation; when the user says not to use categories, ask for the task alone.
- When your instructions (a CLAUDE.md, a skill) name the task a page goes on, put it there without asking for the task, and ask for the category unless they name it too.

#### The task

- Offer the first two tasks `floe task --project <project>` prints, each named by its ID with its title as the choice's description in a dialog, and by its ID then its title with no dialog (`#12 Login screen`), then `A new task` and `None`; with fewer open tasks, those there are.
- For example: `Put "Login design" (<its URL>) on a task? #12 Login screen / #15 Price table / A new task / None`.
- Say in the question that the user can type instead another task's ID or words of its title, or a new task's ID and title (`#20 Search page`).

#### The category

- `floe task` prints a task's categories in order after `categories`, each a Go string literal, with spaces between (`categories "Designs" "Samples" "Ideas"`): read each name as Go reads the literal, without its quotes, `\t` a tab, `\"` a `"` and `\\` a `\` (`"A \"B\""` is `A "B"`).
- Offer the first two of the task's categories, each named as read so, then `A new category` and `None`; a new task, or one with no category, gets `A new category` and `None`.
- Say in the question that the user can type instead a new category's name, and, when the task has more categories, another of them, naming them all.
- The first time in the conversation, also say in the question what a category is: a heading on the task's page that its pages are listed under (`Designs`, `Samples`); a page of no category is listed first, under `No category`.

#### Putting the page

- Take a task's ID, a new task's title and a category's name from the user's answers. When the user types words of a task's title, find the task in what `floe task --project <project>` prints, and ask again if none or several match.
- When the user types an ID, run `floe task --project <project> --task "<ID>"`, which tells a task in any state: it is a new task only when floe says there is none (`no task #20 in project app`). For a task there is, leave the words after its ID unused.
- A category name the user types that is one of the task's categories but for upper and lower case is that category, named as read from `floe task`; one that matches none is a new category.
- Put the page there without reading `floe task guide`: `floe task --slug <slug> --task "<ID>"`, the ID `#` and all (`--task "#12"`), with `--category '<name>'` for a category.
- For a new task, add `--new --title '<title>'` to that command: floe makes a task by `--new` alone. Do not give `--title` for a task there is: it changes that task's title.

## Themes

- Do not write `theme` unless the user names a theme for the page: the user picks themes on the management page, at the root of the page's URL.
- When you give the user a URL, run `floe theme default`. If it prints a name, say nothing of themes. If it prints nothing, tell the user once in the conversation that the default theme for new pages can be chosen in the management page's settings (the gear in the page list's bar).

## Picking parts

Each item of `parts` is one key naming the part. `floe part list` says what each part is for and gives most parts' keys, `floe part show <part> schema.json` a part's keys, types and details. The template's `schema.json` says which parts each section or field takes.

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
| Structure or flow: boxes and arrows | `diagram` (*Guides to read first*) |
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

## YAML that floe reads as you meant

Write so:

- Write a text value in `'…'`, or over several lines as a `|` block, without judging whether it needs quotes: `heading: 'Step 1: plan'`, ``text: '`algorithms` fixes the methods.'``.
- In `'…'`, all on one line, write a `'` as `''`; YAML reads nothing else there specially, not a `\` nor a `"` (Markdown does, as below). In a `|` block, write the text as it is, its first line indented as the lines after it, with spaces only.
- Leave bare only a number, `true` or `false` that is not meant as text, and a word that starts with an ASCII letter or digit, holds only ASCII letters, digits, `_`, `.`, `/` and `-`, and does not read as a number or `null` (`template: report`, `tone: warn`; not `1.10` or `08`, as below).
- Write a comment on a line of its own, before the key it is about, not on a value's line; one on a `|` block goes on its `|` line (`text: | # note`), also where `|` blocks follow each other. A comment MUST NOT follow a bare text on its line, in `{ }` or `[ ]` too, nor come right after a `|` or `>` block, at any depth.
- To change a data file from a program, write it with a YAML library set to write no anchors or aliases, to keep non-ASCII text as it is, and to keep the keys' order.

These pass `floe check` and change what you wrote:

- Do not leave bare a text that reads as a number where a number goes too: `default: 1.10` of `fields` is drawn `1.1`. Write `'1.10'`.
- Do not write a `,` in a bare item of `[ ]`: it ends the item (`tags: [Tokyo, Osaka office]` is two tags).
- Do not use `"…"`: a `\` in it is an escape, mostly with no mistake (`\_` is a no-break space).
- In a value that is Markdown (`text`, a table's cell, …), in `'…'` and `|` blocks too, a `\` before a punctuation mark is taken off (`C:\path\_logs` is drawn `C:\path_logs`), and a paragraph whose first line starts with four spaces is code: write a path in backticks, or a `\` as `\\`.
- Beside Japanese, Chinese or Korean letters, `**`, `*` and `~~` mark text across punctuation too (`**「題」**に` is bold): write a `*` meant as a character there as `\*` or in backticks.
- Do not indent the first line of a `|` block deeper than a later one: the block ends before the first shallower line, which is read as YAML.
  - A line that reads as a key or an item, as deep as the block's key or `-`, is taken as one, with no mistake where it is allowed.
  - Any other is refused, at that line or one after it.
  - For text whose first line starts with spaces, write after the `|` how far the block is indented past its key (`|2`).

floe refuses these (*Reading mistakes*):

- A text MUST NOT be left bare where YAML reads it as something else: with a `: ` (a key) or a ` #` (a comment) in it, starting with a backtick or `%`, or reading as a number, `null` or `~` where only text goes (`'42'` in a table's cell, `'0x1F'`, `'null'`, `'-'`): put it in `'…'`.
- A file MUST be one document (no `---`), with no anchors, aliases, merge keys (`<<`) or tags, and each key written once in its keys and values.

## Reading mistakes

| A mistake | Do |
|---|---|
| with a place in what you wrote, `file:line:column:` (or `file:` for the whole file) | Fix it there |
| without a place, about what you gave the command (a path, a flag, a slug, a task, a `FLOE_…` you set yourself) | Fix what you gave |
| any other, about the user's settings (`FLOE_…` of their environment, the port), floe or the machine (a port in use, …) | Tell the user what floe printed, and change none of their settings and stop no server yourself |

`heading: Step 1: plan`, `lead: PR #100` and `42` in a table's cell are refused so:

```text
h.yaml:5:20: mapping values are not allowed in this context
c.yaml:4:7: a # on the line of this value not in quotes begins a YAML comment, which may have cut the value short, to PR: if the # is part of the value, put the value in quotes; else write the comment on a line of its own
t.yaml:10:16: got number, want string or object
```

A key a part does not take, a key it needs left out, or a value of the wrong type is a mistake of the schema: read the schema (*Picking parts*).

```text
plan.yaml:8:19: additional properties 'colour' not allowed
plan.yaml:9:17: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```
