# Using floe: a guide for agents

floe turns a YAML file you write into a page for the user: it checks the file, renders it with a template, and serves it at the URL printed by `floe publish`. This guide, `floe <command> --help`, and the `list` and `show` commands tell you all you need to know.

## Writing a page

1. Pick a template from `floe template list`:
   - the one made for the page's purpose, if there is one (`design` for a design document, `handoff`, `runbook`, …);
   - else `report`;
   - before writing, read its example and schema: `floe template show <template> example.yaml` and `floe template show <template> schema.json`. The example shows the file's structure; do not treat its parts as a fixed set. Pick parts in step 3.
2. Name the file for the page, as `db-choice.yaml`. The file's name without `.yaml` becomes the page's slug in its URL. Every data file needs `template` and `lang`; `title` names the page, and a `report`'s `lead` sums it up under the title. Leave `theme` out ([Themes](#themes)).

   ```yaml
   template: report
   lang: en
   title: Moving the team wiki
   lead: The wiki moves in two weekends.
   sections:
     - heading: Steps
       parts:
         - text: Freeze edits on **Friday evening**, then copy the pages.
   ```

3. Run `floe part list` and choose parts that fit the content from those allowed by the template's schema (read in step 1). Where the choice is easily mistaken, use the table in [Picking parts](#picking-parts). For each part the example does not show, look up its keys before writing it, as that section says: a guessed key is refused.
4. Run `floe check db-choice.yaml`, fix every mistake, and run it again until none is left. When only `warning:` lines are left, you are done; they are advice.
5. Run `floe publish db-choice.yaml` from the repository the page is about (`cd ~/work/app && floe publish ~/notes/db-choice.yaml`), and give the user the URL it prints. A new page goes in a project, which groups pages on the management page and the task list. floe names the project after the git repository `floe publish` runs in: its `origin`'s name if it has one, else the repository's directory name. Outside a git repository, it uses the name of the directory it runs in. Where the data file is does not matter. `floe publish` says it on stderr after the URL (`page in project app`): if that is where you happened to be (`tmp`) and not the repository the page is about, move to that repository and publish again with `--move-project`, which puts the page in the project of the directory it runs in (then check the page's task with `floe work`, and set it again if it is not the one you mean). The user can also change the project in the page's Edit on the management page (Slug and project). You need not fetch the page.

## Publishing again

- floe identifies a page by its source file, not by the directory you run floe from: publish the same file again, from any directory, without `--slug`, and it replaces its page, even after the user renamed the page on the management page.
- If floe refuses the name, a page of another file has it: rename your file (`db-choice-2.yaml`) or pass another `--slug`, and publish it again.

## Themes

- Do not write `theme` unless the user names a theme for the page. The user picks themes on the management page, at the root of the page's URL.
- When you give the user a URL, run `floe theme default`. If it prints a name, say nothing of themes. If it prints nothing, tell the user once in the conversation that the default theme for new pages can be chosen in the management page's settings (the gear).

## Picking parts

Each item of `parts` is one key naming the part. `floe part list` lists each part's keys and what it is for. For parts whose keys are not listed (`chart`, `stats`, `diagram`, …), read `floe part show <part> schema.json` to see their keys, types and details. The template's `schema.json`, read in step 1, says which parts each section or field accepts.

Use this table when the right part is not obvious:

| Content | Part |
|---|---|
| Prose, or a short list | `text` (Markdown) |
| Steps in order, to do or done | `steps`; floe numbers them, so write no number in a title |
| Items with longer explanations, such as risks, requirements or terms | `entries`; put short states or labels in each item's `tags` (`tags: [done]`) |
| Options weighed with pros and cons | `options` |
| One decision with its reasons | `decision` |
| Questions for the reader to decide | `asks` |
| Results of checks or tests | `checks`, each with `result: ok`, `ng` or `unchecked`; floe counts the results at the top; not the conditions of a choice (`table`, `options`) nor what is still to do (`checklist`) |
| Events at dates | `timeline`; stages without dates are `steps` |
| Short values compared in rows and columns; before-and-after values in separate columns (leave the first column's heading empty) | `table` |
| Independent blocks to view side by side, each with its own parts; useful for before-and-after screens or code | `columns` |
| A few key values at the top | `facts` |
| Numbers over time, or compared | `chart` |
| Structure or flow: boxes and arrows | `diagram` ([Diagrams](#diagrams)) |
| Code; commands with what they printed | `code`; `terminal` (write no `$ `) |

Draw no table or diagram with characters in `text` or `code`. Values look like this:

```yaml
- text: |
    One paragraph of **Markdown**.
- note: { tone: warn, text: Edits during the freeze are lost. }
- facts:
    - { name: Owner, value: Ana }
- steps:
    - { title: Freeze edits, state: done }
- entries:
    - { title: Broken links, tags: [120 pages], text: A link check runs after the copy. }
- checks:
    - { title: Pages copied, result: ok, actual: All 1840 pages. }
    - { title: Search, result: ng, actual: The index is empty. }
    - { title: Old links, result: unchecked, actual: Waits for the copy. }
- table:
    columns: [Page, { title: Links, num: true }]
    rows:
      - [Home, "12"]
```

## Diagrams

floe validates each diagram part: it refuses text that overflows, overlapping parts, and edges that pass through boxes, reporting the line to change. Choose how to write the diagram based on what it needs to show.

A `graph` for a flow, a state machine, or a diagram that changes often or joins many things: write the boxes and what joins what, and floe places them. `x` and `y` (required) only say where the graph starts.

```yaml
- diagram:
    title: How a page is published
    parts:
      - graph:
          x: 20
          y: 20
          nodes:
            - box: { name: write, rows: [Write the data] }
            - box: { name: check, shape: hex, rows: ["Checks pass?"] }
            - box: { name: show, rows: [Show the page] }
          edges:
            - { from: write, to: check }
            - { from: check, to: show, label: "yes" }
            - { from: check, to: write, label: "no" }
```

Use manually positioned parts for diagrams read as pictures, such as architecture or layout diagrams, where the spatial arrangement matters. Give each box an `x` and `y`. Leave `w` and `h` out to let floe size the box to its text; specify them when you need to align boxes. Name both ends of each edge.

```yaml
- diagram:
    title: How a request flows
    parts:
      - box: { name: user, x: 20, y: 40, rows: [User] }
      - box: { name: api, x: 220, y: 40, rows: [API server] }
      - box: { name: db, x: 220, y: 160, rows: [Database] }
      - edge: { from: user, to: api, label: HTTPS }
      - edge: { from: api, to: db }
```

In an AWS architecture, represent each service with an AWS icon, not a box. Do not draw a box around the icon. First run `floe icons fetch aws` if the icon set is not kept yet, then `floe icons search <word>` finds a name (a short one such as `alb` works).

`floe diagram guide` explains both ways in detail, including their parts, keys, layout rules and checks. Read it before writing a larger diagram. `floe diagram example <kind>` prints a complete spec of each kind (architecture, sequence, ER, …) to copy. The examples set `w` and `h` to fix box sizes on their grid, so that boxes align. If you change a box's text, omit its `w` or increase it to at least the minimum floe reports (`needs w: 174 or more`).

## Writing YAML that floe reads as you meant

First:

- Quote text that holds `: ` or ` #`, that starts with a backtick, and, in `{ }` or `[ ]`, a `,`: `heading: "Step 1: plan"`, ``text: "`algorithms` fixes the methods."``. Unquoted, YAML reads them as a key, a comment or the end of the text, and refuses a text that starts with a backtick. floe refuses most of these, but a `,` in `[ ]` silently splits the item: `tags: [Tokyo, Osaka office]` is two tags.
- Write a comment on a line of its own, no deeper than the key of the value above it (or the `-` of a list item), and never in `{ }` or `[ ]`.
- In a `|` or `>` text, indent the first line as the lines after it. A deeper first line ends the text early, and floe refuses a line after it.
- Write a number where text is wanted, such as a table's cell, in quotes: `"42"`.
- Write one document: no `---`, anchors, aliases, merge keys (`<<`) or tags.

Then:

- Beside Japanese, Chinese or Korean letters, `**`, `*` and `~~` are read across a mark too (`**「題」**に` is bold): write a `*` meant as a mark there as `\*` or in backticks.
- In a `|` or `>` text, indent with spaces only: a tab right after the spaces of its first line is refused.

`heading: Step 1: plan`, `lead: PR #100` and `42` in a table's cell are refused so:

```text
h.yaml:5:14: mapping value is not allowed in this context
c.yaml:4:10: a # began a YAML comment, which may have cut the value short, to PR: if the # is part of the value, put the value in quotes, or, in a | or > text, indent all its lines alike, the # line too, or write its indentation after the | or > (like |2); else write the comment on a line of its own, no deeper than the value's key (or the - of an item)
t.yaml:10:16: got number, want string or object
```

## Reading mistakes

A mistake is in what you wrote, so you fix it. A failure is in floe itself, so tell the user what it printed.

`floe check` prints one mistake per line, prefixed with `file:line:column:` (or just `file:` for a mistake in the whole file), and exits with code 1. It checks in stages, so fixing the mistakes may bring out those of the next stage.

- Schema mistakes include keys a part does not accept, missing required keys, and values of the wrong type: read the schema, as [Picking parts](#picking-parts) explains.
- A mistake of YAML: see the section above.

```text
plan.yaml:8:11: additional properties 'colour' not allowed
plan.yaml:9:17: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

## More

- `floe render db-choice.yaml` writes `db-choice.html` beside it, to keep in a repository.
- `floe publish --link file.html` shows an HTML file of your own as it is.
- To change a data file from a program, read and write it with a YAML library set to write no anchors or aliases, to keep non-ASCII text and the keys' order (PyYAML: `allow_unicode=True`, `sort_keys=False`, a Dumper whose `ignore_aliases` returns true).
