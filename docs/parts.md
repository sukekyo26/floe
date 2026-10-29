# Parts and themes

How to make a part, a look or a theme of your own. A part draws an item of a data file; a theme sets a page's colours, faces and the shape of the parts. To use parts in a template, see [templates.md](templates.md#parts-in-a-template); to write them in a data file, see [pages.md](pages.md#parts).

## Where parts and themes are

- Parts, looks and shared schemas of your own are in `FLOE_PARTS_DIR` (default `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`): a part or a look is a directory, a shared schema a file `<name>.json` beside them. Another file right there is not read, and nothing says so: put a part's files in its directory.
- Themes of your own are in `FLOE_THEMES_DIR` (default `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`), a directory each. A file right there is not read, and nothing says so: put a theme's files in its directory.
- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit. A name floe's parts, looks, shared schemas or themes have is refused: yours sit beside floe's, and never take their place. A part or look is not named `base` or `palette`, the names of a theme's files for every page.
- A directory in them may not be a symlink; a file may.

```sh
floe part new quote     # makes the smallest part, quote, to fill in
floe part check         # checks every part, look, shared schema and theme of your own
floe part show quote    # prints its looks, schema and checks
```

A mistake of a part, theme or template of your own stops every page drawn (`floe check`, `floe render`, `floe publish`), whether it uses that one or not, so that none goes unseen: `floe part check` and `floe template check` list them all. While you write one, make it in another directory and move it in once it is checked, as [templates.md](templates.md#make-one-in-another-directory) tells: set `FLOE_PARTS_DIR` or `FLOE_THEMES_DIR` to it and run `floe part check`.

## A part's files

```text
quote/
├── schema.json    # what a data file may write of it (JSON Schema)
├── part.html      # how it is drawn, a Go html/template
├── part.css       # its style (may be empty)
├── part.yaml      # the looks it takes (optional)
├── checks.yaml    # relations between its items (optional), as a template's
└── words.yaml     # its words, by language (optional), as a template's
```

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- A schema names it as floe's are named, `"$ref": "floe:///parts/quote.json"`; its own schema names floe's parts and the shared schemas so too.
- Its element is its box, which names it: `data-floe-part="quote"`.
- `part.html` sees the item as `.`, and the functions of a template's `page.html` but `asset` ([templates.md](templates.md#pagehtml)).
- It has the functions floe's parts draw with too. floe's parts, in floe's repository (https://github.com/sukekyo26/floe/tree/develop/internal/kit/floe/parts), show them in their `part.html`:
  - `{{copyButton}}` draws the copy button, in the page's language. Put it right under the part's box (`<figure data-floe-part="quote">…{{copyButton}}</figure>`): what it copies is found from the button up, by the nearest box of a part, so that in the box of a part inside yours it copies that part's, not yours. It copies the text of each element of the part marked `data-floe-copy`, each ending in a line break (code marks its `<code>`; a terminal, each command). Its look is floe's; the part's CSS places it and keeps text clear of it, as code's does: `isolation: isolate` on the part's box, so that the button stays under the page outside; `margin: var(--floe-copy-gap)` around it; and, where text lies under it, `padding-inline-end: var(--floe-copy-room)` after the longest line, under `:scope:has(> .floe-copy)`, so that a line scrolled to its end ends left of the button. floe sets `--floe-copy-size` (the button's size), `--floe-copy-gap` and `--floe-copy-room` on the page's root, and applies none of them to a part.
  - `{{diagram}}` takes nothing and draws the part's whole value as a diagram. The value is a spec as the part `diagram` takes it, nothing else: a key beside the spec (a `caption`) is refused by the diagram's schema.
  - `{{script .js .data}}` draws an element, and a script that calls `.js`, a function, with it and `.data`, as the part `script` does.
  - `{{$d := diff .text}}` reads a diff as `git diff` writes it: `$d.Lines`, each with `.Kind` and `.Text`; and `$d.Added` and `$d.Removed`, how many `add` and `del` lines it has. `.Kind` is `hunk` for a line starting with `@@`; `meta` for one starting with `\` (in a hunk too), for one starting with `--- ` or `+++ ` outside a hunk, and for any other line not starting with `+`, `-` or a space (`diff --git`, `index`), which also ends a hunk; `add` and `del` for the other lines starting with `+` and `-`, outside a hunk too; `""` for a line starting with a space, or empty.
  - `{{range bars .}}` reads a list of items of `label`, `value` and `tone`, a `value` that is not a number, or is below 0, being a mistake told at the part's value: each comes as `.Label`, `.Tone`, `.Value` as written and `.Width`, its value as a percentage of the largest. To have such a mistake told at the value itself, refuse it in the part's schema with both `"type": "number"` and `"minimum": 0`, as `bars`'s does: `minimum` alone lets a value that is not a number through to `bars`.
  - `{{range tree .}}` reads a list of items of `path` (a folder's ending in `/`), `note` and `change`, and nests them into the folders the paths name, a folder not listed of its own too: each comes as `.Name`, `.Note`, `.Change`, `.Dir` (a folder) and `.Children`.
- Its schema takes floe's `format`s as a template's does (`html`, `markdown`, `image`), declared under `properties` or `items`, where floe follows, or as the root, and a shared schema that holds them. Such an item comes to `part.html` drawn: `html` as written, `markdown` as HTML, `image` as a data URI (`<img src="{{.file}}" alt="{{.alt}}">`). Write it as `{{.h}}`: do not pass it to a function (`markdown`, `markdownLine`, `printf`, `slice`, `eq`, …) nor put it in a `<script>`, as it is HTML, not the text written, and does not work there. An item of `html` is checked in the page drawn as a template's is ([templates.md](templates.md)). A plain string drawn as Markdown is `{{markdown .text}}`. A value of `any.json` does not come drawn as one item but as a map of every kind ([Looks and shared schemas](#looks-and-shared-schemas)).
- `floe part show <name>` prints what one takes, floe's too: copy from floe's parts what fits.

## Looks and shared schemas

- A look alone is a directory of a `part.css` (and a `part.yaml` with its `description:` and the looks it takes), without a schema: it draws nothing, and lends its style to parts and elements that take it. floe's looks alone are `panel` and `rows`.
- A part takes the looks of others by its `part.yaml`, `looks: [panel]`, and the looks those take in turn, each before it, which it adjusts.
- A shared schema is a `<name>.json` a template's or a part's schema names (`floe:///parts/<name>.json`), always at its root. floe's are `tone.json` (`accent`, `good`, `warn`, `bad`) and `any.json` (a part of any kind). Its schemas are checked as a template's are, from its own root: a part or a format of floe's it holds is drawn where `properties` and `items` reach it from there, in a template's page or a part's alike.
- A value of `any.json` comes to `part.html` as it does to a template's `page.html`, a map of every kind: the kind written holds the part drawn, the others nil. `{{.x}}` writes the map as text; draw it with `{{range .x}}{{.}}{{end}}`, or `{{range .}}{{.}}{{end}}` when the schema's root is `any.json` ([templates.md](templates.md#parts-in-a-template) tells how to draw one kind alone).

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in five cascade layers, each later one winning over those before it whatever the selectors (`floe.parts` the strongest):

1. `floe.palette`: the palettes, floe's `default` and then the theme's.
2. `floe.base`: floe's base, the page's ground, colour and face on its root, and text, lists, links, code and quotes.
3. `floe.looks`: the looks of elements that take one without being a part's box (`data-floe-look`).
4. `floe.page`: the template's `page.css`.
5. `floe.parts`: the parts' style, each part after the looks it takes, then the theme's `base.css` and its files of the parts.

A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.

As in `page.css`, a `}` that closes no block and an `@import` are refused at their line.

## Borrowing a look

- An element that is no part's box takes a look by `data-floe-look`: `<section data-floe-look="panel">` is a panel in every theme. It has it in `floe.looks`, under the parts and the template, so that the part or template it is in adjusts it.
- floe's parts take looks so: `compare`, `fields`, `findings`, `matrix` and `risks` take the table's, `stats`, `options`, `facts`, `links` and `decision` the cards', `diff` and `terminal` code's, `checklist`, `faq`, `glossary` and `keys` the look `rows`, and the other parts drawn in a panel `panel`'s. A theme's rules for a look draw every part that takes it.

## Themes

A theme of your own is a directory of these files, each left out if not needed:

| File | What it holds |
|---|---|
| `palette.css` | The variables of [the palette](templates.md#the-palette), and those of names of its own, on `:root`. It goes over floe's `default` palette, so it writes only what it changes. |
| `base.css` | Rules for every page, such as Markdown's quotes. |
| `<name>.css` | The style of the part or look `<name>`, scoped as its `part.css` is: `panel.css` reaches every panel, `code.css` code, a diff and a terminal. |

- A file of another name is refused.
- Write a colour once, in `palette.css`, and take it with `var()` (`var(--accent)`) in the other files, so that the template's colours on `:scope` and the palette agree.
- A data file picks the theme with `theme:`, as one of floe's.

## Checking

`floe part check` reads every part, look, shared schema and theme of your own over floe's own, and prints every mistake of them (exit code 1): of their files, their names, the looks they take, a part that takes its own look, a path of a `checks.yaml` that its schema does not declare, the schemas of parts and shared schemas, and their CSS. A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.
