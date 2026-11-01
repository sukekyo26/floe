# Writing pages

How to write a data file, the YAML that floe draws a page from with one of floe's templates or one of your own. A `diagram` part's spec is in [diagrams.md](diagrams.md), and showing an HTML file or a folder as it is in [links.md](links.md); [templates.md](templates.md) tells how to make a template of your own, and [commands.md](commands.md) every command and flag.

## A data file

A data file names a template, and the template draws the page from its data.

```yaml
template: report
lang: en
title: Moving the team wiki
eyebrow: Plan · 2030-04
lead: The wiki moves in two weekends.
sections:
  - heading: Steps
    parts:
      - text: |
          1. Freeze edits on **Friday evening**.
          2. Copy the pages.
      - terminal:
          shell: bash
          steps:
            - { run: wiki export --out pages/, out: 1840 pages written }
      - entries:
          - title: Broken links
            tags: [120 pages, { pill: watching, tone: warn }]
            text: A link check runs after the copy.
      - note: { tone: warn, text: Edits during the freeze are lost. }
```

```sh
floe check plan.yaml            # prints its mistakes and warnings, or nothing
floe publish plan.yaml          # stores the page and prints its URL
floe publish --link plan.yaml   # draws it each time it is opened, reloading as the file is saved
floe render plan.yaml           # writes plan.html beside it, which opens without floe
```

[commands.md](commands.md) tells their flags.

Every data file holds these keys, which floe reads itself:

| Key | What it is |
|---|---|
| `template` | The template that draws the page (one of floe's below, or one of your own). Required. |
| `lang` | The page's language (`<html lang>`), as a tag such as `en`, `ja` or `pt-BR`. Required. The words floe and the parts draw (`required` in `fields`, the zoom buttons of a diagram, the copy button of code) and the text of a diagram follow it, English for a language they have no words of. |
| `title` | The page's title. |
| `theme` | The theme the page is drawn in (default `default`): its colours, faces and the shape of the parts. `floe template show <template>` lists the themes on its `themes:` line: `default`, `soft`, `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `retro`, `swiss`, `washi`, `aurora` and `app`. |

The rest is the template's. floe's templates:

| Template | For | Required |
|---|---|---|
| `report` | A report: sections of parts, each with subsections one level deep | — |
| `design` | A design document, its conclusion first | `background`, `goals` |
| `handoff` | A handoff of work or a status update, what to decide first | `date` |
| `adr` | A decision record, one decision on a sheet | `status`, `context`, `decision` |
| `postmortem` | An incident review, written without blame | `summary`, `timeline`, `cause` |
| `runbook` | Steps to follow, with the commands to run | `steps` |
| `slides` | Slides to show one at a time | `slides` |

`floe template list` describes each, and `floe template show <template>` prints its keys, each described in its `schema.json`, and an `example.yaml` to start from: an item of `"format": "markdown"`, or described as Markdown, is Markdown. What a page shows beyond its keys:

- `report` takes `eyebrow` (a short line above the title), `lead` (a sentence or two under it) and `sections`, each a `heading` over `parts`, and `sections` of its own after the parts: subsections, each a `heading` (an `<h3>`) over `parts`, one level only. A section with subsections has a heading; a heading is never empty, so a section with none leaves the key out.
- The keys that are not Markdown are drawn as written: report's `eyebrow`, `lead` and headings; design's `status`, `owner` and `date`; handoff's `date`, `from`, `to`, `by`, `ref` and `who`; adr's `number`, `date` and `deciders`; postmortem's `date`, `authors`, `owner` and `ref`; runbook's `owner`, `updated` and a step's `who`; and the heading of a section of `more`.
- `handoff` heads the page with the counts of what is done, in progress, to decide and next, and numbers `asks` Q1, Q2 and on.
- `runbook` draws a step's `who` as a badge beside its title, its `run` with the copy button of `terminal`, and `rollback` in a frame of its own.
- `slides` shows each slide as tall or wide as the screen with its number and the count in the corner, and prints each on a page of its own, sideways. Its parts are as large as in `report`: zoom in with the browser to make them larger. With `direction: across`, a mouse's wheel scrolls down only, and in the management page's frame the ← → keys work once the frame is clicked.

## Parts

Each part is one key naming its kind. `floe part list` lists them with a line each, and `floe part show <name> schema.json` prints what one takes (its schema, as written).

To list things, pick the part by what the rows are:

- One line each: a Markdown list in `text`.
- A name and its longer text, which a table squeezes into a narrow column, such as requirements, risks and what is done about them, or terms: `entries`, a state in a tag.
- A decision: `decision` for one told with its reasons and the options set aside, `options` for options weighed side by side with their pros and cons, and `entries` for a run of short decisions, the pick in a tag.
- What happened in order: `timeline`.
- Words and what they mean: a Markdown list, or `entries` when a meaning is long.
- A thing before and after: `compare`.
- A few short values at the top of a page: `facts`.
- Rows compared both along and across, each cell short: `table`.

The parts `report` takes in a section:

| Part | What it draws |
|---|---|
| `text` | Markdown, tables included; raw HTML in it is left out. A fenced block of code (```` ``` ````) is drawn as plain code and warned of: write code in `code` and commands in `terminal`. An image file named in it is not shown, only a data URI of a png, gif, jpeg or webp: write images in `image`. |
| `code` | Code as it is, under its `file` with its `lang`. A button over it copies the code. |
| `terminal` | Commands (`run`) and what they printed (`out`), at the prompt of its `shell` (`bash`, `sh`, `zsh`, `powershell`, `cmd`). floe draws the prompt, so a `run` that starts with one (`$ ls`) is refused. A button over them copies the commands, a line each, without the prompt or what they printed. |
| `table` | Rows compared both along and across, each cell short. `columns` and `rows`. A column is its title, or `{ title, num, pills }`: `num: true` for numbers, and `pills`, words with their tones (`{ done: good }`), for a cell of the column that is one of them, whole, to be drawn as that pill. A cell is a line of Markdown, a number in quotes as it is drawn, or a pill `{ pill, tone }`, written as an object, not in quotes. |
| `entries` | Records one under another, each a `title` (a line of Markdown) with its `tags` beside it (each as a table's cell: a line of Markdown or a pill) and its `text` (Markdown) the full width under them. A tag shows no column's name, so say in it what it is (`merged 2026-10-30`). Code in `text` is drawn as it is, without a copy button; put it in a `code` part after the entries. |
| `image` | An image, such as a screenshot: its `file` and its `alt`, what it shows ([Images](#images)). |
| `note`, `stats`, `progress`, `steps`, `checklist` | A note in a tone, large numbers, bars of progress, numbered steps, items ticked once done. |
| `compare`, `options`, `facts`, `diff`, `tree`, `links` | What changes before and after, options with pros and cons, names with values, a diff (a button over it copies the diff as written), files and folders, links. |
| `findings`, `decision`, `fields` | What a review found, a decision, the keys of a setting. |
| `timeline`, `bars`, `faq`, `keys` | Dates in order, bars of values, questions and answers, keys to press. |
| `details`, `tabs` | Parts folded under a title, or shown one at a time under tabs; they hold `text`, `code`, `terminal`, `diff`, `note`, `table`, `entries`, `image` and `diagram`. |
| `diagram` | A diagram, checked and drawn as inline SVG ([diagrams.md](diagrams.md)). |
| `script` | Something that moves ([Scripts and HTML](#scripts-and-html)). |
| `html` | HTML that goes in as it is: a key of `any.json`, not a part of its own, so `floe part show` has none ([Scripts and HTML](#scripts-and-html)). |

A `tone` is `accent`, `good`, `warn`, `bad` or `muted`.

## Writing text

A data file is one YAML document of keys and values. One with `---` between documents, or with anchors, aliases, merge keys (`<<`) or tags, is refused.

- Write text with a `: `, a ` #` or, in `{ }` or `[ ]`, a `,` in it in quotes (`"…"`) or as a block (`|-`): YAML reads them as a key and its value, the start of a comment, or the end of the text.
- A comment on the line of a value not in quotes is refused, in every YAML file floe reads, as it may have cut the value short (`lead: PR #100` reads as `PR`): write the comment on a line of its own.
- A `|` or `>` text whose first line (not of spaces alone) begins with spaces and a tab is refused, in every YAML file floe reads: YAML 1.2 reads the tab as text, but floe's YAML library as indentation, and it drops or shortens the lines after it. Begin the line without the tab, or write the text's indentation after the `|` or `>` (`|2` for text two spaces past its key).
- A key without a value in `{ }` is refused too, as the rest of text a `,` ended (`{ label: 1,500 words }` reads as `label: 1` and a key `500 words`). A `,` in `[ ]` is not caught: `[1,500 words]` is two items.
- A file is UTF-8, without a byte order mark past its start.
- To change a data file from a program, read it with a YAML library, change the values and write it out: the library quotes the text that needs it. Set it to write no anchors or aliases, and to keep non-ASCII text as it is and the keys in their order (in PyYAML: `allow_unicode=True`, `sort_keys=False`, and a Dumper whose `ignore_aliases` returns true).

## Images

In `report`, an image is a part `image` (`- image: { file: shots/list.png, alt: The list of pages }`), in a section or subsection, `details` or `tabs`: its `file` is the path of the image, and its `alt` what it shows. A template of one's own takes an image as an item of `"format": "image"`, whose value is the path. The path is relative to the data file's directory and under it: one `/` between names, without `./`, `..`, or a `/` at either end. The image goes into the page as a data URI, so that the page holds all it shows: a png, jpeg, gif, webp or svg file, by its name, up to 1 MiB. `floe publish` sends the server the images the page shows.

An image file Markdown names (`![x](shots/list.png)`) is drawn as written, and the page's CSP does not show it; only a data URI of a png, gif, jpeg or webp written in it shows. Write images in `image`.

## Diagrams

A `diagram` part, in a section or in `details` and `tabs`, draws a diagram from a spec written in place, checked before the page is drawn: [diagrams.md](diagrams.md) tells how a spec is written.

## Scripts and HTML

- A `script` is `js`, a function the page calls with an empty element in its place and `data`, any value: `function (el, data) { … }`. A mapping in `data` comes in the order of its keys' names (write a list where the order matters), and the function runs where it stands, before the rest of the page is read. A `js` holding `</script` or `<!--` is refused; a mistake inside the function shows in the browser.
- An `html` part goes in as it is. It leaves no element open, nor closes one of the page around it (a `<b>` left open, a `</section>`), refused at the part: write each element whole. The page drawn holds no `<style>`, `<base>`, `<noscript>`, or `<meta>` but for its charset and viewport, whoever writes it, refused at the data file's `template:`: style the HTML with `style` attributes.
- The scripts of a page drawn from a data file cannot send what it holds out: `fetch`, XHR, images from elsewhere and forms are blocked by its CSP. Libraries from cdnjs, jsDelivr and unpkg, and Google Fonts, load.

## Reading mistakes

`floe check` prints the mistakes one to a line, and the exit code is 1 when there is one. A data file is checked in stages ([templates.md](templates.md#the-order-of-the-checks)): every mistake of a stage is told at once, and a stage runs only when the one before it has none. So the shape is checked first:

```text
plan.yaml:5:5: additional properties 'colour' not allowed
plan.yaml:8:23: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'
```

and once that is fixed, the checks after it:

```text
plan.yaml:8:36: write the command without its prompt, which floe draws from the shell ("$ ls")
```

- `file:line:column:` is where the mistake is: the value to change, or the key that holds it. A mistake told with `file:` alone, no line, is one of the whole file: what it lacks at the top (`design.yaml: missing properties 'background', 'goals'`), or a check of the template's on the whole data file. Fix every mistake told, then run `floe check` again: it may tell those of the next stage.
- A mistake of the shape (`additional properties 'x' not allowed`, `missing property 'x'`, `got string, want array`) is the template's or the part's schema refusing the data: `floe template show <template> schema.json` and `floe part show <part> schema.json` print what they take.
- A mistake of a part, look, shared schema, theme or template of your own stops every page, whether it uses that one or not: the mistake is told at its own file (`~/.config/floe/parts/quote/schema.json:2:1: …`). `floe part check` and `floe template check` list them.
- A mistake told once is not told again, even if many places name it.

### Warnings

A page drawn may have warnings: advice on a better way to write it, which leaves the page drawn and the exit code as it is (0 when there is no mistake). They are printed after the mistakes, by `floe check`, `floe render`, `floe template check` and `floe publish`:

```text
plan.yaml:12:15: warning: this text holds a fenced block of code on its line 1, drawn as plain code: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button
```

- A fenced block of code in a `text` part, in a section (report's and its subsections, `more` of design, handoff, adr, postmortem and runbook, runbook's `text` of a step, and a slide of slides) or in `details` and `tabs`, is warned of: write it in `code`, or a command in `terminal`.
- A file with a mistake is not drawn, so its warnings come once its mistakes are fixed. Those of the other files drawn are told with the mistakes.
