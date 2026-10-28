# Writing pages

How to write a data file, the YAML that floe draws a page from. This is all you need to write pages with floe's template `report`; [templates.md](templates.md) tells how to make a template of your own, and [commands.md](commands.md) tells every command and flag.

## A data file

A data file names a template, and the template draws the page from its data. floe's template is `report`: a header over sections, each a heading over parts.

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
      - table:
          columns:
            - title: Risk
            - { title: Pages, num: true }
            - title: State
          rows:
            - [Broken links, '120', { pill: watching, tone: warn }]
      - note: { tone: warn, text: Edits during the freeze are lost. }
```

```sh
floe check plan.yaml     # prints its mistakes, or nothing
floe publish plan.yaml   # stores the page and prints its URL
```

Every data file holds these keys, which floe reads itself:

| Key | What it is |
|---|---|
| `template` | The template that draws the page (`report`, or one of your own). Required. |
| `lang` | The page's language (`<html lang>`), as a tag such as `en`, `ja` or `pt-BR`. Required. The words floe and the parts draw (a heading of `risks`, the zoom buttons of a diagram) and the text of a diagram follow it, English for a language they have no words of. |
| `title` | The page's title. |
| `theme` | The theme the page is drawn in (default `default`): its colours, faces and the shape of the parts. `floe template show <template>` lists the themes on its `themes:` line: `default`, `soft`, `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `swiss` and `washi`. |

The rest is the template's: `report` takes `eyebrow` (a short line above the title), `lead` (a sentence or two under the title) and `sections`, each a `heading` and `parts`. `floe template show <template>` prints what a template takes and an example to start from.

## Parts

Each part is one key naming its kind. `floe part list` lists them with a line each, and `floe part show <name>` prints what one takes (its schema, as written). The parts `report` takes in a section:

| Part | What it draws |
|---|---|
| `text` | Markdown, tables included; raw HTML in it is left out. A fenced block of code (```` ``` ````) is refused: write code in `code` and commands in `terminal`. An image file named in it is not shown, only a data URI of a png, gif, jpeg or webp: write images in `image`. |
| `code` | Code as it is, under its `file` with its `lang`. |
| `terminal` | Commands (`run`) and what they printed (`out`), at the prompt of its `shell` (`bash`, `sh`, `zsh`, `powershell`, `cmd`). floe draws the prompt, so a `run` that starts with one (`$ ls`) is refused. |
| `table` | `columns` and `rows`; each cell is a line of Markdown, a number in quotes as it is drawn, or a pill `{ pill, tone }`. |
| `image` | An image, such as a screenshot: its `file` and its `alt`, what it shows ([Images](#images)). |
| `cards`, `note`, `stats`, `progress`, `steps`, `checklist` | Cards, a note in a tone, large numbers, bars of progress, numbered steps, items ticked once done. |
| `compare`, `options`, `facts`, `diff`, `tree`, `links` | What changes before and after, options with pros and cons, names with values, a diff, files and folders, links. |
| `findings`, `decision`, `risks`, `matrix`, `fields` | What a review found, a decision, risks, a table of yes / part / no, the keys of a setting. |
| `timeline`, `bars`, `faq`, `glossary`, `keys` | Dates in order, bars of values, questions and answers, words and their meaning, keys to press. |
| `details`, `tabs` | Parts folded under a title, or shown one at a time under tabs; they hold `text`, `code`, `terminal`, `diff`, `note`, `table` and `image`. |
| `diagram` | A diagram, checked and drawn as inline SVG ([Diagrams](#diagrams)). |
| `script` | Something that moves ([Scripts and HTML](#scripts-and-html)). |
| `html` | HTML that goes in as it is ([Scripts and HTML](#scripts-and-html)). |

A `tone` is `accent`, `good`, `warn` or `bad`.

## Writing text

A data file is one YAML document of keys and values. One with `---` between documents, or with anchors, aliases, merge keys (`<<`) or tags, is refused.

- Write text with a `: `, a ` #` or, in `{ }` or `[ ]`, a `,` in it in quotes (`"…"`) or as a block (`|-`): YAML reads them as a key and its value, the start of a comment, or the end of the text.
- A comment on the line of a value not in quotes is refused, in every YAML file floe reads, as it may have cut the value short (`lead: PR #100` reads as `PR`): write the comment on a line of its own.
- A key without a value in `{ }` is refused too, as the rest of text a `,` ended (`{ label: 1,500 words }` reads as `label: 1` and a key `500 words`). A `,` in `[ ]` is not caught: `[1,500 words]` is two items.
- A file is UTF-8, without a byte order mark past its start.
- To change a data file from a program, read it with a YAML library, change the values and write it out: the library quotes the text that needs it. Set it to write no anchors or aliases, and to keep non-ASCII text as it is and the keys in their order (in PyYAML: `allow_unicode=True`, `sort_keys=False`, and a Dumper whose `ignore_aliases` returns true).

## Images

In `report`, an image is a part `image` (`- image: { file: shots/list.png, alt: The list of pages }`), in a section, `details` or `tabs`: its `file` is the path of the image, and its `alt` what it shows. A template of one's own takes an image as an item of `"format": "image"`, whose value is the path. The path is relative to the data file's directory and under it, without `..` or names starting with a dot. The image goes into the page as a data URI, so that the page holds all it shows: a png, jpeg, gif, webp or svg file, by its name, up to 1 MiB. `floe publish` sends the server the images the page shows.

An image file Markdown names (`![x](shots/list.png)`) is drawn as written, and the page's CSP does not show it; only a data URI of a png, gif, jpeg or webp written in it shows. Write images in `image`.

## Diagrams

A `diagram` part is a spec written in place: a canvas and the parts placed on it at absolute coordinates (the top left is 0, 0). It is checked before the page is drawn, its mistakes at the data file's lines and columns, and drawn as inline SVG that zooms. `floe diagram render` draws a spec of a file of its own the same way ([commands.md](commands.md#floe-diagram-render-specyaml)).

```yaml
- diagram:
    width: 640
    height: 200
    # read aloud by screen readers
    title: How the API is laid out
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, w: 200, h: 56, tone: accent, rows: [[ALB, main code], public entry] }
      - box: { name: api, x: 400, y: 72, w: 200, h: 56, rows: [API server, ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

- `example: <kind>` in place of the rest draws a built-in example (`floe diagram example` lists them). The page shows the diagram, not its spec: to read or copy an example's spec, print it with `floe diagram example <kind>`.
- Text (a `title`, a row, a label) is text, or its words in each language, English among them: `{ en: Store, ja: 保存先 }`. The page draws the words of its `lang:`, and the spec is laid out and checked once for every language it is written in.
- Each part is one key naming its kind. `floe part show diagram` prints every key; `floe diagram example <kind>` prints a spec of each kind, its comments explaining its grid.

| Part | Keys |
|---|---|
| `frame` | `x` `y` `w` `h`, `label`, `name`, `accent`, `square` (no rounded corners), `icon` (an icon such as `aws/group/region`: the AWS group style, its icon in the corner). Its label sits inside the top left corner, so place its contents 32 or more below its top (40 with an icon). |
| `box` | `x` `y`, `rows`, `w` `h` (as large as the rows need when left out), `name`, `origin` (`left`; `centre` for an `x` at the middle), `tone`, `shape` (`rect`; `pill`, `hex` for a decision, `state`). A row is text, or `[text, style]` with the style `main`, `sub`, `main code` or `sub code`; without a style the first row is `main` and the rest `sub`. |
| `table` | `x` `y`, `title`, `rows`, `w`, `name`, `origin`, `tone`. A row is `[name]`, `[name, type]`, `---` for a separator, or `{ name, cells: [name, type] }` for a row an edge can end at. |
| `icon` | `x` `y` (the top left), `icon`, `label` (below the icon), `size` (48), `name`. `icon` is the icon's set and its name in the set, like `aws/amazon-ec2` (see `floe icons search`). |
| `edge` | `from` and `to`, each the `name` of a part or a named row, or a point `[x, y]`; `fromSide` and `toSide` (`top`, `right`, `bottom`, `left`), `via` (points to turn at), `straight` (point to point, not right-angled), `label` with `at: [x, y]`, `dashed`, `bold`, `tone`, `start` (`none`), `end` (`arrow`), `crossing`. The ends are `arrow`, `triangle`, `diamond`, `odiamond`, `one`, `many`, `zero-one`, `zero-many` or `none`. `crossing: true` lets this edge cross others. |
| `dot` | `cx` `cy`, `r` (7), `name`, `tone`, `hollow`. Edges may reach its centre. |
| `bar` | `x1` `x2` `y`, `name`, `tone`, `label` (right of the bar). |
| `axis` | `x1` `x2` `y`, `ticks: [[x, text], ...]`, `grid` (the top of the grid lines). |
| `circle` | `cx` `cy` `r`, `name`, `tone`. Circles may overlap. |
| `label` | `text`, `x` `y`, `anchor` (`middle`; `start`, `end`), `plain` (no background), `strong`, `in` (the names of the circles of a Venn diagram it sits inside, outside the others; without `x` and `y`, floe places it). |
| `line` | `x1` `y1` `x2` `y2`, `kind` (`line`; `lifeline`, `divider`). It may cross other parts, but not leave the canvas. |
| `raw` | `svg`: SVG that no part can draw, of SVG's elements alone (no script or `foreignObject`), not laid out or checked. |

An edge without `via` and sides runs right-angled from the side of one end facing the other, with two turns at most; give `via` or a side to take another way.

The checks:

- Text too wide or too tall for its box (a frame's label for the frame's width).
- Parts (edges, lines and axes too) sticking out of the canvas.
- Boxes, marks, labels and frame labels overlapping each other, and boxes and marks straddling the edge of a frame. Frames are not checked against one another, so they may cross.
- Edges running through boxes, labels or frame labels, and edges crossing like an X (unless one has `crossing: true`).
- An edge ending at a name no part has, or at a side its end does not have.

## Scripts and HTML

- A `script` is `js`, a function the page calls with an empty element in its place and `data`, any value: `function (el, data) { … }`. A mapping in `data` comes in the order of its keys' names (write a list where the order matters), and the function runs where it stands, before the rest of the page is read. A `js` holding `</script` or `<!--` is refused; a mistake inside the function shows in the browser.
- An `html` part goes in as it is. It leaves no element open, nor closes one of the page around it (a `<b>` left open, a `</section>`), refused at the part: write each element whole. The page drawn holds no `<style>`, `<base>`, `<noscript>`, or `<meta>` but for its charset and viewport, whoever writes it, refused at the data file's `template:`: style the HTML with `style` attributes.
- The scripts of a page drawn from a data file cannot send what it holds out: `fetch`, XHR, images from elsewhere and forms are blocked by its CSP. Libraries from cdnjs, jsDelivr and unpkg, and Google Fonts, load.

## Reading mistakes

`floe check` prints the mistakes one to a line, and the exit code is 1 when there is one. A data file is checked in stages ([templates.md](templates.md#the-order-of-the-checks)): every mistake of a stage is told at once, and a stage runs only when the one before it has none. So the shape is checked first:

```text
plan.yaml:5:5: additional properties 'colour' not allowed
plan.yaml:8:23: value must be one of 'accent', 'good', 'warn', 'bad'
```

and once that is fixed, the checks after it:

```text
plan.yaml:8:36: write the command without its prompt, which floe draws from the shell ("$ ls")
```

- `file:line:column:` is where the mistake is: the value to change, or the key that holds it. Fix every mistake told, then run `floe check` again: it may tell those of the next stage.
- A mistake of the shape (`additional properties 'x' not allowed`, `missing property 'x'`, `got string, want array`) is the template's or the part's schema refusing the data: `floe template show <template>` and `floe part show <part>` print what they take.
- A mistake of a part, theme or template of your own stops every page, whether it uses that one or not: the mistake is told at its own file (`~/.config/floe/parts/quote/schema.json:2:1: …`). `floe part check` and `floe template check` list them.
- A mistake told once is not told again, even if many places name it.

## Checking and publishing

- `floe check <data.yaml>...` checks data files without writing anything or starting the server: run it after each change, and in a repository's CI.
- `floe publish <data.yaml>` stores the page as it was drawn and prints its URL; publishing again overwrites it. `floe publish --link <data.yaml>` instead draws the page from the file each time it is opened, and reloads the open tab when the file is saved, to read a page while it is written.
- `floe render <data.yaml>...` writes the page beside the data file, as one HTML file that opens without floe.

[commands.md](commands.md) tells their flags.
