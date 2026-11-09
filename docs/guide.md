# Using floe: a guide for agents

floe shows the user a page you write as a YAML file: it checks the file, draws it with a template and serves it at the URL `floe publish` prints. This guide, `floe <command> --help` and the `list` and `show` commands tell all you need.

## Writing a page

1. Pick a template from `floe template list`:
   - the one made for the page's purpose, if there is one (`design` for a design document, `handoff`, `runbook`, …);
   - else `report`;
   - before writing, read its example and schema: `floe template show <template> example.yaml` and `floe template show <template> schema.json`.
2. Name the file for the page, as `db-choice.yaml`: its name, without `.yaml`, names the page in its URL. Leave `theme` out ([Themes](#themes)).

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

3. Pick the parts by the table in [Picking parts](#picking-parts), and look up the keys of each part the example does not show in `floe part list` before writing it: a guessed key is refused.
4. Run `floe check db-choice.yaml`, fix every mistake, and run it again until it prints no mistake; when only `warning:` lines are left, you are done (they are advice).
5. Run `floe publish db-choice.yaml` and give the user the URL it prints. You need not fetch the page.

## Publishing again

- Run `floe publish` again in the same project. floe takes the project from the directory you run it in (its git repository, from anywhere inside; outside one, the directory's name), not from where the file is. Without `--slug`, it replaces the page, even after the user renamed it on the management page.
- From another project, floe takes the file as another page: it refuses the name, or, for a renamed page, makes a second page. So when the name of a page you published is refused, run `floe publish` where you ran it before.
- Rename the file or pass another `--slug` only when you know the page is not yours, not merely because you do not remember publishing it.

## Themes

- Do not write `theme` unless the user names a theme for the page. The user picks themes on the management page, at the root of the page's URL.
- When you give the user a URL, run `floe theme default`. If it prints a name, say nothing of themes. If it prints nothing, tell the user once in the conversation that the default theme for new pages can be chosen in the management page's settings (the gear).

## Picking parts

Each item of `parts` is one key naming the part. `floe part list` gives each part's keys (a part takes no other); where it names none, read `floe part show <part> schema.json`. A template of your own: its `schema.json`.

Pick the part by what the content is:

| Content | Part |
|---|---|
| Prose, or a short list | `text` (Markdown) |
| Steps in order, to do or done | `steps`; floe numbers them, so write no number in a title |
| Items, each with a longer explanation: risks, requirements, terms | `entries`, a state in a tag |
| Options weighed with pros and cons | `options` |
| One decision with its reasons | `decision` |
| Questions for the reader to decide | `asks` |
| Results of checks or tests | `checks`, each passed, failed or not checked, counted on top; not the conditions of a choice (`table`, `options`) nor what is still to do (`checklist`) |
| Events at dates | `timeline`; stages without dates are `steps` |
| Rows compared along and across, each cell short | `table` |
| A few key values at the top | `facts` |
| A thing before and after | `compare` |
| Numbers over time, or compared | `chart` |
| Structure or flow: boxes and arrows | `diagram` ([Diagrams](#diagrams)) |
| Code; commands with what they printed | `code`; `terminal` (write no `$ `) |

For anything else (review findings, large numbers, files, fields, links, …) read the descriptions in `floe part list`. Draw no table or diagram with characters in `text` or `code`. Values look like this:

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

Write a `graph`: the boxes and what joins what. floe places the boxes; give only where the graph starts (`x`, `y`).

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

`floe diagram example` lists examples of each kind (architecture, sequence, ER, …); `floe diagram example <kind>` prints one to copy. A diagram whose text overflows or whose parts overlap is refused at the line to change.

## Writing YAML that floe reads as you meant

First:

- Quote text that holds `: ` or ` #`, and, in `{ }` or `[ ]`, a `,`: `heading: "Step 1: plan"`. Unquoted, YAML reads them as a key, a comment or the end of the text. floe refuses most of these, but a `,` in `[ ]` silently splits the item: `tags: [Tokyo, Osaka office]` is two tags.
- Write a comment on a line of its own, no deeper than its key, and never in `{ }` or `[ ]`.
- In a `|` or `>` text, indent the first line as the lines after it. A deeper first line ends the text early, and floe refuses a line after it.
- Write a number where text is wanted, such as a table's cell, in quotes: `"42"`.
- Write one document: no `---`, anchors, aliases, merge keys (`<<`) or tags.

Then:

- Beside Japanese, Chinese or Korean letters, `**`, `*` and `~~` are read across a mark too (`**「題」**に` is bold): write a `*` meant as a mark there as `\*` or in backticks.
- Begin a `|` or `>` text without a tab after its spaces.

`heading: Step 1: plan`, `lead: PR #100` and `42` in a table's cell are refused so:

```text
h.yaml:5:14: mapping value is not allowed in this context
c.yaml:4:10: a # began a YAML comment, which may have cut the value short, to PR: if the # is part of the value, put the value in quotes, or, in a | or > text, indent all its lines alike, the # line too, or write its indentation after the | or > (like |2); else write the comment on a line of its own, no deeper than the value's key (or the - of an item)
t.yaml:10:16: got number, want string or object
```

## Reading mistakes

`floe check` prints one mistake to a line, at `file:line:column:` (one of the whole file at `file:` alone), and exits 1. It checks in stages: after fixing all it printed, run it again, as the next stage may print more.

- A key a part does not take, a key it lacks, or a value of another type is the schema refusing it: read `floe part show <part> schema.json`, or the template's `schema.json`.
- A mistake of YAML: see the section above.

```text
plan.yaml:5:5: additional properties 'colour' not allowed
plan.yaml:8:23: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

## More

- `floe render db-choice.yaml` writes `db-choice.html` beside it, to keep in a repository.
- `floe publish --link file.html` shows an HTML file of your own as it is.
- To change a data file from a program, read and write it with a YAML library set to write no anchors or aliases, to keep non-ASCII text and the keys' order (PyYAML: `allow_unicode=True`, `sort_keys=False`, a Dumper whose `ignore_aliases` returns true).
