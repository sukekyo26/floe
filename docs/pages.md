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

`floe template list` describes each, with the keys of its data file (its shape; for a `-`, read `floe template show <template> schema.json`), and `floe template show <template>` prints its keys, each described in its `schema.json`, and an `example.yaml` to start from: an item of `"format": "markdown"`, or described as Markdown, is Markdown. What a page shows beyond its keys:

- `report` takes `eyebrow` (a short line above the title) and `lead` (a sentence or two under it), both lines of Markdown: what looks like a tag (`<prefix>`, `Vec<u8>`) is left out and marks such as `*` and `_` are read, so write those as text in backticks (for a strikethrough, see [Writing text](#writing-text)); a bare URL or email address becomes a link, but not a URL of an IP address or localhost (`http://127.0.0.1:7325`); an image file named in them is not shown, only a data URI of a png, gif, jpeg or webp. It also takes `sections`, each a `heading` over `parts`, and `sections` of its own after the parts: subsections, each a `heading` (an `<h3>`) over `parts`, one level only. A section with subsections has a heading; a heading is never empty, so a section with none leaves the key out. A heading is a line of Markdown, as `lead` is, and its id is made of the words drawn: what looks like a tag (`Vec<u8>`) is left out, as in `lead`, so write it in backticks, and a heading of a tag alone stands out of the contents; a tag left out, a mark read (`__init__`), a backslash or a character reference (`&copy;`) and a link's address change it from the words written. `toc` sets whether the page holds its contents, the sections with a heading and their subsections, beside the sections on a screen 1000px wide or more (then the page takes the whole window) and over them, the sections alone, on a narrower one; left out, a page of 4 sections with a heading or more holds them. They are not on paper. Their links go to `#section.0`, `#section.0.1` and on, by the sections' order: link to a section from elsewhere by its heading's id, which stays as before (`#1-what-happens`). In CSS write such an id as `[id="section.0"]`, as `#section.0` reads `.0` as a class.
- The keys that are not Markdown are drawn as written: design's `status`, `owner` and `date`; handoff's `date`, `from`, `to`, `ref` and `who`; the part `asks`' `by`; adr's `number`, `date` and `deciders`; postmortem's `date`, `authors`, `owner` and `ref`; runbook's `owner`, `updated` and a step's `who`; and the heading of a section of `more`.
- `handoff` heads the page with the counts of what is done, in progress, to decide (the questions with no `decided`) and next, and draws `asks` by the part `asks`.
- `runbook` draws a step's `who` as a badge beside its title, its `run` with the copy button of `terminal`, and `rollback` in a frame of its own.
- `slides` shows each slide as tall or wide as the screen with its number and the count in the corner, and prints each on a page of its own, sideways. Its parts are as large as in `report`: zoom in with the browser to make them larger. With `direction: across`, a mouse's wheel scrolls down only, and in the management page's frame the ← → keys work once the frame is clicked.

## Parts

Each part is one key naming its kind. `floe part list` lists them with a line each and the keys each takes (its shape), and `floe part show <name> schema.json` prints the rest (its schema, as written): what each key takes, and the shape a `-` stands for. `html` takes HTML as text and has no schema.json: `floe part show html` shows it.

To list things, pick the part by what the rows are:

- One line each: a Markdown list in `text`.
- A name and its longer text, which a table squeezes into a narrow column, such as requirements, risks and what is done about them, or terms: `entries`, a state in a tag.
- A decision: `decision` for one told with its reasons and the options set aside, `options` for options weighed side by side with their pros and cons, `asks` for questions put to the reader, each with its options, what is recommended and, once decided, what was, and `entries` for a run of short decisions, the pick in a tag.
- What was checked and what came of it, a survey's or a test's: `checks`, each with ok, ng or unchecked.
- How numbers move, or numbers compared: `chart`, as a line or as upright bars.
- What happened in order: `timeline`.
- Words and what they mean: a Markdown list, or `entries` when a meaning is long.
- A thing before and after: `compare`.
- A few short values at the top of a page: `facts`.
- Rows compared both along and across, each cell short: `table`.

The parts `report` takes in a section:

| Part | What it draws |
|---|---|
| `text` | Markdown, tables included; raw HTML in it is left out. Strikethrough as in [Writing text](#writing-text). A fenced block of code (```` ``` ````) is drawn as plain code and warned of: write code in `code` and commands in `terminal`. An image file named in it is not shown, only a data URI of a png, gif, jpeg or webp: write images in `image`. |
| `code` | Code as it is, under its `title` (what it shows, in words) and its `file` (the file it is of), with its `lang`. `start` numbers the lines from that number; `mark` marks lines by those numbers, or by their count from 1 without `start`. A number no line has marks nothing. A button over it copies the code, without the numbers. Draw no diagram or table in it with characters: write a `diagram`, `table` or `tree`. |
| `terminal` | Commands (`run`) and what they printed (`out`), at the prompt of its `shell` (`bash`, `sh`, `zsh`, `powershell`, `cmd`). floe draws the prompt, so a `run` that starts with one (`$ ls`) is refused. A button over them copies the commands, a line each, without the prompt or what they printed. |
| `table` | Rows compared both along and across, each cell short. `columns` and `rows`. A column is its title, or `{ id, title, num, pills }`: `num: true` for numbers, and `pills`, words with their tones (`{ done: good }`), for a cell of the column that is one of them, whole, to be drawn as that pill. A row is its cells in the order of the columns, or its cells by the `id`s of the columns (`{ state: done, who: walter }`): a column left out is empty, a key that is no column's `id` is refused, and a table may hold both. A cell is a line of Markdown, a number in quotes as it is drawn, or a pill `{ pill, tone }`, written as an object, not in quotes. |
| `entries` | Records one under another, each a `title` (a line of Markdown) with its `tags` beside it (each as a table's cell: a line of Markdown or a pill), its `text` (Markdown) the full width under them, and its `parts` under that, the parts `details` holds. A tag shows no column's name, so say in it what it is (`merged 2026-10-30`). Write code in `parts`, as `code` and `terminal`; a fenced block in `text` is drawn as plain code and warned of. |
| `image` | An image, such as a screenshot: its `file`, its `alt`, what it shows, and a `caption` under it if you like ([Images](#images)). |
| `note`, `checklist` | A note in a tone, items ticked once done. |
| `steps` | Numbered steps, each a `title` with its `tags` under it (each as a table's cell: a line of Markdown or a pill), its `text` (Markdown) and its `state`. Say in a tag what it is (`PR #428`), and name a step in one by what stays when the steps move (`after #428`, not `after 1`): the rings are numbered anew, a tag's words stay as written. |
| `stats` | Large numbers side by side, each a `label`, a `value`, a `unit` and a `note` in its `tone`. With `from`, the number before, it is drawn as `3.1 → 1.3` with the change under it, as `-1.8` and for the better or the worse in its colour, or the same with no number: write `better: lower` or `higher` with it, which way is for the better, and `from` and `value` as digits alone in quotes, as `'-3.5'` (no `1,234`, as the pattern the mistake quotes says). The change has as many decimals as the longer of the two, every digit of it, and no percentage, which a `note` may say. |
| `compare`, `options`, `facts`, `diff`, `tree`, `links` | What changes before and after, options with pros and cons (each may hold `parts` under its title, the parts `details` holds, such as a picture, a diagram or code of the option, the cards on a line starting their parts and their text at one height), names with values, a diff (a button over it copies the diff as written), files and folders, links. |
| `asks` | Questions put to the reader, numbered Q1, Q2 and on: a `question`, its `options`, the one to `recommend` and `why`, who decides (`by`), and what was `decided`. An option written the same as `recommend` or `decided` takes a ring and a pill; one that matches no option stands on a line of its own. A question decided has a round check at its top right and a ring on its decision alone; its other options are quiet, the one recommended keeping its pill. |
| `checks` | What was checked and what came of it, in a survey or a test, in the order written, the count of each result on top. Each has a `title`, a `result` of `ok`, `ng` or `unchecked` (drawn in the page's language, such as Pass, Fail and Not checked in English), the `actual` result (Markdown), `parts` under it (what `details` holds, such as `code`), and `how` it was checked. `actual` says why for `unchecked`. For what is still to do, write `checklist`; for what a review found, `findings`. |
| `chart` | Numbers over a row of names, as a line (`kind: line`) for how a number moves or as upright bars (`kind: bar`) for comparing them, with the numbers in a folded table. It takes a `title`, a `unit`, the names `x` in quotes, and at most 4 `series`, each a `name` and `values`, a number for each of `x`. The scale starts at 0; a line may write `min` to start it there instead, and with a value below `min` it is not used and the scale takes in 0 and every value. Bars take no `min`, which is refused as `false schema`. Text keeps the page's size at every width: in a box under 34rem a line's end says its value alone. A name of `x` past 8rem is cut with …, and a number past 9 digits is rounded or written with an exponent; the table and the tooltip on hovering hold them whole. A tick's word says the value of its line: ticks that 5 digits could not tell apart or say draw without words. The legend names every series, one too. |
| `findings`, `decision`, `fields` | What a review found, a decision, the keys of a setting. |
| `timeline`, `bars`, `keys` | Dates in order, bars of values, keys to press. A bar with `of` is filled as far as its `value` goes of that whole and reads `5 / 8`, for how far a piece of work has gone; the others are shares of the largest value among them. |
| `details`, `tabs` | Parts folded under a title, or shown one at a time under tabs, for things read one at a time; they hold every part but `tabs`, `columns` and `html`: a form in tabs' form would be dropped, columns would hold columns, and an `html` part's `</form>` would close tabs' form. A part laid out in cards, keys, a timeline and bars lays out by the width of its place, not the window's, and the pills of `asks` wrap there. A `script` in a closed `details` or a tab not shown runs with an element of no width. |
| `columns` | Parts side by side, to see at once: a screen before and after, data beside its reading. Each column is an optional `title` over `parts`, which hold what `details` holds. Columns are of one width, 16rem at the least, so that how many stand side by side follows the width of the page's text (five at most in `report`, three in `adr` and `runbook`), and the rest fold under them. |
| `diagram` | A diagram, checked and drawn as inline SVG ([diagrams.md](diagrams.md)). |
| `script` | Something that moves ([Scripts and HTML](#scripts-and-html)). |
| `html` | HTML that goes in as it is: a key of `any.json`, not a part of its own, so it has no schema.json; `floe part show html` prints `any.json`, which holds it ([Scripts and HTML](#scripts-and-html)). |

A `tone` is `accent`, `good`, `warn`, `bad` or `muted`.

## Writing text

A data file is one YAML document of keys and values. One with `---` between documents, or with anchors, aliases, merge keys (`<<`) or tags, is refused.

- In Markdown, `~~text~~` is struck through and a single `~` stays text (`2026-11-01~11-08`, `~/work`), in every key floe draws as Markdown: a text, a table's cell, a heading, a lead.
- Write text with a `: `, a ` #` or, in `{ }` or `[ ]`, a `,` in it in quotes (`"…"`) or as a block (`|-`): YAML reads them as a key and its value, the start of a comment, or the end of the text. Unquoted, such text is refused in YAML's own words, which change with what follows the `: ` (`mapping value is not allowed in this context`, ``'`' is a reserved character``), at the start of the value: look in that value for a `: `.
- A comment on the line of a value not in quotes is refused, in every YAML file floe reads, as it may have cut the value short (`lead: PR #100` reads as `PR`): write the comment on a line of its own, outside any `{ }` or `[ ]`.
- A comment can't be written in `{ }` or `[ ]`, even on a line of its own, in any YAML file floe reads: write it on a line outside them.
- A `|` or `>` text whose first line (not of spaces alone) begins with spaces and a tab is refused, in every YAML file floe reads: YAML 1.2 reads the tab as text, but floe's YAML library as indentation, and it drops or shortens the lines after it. Begin the line without the tab, or write the text's indentation after the `|` or `>` (`|2` for text two spaces past its key).
- A key without a value in `{ }` is refused too, as the rest of text a `,` ended (`{ label: 1,500 words }` reads as `label: 1` and a key `500 words`). A `,` in `[ ]` is not caught: `[1,500 words]` is two items.
- A file is UTF-8, without a byte order mark past its start.
- To change a data file from a program, read it with a YAML library, change the values and write it out: the library quotes the text that needs it. Set it to write no anchors or aliases, and to keep non-ASCII text as it is and the keys in their order (in PyYAML: `allow_unicode=True`, `sort_keys=False`, and a Dumper whose `ignore_aliases` returns true).

## Images

In `report`, an image is a part `image` (`- image: { file: shots/list.png, alt: The list of pages }`), in a section or subsection, `details`, `tabs`, `columns` or the `parts` of an item of `entries`, `checks` or `options`: its `file` is the path of the image, and its `alt` what it shows (for a screen reader, and where it cannot be shown). A `caption` is shown under the image (a line of Markdown: `caption: Figure 1: the list of pages`). An image drawn smaller than its size, to fit the column, zooms as a diagram does: with the toolbar over it, Ctrl / ⌘ + wheel, a pinch or the keys `+ - 0`, and moves while zoomed by a drag or the arrow keys; the toolbar shows its buttons alone; the frame tells this in its tooltip, in a line under it while the keyboard is on it, and as its description. An svg with a `viewBox` alone, without `width` and `height`, has no size to zoom from, and does not zoom. A template of one's own takes an image as an item of `"format": "image"`, whose value is the path. The path is relative to the data file's directory and under it: one `/` between names, without `./`, `..`, or a `/` at either end. The image goes into the page as a data URI, so that the page holds all it shows: a png, jpeg, gif, webp or svg file, by its name, up to 1 MiB. `floe publish` sends the server the images the page shows.

An image file Markdown names (`![x](shots/list.png)`) is drawn as written, and the page's CSP does not show it; only a data URI of a png, gif, jpeg or webp written in it shows. Write images in `image`.

## Diagrams

A `diagram` part, in a section or in `details`, `tabs`, `columns` and the `parts` of an item of `entries`, `checks` or `options`, draws a diagram from a spec written in place, checked before the page is drawn: [diagrams.md](diagrams.md) tells how a spec is written.

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
- A mistake of YAML itself (`mapping value is not allowed in this context`) is told in the words of floe's YAML library, at the place it names, which for a `: ` in a value not in quotes is the start of the value: see [Writing text](#writing-text).
- A mistake of the shape (`additional properties 'x' not allowed`, `missing property 'x'`, `got string, want array`) is the template's or the part's schema refusing the data: `floe template show <template> schema.json` and `floe part show <part> schema.json` print what they take.
- A mistake of a part, look, shared schema, theme or template of your own stops every page, whether it uses that one or not: the mistake is told at its own file (`~/.config/floe/parts/quote/schema.json:2:1: …`). `floe part check` and `floe template check` list them.
- A mistake told once is not told again, even if many places name it.

A page holds at most 512 elements open at once, counting `<html>`, `<body>` and what its template and parts open. Past that, the page is refused with `html: open stack of elements exceeds 512 nodes`, at the data file alone, since Markdown, an `html` part, a part or `page.html` may each have gone that deep, or at the line and column of a diagram's `raw` that goes that deep by itself: nest less.

### Warnings

A page drawn may have warnings: advice on a better way to write it, which leaves the page drawn and the exit code as it is (0 when there is no mistake). They are printed after the mistakes, by `floe check`, `floe render`, `floe template check` and `floe publish`:

```text
plan.yaml:12:15: warning: this text holds a fenced block of code on its line 1, drawn as plain code: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button
```

- A fenced block of code in a `text` part, in a section (report's and its subsections, `more` of design, handoff, adr, postmortem and runbook, runbook's `text` of a step, and a slide of slides) or in `details`, `tabs`, `columns` and the `parts` of an item of `entries`, `checks` or `options`, in the `text` of an item of `entries` or `options`, and in the `actual` of an item of `checks`, is warned of: write it in `code`, or a command in `terminal`.
- A file with a mistake is not drawn, so its warnings come once its mistakes are fixed. Those of the other files drawn are told with the mistakes.
