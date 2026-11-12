# Themes

A theme is a directory of CSS files that changes a page's colours, typefaces and the look of the parts, by setting variables and writing rules. A data file selects a theme with `theme:`, as it selects one of floe's. This doc says what each file and variable changes, how a change takes effect, and what `floe theme check` refuses.

## Commands

| Command | What it does |
|---|---|
| `floe theme new <name>` | Makes the smallest theme in `FLOE_THEMES_DIR`: an empty `palette.css` to fill in. |
| `floe theme check` | Checks every theme of yours and the parts, looks and shared schemas of yours, as `floe part check` does (a theme's `<name>.css` needs its part). Prints every mistake; exit code 1 when there is one. |
| `floe theme list` | Lists the themes, floe's and yours. |
| `floe theme show <name>` | Lists the files the theme reads, floe's themes too. |
| `floe theme show <name> <file>` | Prints the file named. `floe theme show default palette.css` is floe's palette, which yours goes over. |

## Where themes are

- A theme is a directory in `FLOE_THEMES_DIR` (default `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`), holding the theme's files. A file directly in `FLOE_THEMES_DIR` is not read, and nothing says so.
- The name MUST be lower case letters, digits, `-` and `_`, starting with a letter or a digit, and MUST NOT be the name of one of floe's themes. A theme of your own cannot take the name of a built-in theme.
- A theme directory, and anything in it, MUST NOT be a link to a directory (a link to a file is read).
- `FLOE_THEMES_DIR` MUST NOT be, or be in, the directory of your parts or templates (`FLOE_PARTS_DIR`, `FLOE_TEMPLATES_DIR`): floe would read the files of one as those of the other.
- A mistake in any theme of yours stops every page, whether or not the page uses that theme, until it is fixed (`floe template guide`, *Where templates are*). A new theme made in another directory and moved in once `floe theme check` is clean stops none: set `FLOE_THEMES_DIR` to that directory for `floe theme new` and `floe theme check` (`floe template guide`, *Make one in another directory*).

## Files

A theme holds these files, each optional. A file or directory of any other name MUST NOT be in a theme: floe refuses it, a `<name>.css` of a name that no part or look has too.

| File | What it holds |
|---|---|
| `theme.yaml` | The key `description:` and no other key (floe refuses another): one line of text saying what the theme is (`description: 'Black on cream, flat primaries'`). `floe theme list` and `floe theme show <name>` print it, and the management page's settings show it under the theme's card as written. floe's own themes are described in the page's language. Without it the description is empty. |
| `palette.css` | The variables of the palette (*Variables*) and variables of names of your own, on `:scope`, the page's root. It goes over floe's `default` palette, so it writes only what it changes. Only `default`'s `palette.css` is read under another theme: `default`'s other files (its panel, rows, table and the like) are not. |
| `base.css` | Rules for the page around the parts. They reach the page, not what a part holds and not a text's Markdown, and sit under the template's `page.css` and the looks. The root is `:scope`: `:root` and `html` are outside the scope. What a part holds is styled in that part's file. |
| `markdown.css` | The look of Markdown (quotes, headings) wherever it stands: on the page, in a text and in what the parts hold. A rule for `p` or `ul` reaches a part's own `p` and `ul` too. Every part's, look's and template's CSS wins over it. |
| `<name>.css` | The style of the part or look `<name>`. `panel.css` reaches every panel, `code.css` code, a diff and a terminal. It is scoped as the part's `part.css` is: `floe part guide` (*The CSS layers and @scope*) says how. |

## Variables

A variable is set on the `:scope` of a file. The defaults below are floe's: `floe theme show default palette.css` prints the palette's, and `floe part show <part> part.css` a part's.

### Palette

Set in `palette.css`. A palette may set properties of the root too (`default` sets `color-scheme: light`).

| Variable | What it changes | Default |
|---|---|---|
| `--bg`, `--surface` | The page's background, a part's background | `#f4f3f0`, `#fff` |
| `--page-bg` | The whole page's background, a pattern too | `var(--bg)` |
| `--fg`, `--muted` | Text, faint text | `#22211f`, `#615f5a` |
| `--line` | Lines and borders | `#dad7d0` |
| `--rule` | The border of a rule across the page, such as between sections | `1px solid var(--line)` |
| `--quiet` | The quiet background within a box (a table's head, a bar's track, a keycap) | `color-mix(in srgb, var(--line) 45%, var(--surface))` |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale background: the colours of the tone `accent` | `#1d6670`, `#dcebec` |
| `--good`, `--warn`, `--bad` | The colours of the tones `good`, `warn`, `bad` | `#1d7542`, `#8a5a10`, `#b3261e` |
| `--good-soft`, `--warn-soft`, `--bad-soft`, `--muted-soft` | The pale backgrounds of those tones and of `muted` | `#dcefe3`, `#f6e8d2`, `#fbe5e2`, `color-mix(in srgb, var(--fg) 8%, var(--surface))` |
| `--on-fill` | The text on a box filled with `--accent` or a tone's colour | `var(--surface)` |
| `--accent-fill`, `--good-fill`, `--warn-fill`, `--bad-fill`, `--muted-fill` | A theme's own fill for a tone. floe sets `--tone-fill` from `--<tone>-fill`, else from the tone's `-soft` | not set |
| `--shadow` | A colour for shadows | `rgb(34 33 31 / 0.12)` |
| `--floe-shadow-x`, `--floe-shadow-y`, `--floe-shadow-blur`, `--floe-shadow-spread` | The theme's one shadow, in px or rem | not set: no shadow |
| `--radius`, `--radius-small`, `--radius-pill` | Round corners, one length each: a box's, a small box's within it, a pill's | `20px`, `6px`, `999px` |
| `--font`, `--display`, `--mono` | The typeface of text, of headings and of fixed-width text | system sans-serif stacks (`floe theme show default palette.css`); `--display` is `var(--font)` |
| `--code-bg`, `--code-fg`, `--code-prompt` | The background of code and a terminal (always dark), their text, and a terminal's prompt | `#1b2120`, `#e2e9e7`, `#b5e05a` |
| `--code-add`, `--code-del` | A diff's lines added and removed | `#6fcf97`, `#f2b8b5` |
| `--chart-1`, `--chart-2`, `--chart-3`, `--chart-4` | The colours of a chart's series, in order | `#008fa3`, `#c2660a`, `#6157d9`, `#c8366f` |
| `--text-width` | The width a page of one column grows to (a template sets its column to it by `--column-width: var(--text-width)`) | `56rem` |
| `--page-width`, `--page-pad` | The width of the column a page is centred in, and the pad each side of it | `96rem`, `16px` |

### floe's own variables

A theme sets these on the element that needs them, in any file (`--tone` and the two after it are read, not set).

| Variable | What it changes | Default |
|---|---|---|
| `--floe-shadow-color` | The colour of the shadow a part's box, a look's element or a quote casts (`.card { --floe-shadow-color: var(--fg); }`; a quote's in `markdown.css`). It reaches no element inside | `transparent` |
| `--floe-inset` | A shadow drawn within the box. `initial` clears it; `none` voids the box's shadow too | `0 0 #0000` |
| `--face`, `--on-face` | The background and the text colour of a box filled with colours of its own | not set (`initial` on what a part's box holds) |
| `--tone`, `--tone-soft`, `--tone-fill` | The colours of the tone of the element they are read in (`data-floe-tone`; accent's outside a tone). floe builds them from the palette's tone colours. They are read, not set: `background: var(--tone-soft); color: var(--tone)` | built from the palette |

### Part variables

A part reads its variables on its own root. A theme sets one on the root `:scope` of its file of the part or look that sets or reads it (`floe part show <part> part.css` prints it): a look's file reaches every part that takes the look (`marked.css` reaches `checklist` and `decision`), a part's file that part alone. floe's own themes set them nowhere else (floe's tests refuse any other place in floe's themes; `floe theme check` does not test yours). A variable a part sets on an element inside it from the data (chart's `--c`, code's `--digits`) is the part's own: a value on the root does not reach it.

| Variable | Set in | What it changes | Default |
|---|---|---|---|
| `--floe-mark` | The file of the part or look that draws the row | The size of a row's marker, its frame within | note `1.6rem`, numbered `2rem`, links `1lh`, marked `1.15rem`, timeline `0.7rem`, a card's number `1.75rem` (on `:scope.numbered`) |
| `--floe-mark-gap` | The same file | The gap between a row's marker and its text | note `0.85rem`, numbered `1rem`, links `0.25rem`, marked `0.3rem`, checklist `0.65rem`, timeline `0.65rem` |
| `--cards-gap` | `cards.css` | The space between what a card holds | `0.4rem` |
| `--checks-gap` | `checks.css` | The space between what an item of `checks` holds | `0.35rem` |
| `--entries-row-gap` | `entries.css` | The space between the lines of an entry's title and tags | `0.2rem` |
| `--table-head-bg`, `--table-head-fg` | `table.css` | A table head's background and text colour | `var(--quiet)`, `var(--muted)` |
| `--table-rule` | `table.css` | The rule between rows; `none` for none | `1px solid var(--line)` |
| `--tabs-picked-bg`, `--tabs-picked-fg`, `--tabs-picked-ring` | `tabs.css` | The selected tab's background, text colour and ring (a `box-shadow`) | `var(--accent-soft)`, `var(--accent)`, `none` |
| `--code-head-bg`, `--code-head-fg` | `code.css` | The heading of code, of a diff and of a terminal | `color-mix(in srgb, var(--code-fg) 5%, transparent)`, `var(--code-fg)` |
| `--code-head-add`, `--code-head-del` | `code.css` | A diff's counts on that heading | `var(--code-add)`, `var(--code-del)` |
| `--code-ground` | `code.css` | The background of the copy button | `var(--code-bg)` |
| `--chart-line`, `--chart-dash` | `chart.css` | A line's width and dashes | `2px`, `none` |
| `--chart-dot`, `--chart-dot-fill`, `--chart-dot-edge`, `--chart-dot-line` | `chart.css` | A dot's radius, fill, edge colour and the edge's width | `4px`, `currentColor`, `var(--surface)`, `2px` |
| `--chart-bar-fill`, `--chart-bar-edge`, `--chart-bar-line` | `chart.css` | A bar's fill, frame colour and the frame's width | `currentColor`, `none`, `0` |
| `--chart-ground` | `chart.css` | The background under the plot, for a box with a pattern that would run into its ticks | `none` |

## How a change takes effect

Each item gives the action, then the reason in parentheses.

### Page and layers

- Name `@keyframes` and `@font-face` with the theme's own name, `washi-fade` (the names are the page's: of two files that make one name the higher layer wins, the template's `page.css` over the theme's `base.css` and `markdown.css`, a theme's file of a part over them all).
- Expect a theme's file of a part to win over the part's `part.css` at equal specificity, and a more specific selector to win by that (it is in the layer of `part.css`, after it). `base.css` and `markdown.css` are in the layer `floe.theme`, under the template's `page.css`.

### Page and column

- Leave `--column-width` to the template's `page.css` (a theme that sets it in its palette widens the column only of a template that sets none: report, design, handoff and postmortem; adr's and runbook's `page.css` win over it; floe does not check it).

### Colours

- Set `--face` and `--on-face` on `:scope, :scope > *` to fill a part's root (floe's base sets both to `initial` on what a part's box holds, and draws the links, a line's code and the focus ring in a box in `--on-face`).
- Set `--quiet` on a box painted in a background close to it, as in `.card { --quiet: var(--surface); }` (`--quiet` melts into such a box).
- Name `--good-fill` and the like in `palette.css`, as in `:scope { --good-fill: #cfe8cf; }`, and draw with `var(--tone-fill)` (floe sets `--tone-fill` from `--<tone>-fill`, else from the tone's `-soft`; `--accent-fill` is for what has no tone).
- Make a box background that lets what is under it through opaque in a band, in `band.css` (a band is filled with the accent, which would show through and sink its text; `floe theme show aurora band.css` does it for glass).

### Shadows and space

- Give a shadow with `--floe-shadow-x`, `--floe-shadow-y`, `--floe-shadow-blur`, `--floe-shadow-spread` in `palette.css`, `--floe-shadow-color` on the element that casts it (`.card { --floe-shadow-color: var(--fg); }`) and `--floe-inset` for one within the box (floe's base writes `box-shadow` from them).
- Do not write `box-shadow` on a part's box, a look's element or a blockquote (it is written over floe's and drops the shadow and the inset).
- Keep what a box draws outside itself within `--floe-room` (0.75rem) (a shadow, a pseudo-element standing out or an outline that reaches further meets the next box or the page's edge; a shadow reaches blur + spread + its offset towards a side, and a tilt or a scale reaches further the bigger the box).
- Draw a decoration on a box that has a focus ring or a selected option (the cards of `options`) with `--floe-inset` or `::before`, as in `.card { --floe-inset: inset 0 0 0 2px var(--surface); }` (the ring is an `outline`, and takes the place of one a theme writes).
- Leave the space between parts to `--floe-room` (floe's base sets it on `:root`, 0.75rem; a `gap` on the root of `cards.css` or `tabs.css` changes it).
- Do not write a word in a `content` string; use symbols (CSS text does not pass `words.yaml`, so a word stays in one language whatever the page's).

### Rows: note, numbered, timeline, links, marked

- Size a row's marker (the symbol of a note, the ring of numbered, the dot of timeline, the sign of links, the letters of `marked`, which `checklist` takes) with `--floe-mark` and `--floe-mark-gap`, and hide it with `content: none` (its column then takes no space).
- Do not set `display`, `gap` or `grid-template` on a row with a marker (floe lays the marker out in the layer `floe.marks`, under a theme's file of a part, and such a rule wins over it without a word).

### cards

- Draw a card in `panel.css` and write in `cards.css` what differs, the background, the frame, the number (a card takes the panel too).
- Change the space between what a card holds with `--cards-gap`, not a `gap` on the card (`options` holding parts, whose rows have no gap, lose a `gap` without a word).
- Size a card's number with `--floe-mark` on `:scope.numbered`, hide it with `content: none` on `.numbered > .card::before`, and draw it as plain text with `--floe-mark: auto` and no background.

### table

- Set the head's background with `--table-head-bg` (it is painted on the head's row, so a gradient runs once across it; a head of a gradient or of layers of background, placed and sized, cannot be a `--face`).
- Write a heading's rule and its type (typeface, size, weight) on `:scope thead th` in `table.css`.

### tabs

- Set the selected tab with `--tabs-picked-bg`, `--tabs-picked-fg` and `--tabs-picked-ring` (a part paints it on screen alone; on paper every tab shows under its title).
- Expect the selected tab in a box filled with colours of its own (a band and the like) to take the box's two colours swapped, `--on-face` as its background and `--face` as its text: `--tabs-picked-*` do not apply there (`tabs/part.css` sets `--tabs-face-bg` and `--tabs-face-fg` from them).
- Set the shadow you give every tab title as the ring too, to have it on the selected tab (the ring is drawn over that shadow).

### code

- Set the colours of a diff's counts with `--code-head-add` and `--code-head-del` when you lighten code's heading (the counts are drawn on it).
- Set `--code-ground` to the same colour as code's lines when you paint them a background of your own on `:scope pre` (it is the copy button's background).
- Write a heading's `color` knowing a terminal's heading takes it (a terminal's heading is fainter, at 85% of `--code-head-fg`, whatever colour a theme gives it).
- Write a heading's rule and its type on `:scope figcaption` in `code.css`.

### chart

- Keep a line to 3px, a dot's radius to 5px, and a dot's edge or a bar's frame to 2px (the labels are kept clear of these sizes).
- Expect a bar too thin for its frame to draw in `currentColor` with no frame, and every bar of a chart narrower than 34rem to ignore `--chart-bar-fill` and `--chart-bar-edge` (a frame there would be all of one).
- Take the series' colour as `currentColor`. A theme's chart variables take no space.

### Corners

- Give each of `--radius`, `--radius-small` and `--radius-pill` one length (four corners, or a `/`, voids a rule that reads it as one corner or inside `calc()` or `min()`).
- Make a pill that wraps round no more than `--floe-pill-end` (`0.75lh`) with `min(var(--radius-pill), var(--floe-pill-end))`.
- Name corners of other shapes with variables of your own, as in `:scope { --hand: 14px 6px 12px 8px / 8px 12px 6px 14px; }`.

## What `floe theme check` refuses

`floe theme check` does what `floe part check` does, so parts and themes are checked together. It prints every mistake and exits with code 1 when there is one. For a theme it refuses:

- a theme directory whose name breaks the naming rule or is the name of one of floe's themes, or that is a link to a directory, or a link to a directory in a theme, or `FLOE_THEMES_DIR` overlapping the directory of your parts or templates;
- a file or a directory of a name floe does not read (*Files*), and a file floe cannot read (a link with no target, a file without permission, a FIFO or a device);
- a `theme.yaml` that is not keys and values, a key other than `description:`, or a `description:` that is not text;
- in a CSS file: CSS floe cannot read (a block, string, comment or `url()` left open at the end of the file, a `}` that closes no block, a string that runs past its line), an `@import`, an escape (`\`) outside a string, a `url()` or a comment, or a `url()` that is not one address or one quoted string alone (`url("a.png" x)`, `url(a b)`).

If `theme.yaml` has a mistake, a theme is a link to a directory, or a file cannot be read, the check prints that alone: mend it and run the check again for the rest.

The check tests CSS syntax, not values or places. A part variable set where its part does not read it (in `base.css`, in the file of another part, on a state, in nested CSS, in a list of selectors, in `@keyframes`, with `!important`), or text that sinks into its background, passes with exit code 0.
