# Diagrams

A diagram is drawn from a spec: a canvas, and the parts placed on it. A page draws it as inline SVG, after floe has checked it. This doc says what a spec holds, what each part takes, how floe lays out a `graph`, how an edge written by hand runs, and what floe refuses.

## The spec

- The part `diagram` holds a spec. It stands in a section's `parts`, and in the `parts` of `details`, `tabs`, `columns` and of an item of `entries`, `checks` and `options`. `floe part show diagram schema.json` prints every key it takes.
- A spec in a file of its own holds what a `diagram:` item holds (the keys written under `- diagram:` in the example below): `title`, `parts` and so on at the top of the file, with no `diagram:` key, no `template` and no `lang`. `floe guide` tells how to write the data file that holds a `- diagram:` item. `floe diagram render <file>` draws it as a page draws it (`floe diagram render --help`). `floe diagram inspect <file>` checks a spec, or the diagrams of a data file, and prints where each part went, without drawing.
- floe checks a spec before the page is drawn, and tells a mistake at the data file's line and column.
- The SVG zooms where it is drawn smaller than its actual size.

```yaml
- diagram:
    width: 640
    height: 200
    # read aloud by screen readers
    title: 'How the API is laid out'
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, tone: accent, rows: [[ALB, 'main code'], 'public entry'] }
      - box: { name: api, x: 400, y: 72, rows: ['API server', ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

| Key | Takes | Default | What it does |
|---|---|---|---|
| `title` | text, required | — | What the diagram shows, which a reader who cannot see it is told |
| `parts` | a list, required | — | The parts, each written as one key naming its kind (*The parts*) |
| `width` | a number, 1 to 10,000,000 | as far right as what is drawn reaches, and 20 more | The canvas's width |
| `height` | a number, 1 to 10,000,000 | as far down as what is drawn reaches, and 20 more | The canvas's height |
| `example` | a kind that `floe diagram example` lists | — | Draws a built-in example, in the page's language. It MUST be the only key: floe refuses it beside `title` or any other |

- The coordinates MUST be within a million either way (the top left is 0, 0), a size of a part within a million, and a canvas within ten million.
- The canvas's top left stays 0, 0 when `width` or `height` is left out, so that the coordinates are as written. `raw` SVG is not measured: write `width` and `height` with it, or it may be cut off.
- A page shows the diagram of an `example`, not its spec. To read or copy an example's spec, print it with `floe diagram example <kind>`, whose comments explain its grid.
- Text (a `title`, a row, a label) is text, or its words in each language: `{ en: Store, ja: 保存先 }`. The words MUST include `en`. The page draws the words of its `lang:`, and floe lays out and checks the spec once for every language it is written in.
- A `title` MUST NOT be empty in any language, and neither MUST the text of a label: every `label` key (a graph's edges and groups among them) and the `text` of the part `label`. For no label, leave the key out.
- A diagram paints its own background, the page's `--bg`, in its frame, and what hides the lines under it (a label's box, a hollow end or dot) is painted that background: on a panel or a theme's patterned page, the diagram stands as a box of the page's background.

## Hand or graph

- Write by hand a diagram read as a picture, such as an architecture or layout diagram, where the arrangement in space matters. You decide where each part goes, so you can keep flows straight, align related parts and place each label on its edge. The example above is written by hand.
- Write a `graph` for a flow, a state machine, or a diagram that changes often or joins many things. You write what joins what, and floe decides where everything stands; adding to it takes no coordinates (*Graph*).
- A diagram written by hand takes more work to change. Moving a part means moving what copies where it stands: the frame round it, and the `via` or `at` of its edges. Writing in an edge only the turns you choose avoids copying them (*Edges written by hand*); a frame is moved by hand.
- A graph usually takes more space than the same diagram written by hand. A diagram wider than the page shrinks to fit, until it is zoomed. Try both `direction`s: one often comes out far better than the other for the same spec.

## Placing parts by hand

Run `floe diagram inspect` on the data file after every few parts. It prints where each part went (`box "alb" 240,90 144x29`), and each refusal at the line to change. Place the next part by the sizes it prints.

- Leave `w` and `h` out to let floe size a box to its text; write them when boxes must align. A `w` too small is refused with the one it needs: `box "a": "A long title here" needs w: <n> or more`.
- Leave these spaces:

| Between | Leave at least | Where to read it |
|---|---|---|
| The boxes of a horizontal edge that has a label | The label's width | The `label` row of `inspect`: `label "HTTPS" 80,5.5 46x18` is 46 wide |
| The boxes of a vertical edge that has a label | The label's height, 18 | The same row |
| The line of a vertical edge and the left of the canvas | Half the label's width, as a label stands across the line, centred on it | The same row |
| A frame's contents and its heading (and its `icon`, if it has one) | Stand below the lower end of the heading and of the icon | The rows `the heading of frame` and `the icon of frame` of `inspect` |

- Write `at` for the label of an edge that crosses the side of a frame: floe does not check a label without `at` against a frame, so a label that stands on the frame's line is not refused.
- To fix a label that sticks out on the left (`label "HTTPS-LONG-LABEL" sticks out of the <w>x<h> canvas`), move the line to the right or shorten the label: a `width` only widens the canvas to the right.
- Write an edge as `{ from, to }` and let floe route it. Use `via`, `fromSide` or `toSide` only when you need to control its path (*Edges written by hand*). floe refuses an edge with a box between its ends (`edge "a"→"c" runs through box "b"; move one of them, or go round it by via`): move a box, or write `via`.

## Cloud icons

Draw each service of AWS, Google Cloud or Azure that a diagram shows, in an architecture or a flow or any other diagram, as its cloud's icon. Do not draw a box around an icon. floe ships no icons: it uses each cloud's official icon set, kept once on each machine.

| Set | How it is kept | Names |
|---|---|---|
| `aws` | `floe icons fetch aws` (about 14MB, from aws.amazon.com) | `aws/amazon-rds`, `aws/res/…` (resources), `aws/group/…` (the AWS group icons) |
| `gcp` | `floe icons fetch gcp` (two zips, about 1MB, from services.google.com) | `gcp/cloud-run` (a core product), `gcp/category/networking` (a product category). Draw a product without an icon of its own with its category's icon, and its own name as the label (Google's guide) |
| `azure` | `floe icons fetch azure <zip>`, with the zip the user downloads (step 1) | `azure/virtual-machine` |

1. Keep the set of the cloud you draw. Without it, `floe check`, `floe diagram render` and `floe diagram inspect` refuse a diagram that holds one of its icons (`inspect` prints where the parts went first): `floe: AWS icons are not in <dir>/aws; run "floe icons fetch aws"…`. Azure icons need the user's agreement to Microsoft's terms, so ask the user to open `https://learn.microsoft.com/azure/architecture/icons/`, agree, download the zip, and tell you where it is. Then run `floe icons fetch azure <zip>` with that path. The zip may be anywhere, and may be removed afterwards (floe keeps the icons it holds in a directory of its own). Until then, draw an Azure service as a box, publish, and tell the user; change it to its icon once the set is kept.
2. Find the name with `floe icons search <set> <word>`, naming the set of the cloud you draw: it lists the names of the set's icons that hold the word, each with its group. `floe icons search aws alb` lists `aws/res/elastic-load-balancing-application-load-balancer`, and `floe icons search gcp storage` lists `gcp/category/storage` and `gcp/cloud-storage`. A short name such as `alb`, `gke` or `vnet` works. Write the name as it is printed, with its set in front: floe refuses `amazon-ec2` and an icon that is not in the set.
3. Write the icon as an `icon` part with `x`, `y` and its name in `icon`, and join icons by edges that name their `name`s. Or write it as an `icon` node of a `graph`, which floe places (`floe diagram example aws-graph` is one). A frame takes an `icon` of its own in its corner, which may be any icon of a set: `floe icons search aws group` lists the AWS group icons (`aws/group/region`, `aws/group/virtual-private-cloud-vpc`), a service's icon such as `aws/aws-step-functions` works too, and `azure/resource-groups` and `azure/virtual-networks` frame an Azure resource group and virtual network.

```yaml
- diagram:
    title: 'A web app on AWS'
    parts:
      - icon: { name: alb, x: 40, y: 40, icon: aws/res/elastic-load-balancing-application-load-balancer, label: ALB }
      - icon: { name: api, x: 240, y: 40, icon: aws/amazon-elastic-container-service, label: ECS }
      - icon: { name: db, x: 440, y: 40, icon: aws/amazon-rds, label: RDS }
      - edge: { from: alb, to: api, label: HTTPS }
      - edge: { from: api, to: db, label: SQL }
```

## The parts

Each part is one key naming its kind. floe refuses a key that no part takes and a required key that is missing. A part's `name` is the name an edge's `from` and `to` (or a graph's `with`) gives: it MUST be unique among the names of the parts, rows and nodes of the diagram.

`tone` is one of `accent`, `good`, `warn`, `bad` and `muted`.

### frame

A box around other parts. Its label sits inside the top left corner.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` `w` `h` | numbers, required | — | The top left, and the width and height |
| `label` | text | none | The heading |
| `name` | a name | none | The name an edge can end at |
| `accent` | `true`, `false` | `false` | Draws the frame in the accent colour |
| `square` | `true`, `false` | `false` | No rounded corners |
| `icon` | an icon of a set, such as `aws/group/region` | none | The icon stands in the corner and the label right of it. Any icon of a set works (*Cloud icons*) |

Place a frame's contents at or below the bottom edge of its heading and of its icon (*Placing parts by hand*).

### box

A box of centred rows of text.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` | numbers, required | — | The top left; with `origin: centre`, `x` is the middle |
| `rows` | a list of rows, required | — | The text (below) |
| `w` `h` | sizes | as large as the rows need | The width and height |
| `name` | a name | none | The name an edge can end at |
| `origin` | `left`, `centre` | `left` | What `x` is: the left edge, or the middle |
| `tone` | a tone | none | The colour |
| `shape` | `rect`, `pill`, `hex`, `state` | `rect` | The outline: `pill` has rounded ends, `hex` is for a decision, `state` is for a state of a state machine |

A row is text, or `[text, style]` with the style `main`, `sub`, `main code` or `sub code`. Without a style, the first row is `main` and the rest are `sub`.

### table

A heading over rows of a name and a type (an ER entity, a class).

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` | numbers, required | — | The top left; with `origin: centre`, `x` is the middle |
| `title` | text, required | — | The heading |
| `rows` | a list, required | — | The rows (below) |
| `w` | a size | as wide as the rows need | The width |
| `name` | a name | none | The name an edge can end at |
| `origin` | `left`, `centre` | `left` | What `x` is |
| `tone` | a tone | none | The colour |

A row is `[name]`, `[name, type]`, `---` for a separator (written as an item of the list: `- ---`), or `{ name, cells: [name, type] }` for a row an edge can end at, by the row's `name`. An edge takes the left or the right side of a row. A row's `name` is a name like any other, so rows of two tables that read alike (`id`) need names that differ (`c_id`, `o_id`).

The heading is 30 tall, a row 22 and a separator 9, and 6 more stand under the last row: a table of a heading and two rows is 80 tall. An edge ends at the middle of its row, and `floe diagram inspect` prints the path of the edge.

### icon

An icon of an icon set (*Cloud icons*).

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` | numbers, required | — | The top left |
| `icon` | the set and the name in it, required | — | Like `aws/amazon-ec2` or `gcp/cloud-run` (`floe icons search` finds a name) |
| `label` | text | none | Centred below the icon, 3 under it and 18 tall; an edge at the icon's bottom side ends under the label |
| `size` | a size | 48 | The icon's width and height |
| `name` | a name | none | The name an edge can end at |

### edge

A line from one end to another, right-angled through `via`, or straight from point to point with `straight`.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `from` `to` | the `name` of a part, of a named row or of a node of a `tree`; or a point `[x, y]`; required | — | The ends |
| `fromSide` `toSide` | `top`, `right`, `bottom`, `left` | chosen by floe (*Edges written by hand*) | The side an end takes |
| `via` | a list of points | none | Points to turn at: `[x, y]`, or one axis, `{ x: 710 }` or `{ y: 218 }` (*Edges written by hand*) |
| `straight` | `true`, `false` | `false` | Point to point, not right-angled |
| `label` | text | none | The words on the edge |
| `at` | `[x, y]` | the middle of the edge's longest run | The middle of the label. A label stands on its line and cuts it: write `at` a little above the line to keep the line whole |
| `dashed` `bold` | `true`, `false` | `false` | The line's look |
| `tone` | a tone | none | The colour |
| `start` `end` | `arrow`, `triangle`, `diamond`, `odiamond`, `one`, `many`, `zero-one`, `zero-many`, `none` | `start` is `none`, `end` is `arrow` | The marker at an end. `one`, `many`, `zero-one` and `zero-many` are the ends of an ER diagram's lines, `many` a crow's foot |
| `crossing` | `true`, `false` | `false` | Lets this edge cross others |

### dot

A circle (a commit, a milestone, a start state).

| Key | Takes | Default | What it does |
|---|---|---|---|
| `cx` `cy` | numbers, required | — | The centre |
| `r` | a size | 7 | The radius |
| `name` | a name | none | The name an edge can end at |
| `tone` | a tone | none | The colour |
| `hollow` | `true`, `false` | `false` | A ring with the background inside |

An edge that names a dot is drawn to its outline, towards the edge's first point off its centre, with the tip of its arrow (or other marker) on the outline. It is drawn to the centre when that run is too short for the marker (or no longer than the radius, with none), between two dots that touch or overlap, and when the end is written as a point at the centre (name the dot). floe checks the edge, and places its label, as one to the centre.

### bar

A horizontal span (a timeline period), its label to the right.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x1` `x2` `y` | numbers, required | — | The ends and the height |
| `name` | a name | none | The name an edge can end at |
| `tone` | a tone | none | The colour |
| `label` | text | none | Right of the bar. With `width` left out the canvas reaches the label; with a `width` written, the label MUST fit in it |

### axis

A time axis with labelled ticks, and grid lines.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x1` `x2` `y` | numbers, required | — | The ends and the height |
| `ticks` | a list of `[x, text]`, required | — | The ticks. A tick of empty text (`""`) is a mark alone, not a label |
| `grid` | a number | none | The top of the grid lines |
| `labels` | `at`, `between` | `at` | `at`: a tick's text at the tick. `between`: halfway between the tick and the next tick on its right, or `x2` past the last |

### circle

A translucent circle, for a Venn diagram. Circles may overlap.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `cx` `cy` `r` | numbers, required | — | The centre and the radius |
| `name` | a name | none | The name a `label`'s `in` gives |
| `tone` | a tone | none | The colour |

### label

Text at `x`, `y`, or in the area of a Venn diagram.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `text` | text, required | — | The words |
| `x` `y` | numbers | — | Where the text stands. Required without `in`; with `in`, both or neither |
| `anchor` | `middle`, `start`, `end` | `middle` | What `x` is. Only with `x` and `y` |
| `plain` | `true`, `false` | `false`; `true` for a label with `in` | No background. `false` gives a label with `in` one |
| `strong` | `true`, `false` | `false` | Bold |
| `in` | the `name`s of circles | none | Stands inside those circles and outside the others. Without `x` and `y`, floe places it |

### line

A line without arrows. It may cross other parts, but not leave the canvas.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x1` `y1` `x2` `y2` | numbers, required | — | The ends |
| `kind` | `line`, `lifeline`, `divider` | `line` | The look: `lifeline` is the dashed line down from a participant of a sequence diagram, and `divider` a dashed line across the diagram that marks a change of scene |

### tree

A tree of boxes. floe places each node under its parent, the children side by side and each depth in one row, and draws an edge down from each node to each child.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` | numbers, required | — | The root's centre and top |
| `root` | a node, required | — | The root |

#### A node of a tree

| Key | Takes | Default | What it does |
|---|---|---|---|
| `rows` | a list of rows, required | — | The text, as a `box` has it |
| `name` | a name | none | The name an edge written by hand can end at |
| `tone` | a tone | none | The colour |
| `shape` | `rect`, `pill`, `hex`, `state` | `rect` | The outline, as a `box` has it |
| `children` | a list of nodes | none | The nodes under this one |

When an edge written by hand ends at a node of a tree, write `fromSide` or `toSide` on a side no edge of the tree ends at. Without it, the edge may run over the tree's edges, which floe does not tell.

### graph

Boxes, icons and groups that floe lays out in ranks (*Graph*).

| Key | Takes | Default | What it does |
|---|---|---|---|
| `x` `y` | numbers, required | — | The top left of all of it |
| `nodes` | a list of nodes | — | The boxes, icons and groups. Required unless `lanes` is written; floe refuses both |
| `lanes` | a list of groups | — | In place of `nodes`: lanes (*Groups and lanes*) |
| `edges` | a list of edges | none | The edges between the nodes |
| `direction` | `down`, `right` | `down` | Which way the edges run |

A node is one key naming its kind, `box`, `icon` or `group`.

#### A box of a graph

| Key | Takes | Default | What it does |
|---|---|---|---|
| `name` | a name, required | — | The name an edge's `from` and `to`, and a `with`, give |
| `rows` | a list of rows, required | — | The text, as a `box` has it |
| `with` | the name of a box or icon of the graph | none | Stands this box in the rank of that one (*Ranks*) |
| `tone` | a tone | none | The colour |
| `shape` | `rect`, `pill`, `hex`, `state` | `rect` | The outline, as a `box` has it |

#### An icon of a graph

An icon with its label below and no box round it.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `name` | a name, required | — | The name an edge's `from` and `to`, and a `with`, give |
| `icon` | the set and the name in it, required | — | Like `aws/amazon-ec2` |
| `with` | the name of a box or icon of the graph | none | Stands this icon in the rank of that one (*Ranks*) |
| `label` | text | none | Below the icon |

#### A group of a graph

Boxes, icons and groups framed together.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `nodes` | a list of nodes, required | — | What the group holds |
| `name` | a name | none | The name an edge's `from` and `to` give |
| `label` | text | none | The heading, in the frame's top left corner |
| `accent` | `true`, `false` | `false` | As a frame's |
| `icon` | an icon of a set, such as `aws/group/region` | none | The icon in the frame's corner, the label right of it. Any icon of a set works (*Cloud icons*) |

#### An edge of a graph

An edge joins two nodes of the graph by name. floe refuses a name that no box, icon or group of the graph has.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `from` `to` | the names of boxes, icons or groups, required | — | The ends |
| `label` | text | none | The words on the edge |
| `dashed` `bold` | `true`, `false` | `false` | The line's look |
| `tone` | a tone | none | The colour |
| `level` | `true`, `false` | `false` | `true` stands its two boxes in one rank (*Level edges*) |
| `start` `end` | `arrow`, `none` | `start` is `none`, `end` is `arrow` | The marker at an end |

### raw

SVG that no part can draw, of SVG's elements alone, not laid out or checked.

| Key | Takes | Default | What it does |
|---|---|---|---|
| `svg` | text, required | — | The SVG |

- The SVG MUST NOT hold `<script>`, `<foreignObject>`, HTML elements (`<div>`, say), or elements a page refuses (`<style>`, `<base>`, `<noscript>`, `<meta>`): floe refuses it.
- Colour the SVG with the variables that `floe theme show default palette.css` defines (the CSS of the palette), as attributes too (`fill="var(--accent-soft)" stroke="var(--accent)"`). These are the colours to take:

| Variable | What it is |
|---|---|
| `--bg`, `--surface` | The page's background, a part's background |
| `--fg`, `--muted` | Text, faint text |
| `--line` | Lines and borders |
| `--quiet` | A quiet background within a box |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale background |
| `--good`, `--warn`, `--bad` | Good, warning and bad |
| `--good-soft`, `--warn-soft`, `--bad-soft`, `--muted-soft` | The pale backgrounds of good, warning, bad and muted |
| `--on-fill` | Text on a shape filled with `--accent` or a tone's colour |

- No other colour has a soft one.
- Do not take the faces, sizes or shadows that palette.css holds (`fill="var(--font)"` is drawn black, as a font is not a colour), nor a variable that a theme alone sets (`--accent-fill` and the other `--*-fill`, which palette.css does not define: drawn black).
- For a tone, write `data-floe-tone` on the element, or on a `<g>` around it within the raw SVG, and read `var(--tone)`, `var(--tone-soft)` and, for a fill, `var(--tone-fill)`: the diagram's root clears any tone from outside it.
- Give text a colour and a size, as the parts' text is 12 to 13: `<text fill="currentColor" font-size="12">` (`currentColor` is `--fg`).
- Do not write a part's classes, `data-floe-look` or `data-floe-part` on an element: floe's own styles reach it (`class="edge"` draws no fill and a grey line).
- floe does not tell these, and draws them black: a variable that is not there, a tone that is not there (`var(--tone)` with no tone), and text without `fill` (at the page's size).

## Graph

A `graph` specifies only how its boxes, icons and groups are joined, and floe places them:

```yaml
- diagram:
    title: 'How a page is published'
    parts:
      - graph:
          x: 20
          y: 20
          nodes:
            - box: { name: write, rows: ['Write the data'] }
            - box: { name: check, shape: hex, rows: ['Checks pass?'] }
            - box: { name: show, rows: ['Show the page'] }
          edges:
            - { from: write, to: check }
            - { from: check, to: show, label: 'yes' }
            - { from: check, to: write, label: 'no' }
```

### Ranks

- Each box or icon goes a rank below the boxes and icons its edges come from, and an edge that closes a cycle is drawn back against the ranks (upward with `down`, to the left with `right`), as written. Two things stand a box or icon in the rank of another: the two ends of an edge written `level` (*Level edges*), and `with`, which names the box or icon to stand with.
- A group has no rank of its own: its frame spans the ranks of what it holds. An edge to or from a group puts the end it enters at least a rank below the end it leaves, and a group counts as its whole span: `x → group` puts the group below `x`, and `group → y` puts `y` below the whole group.
- Where an edge to or from a group would make a cycle, floe draws the edge back against the ranks: the end the edge leaves then stands at least a rank below the end it enters. Where that makes a cycle too, floe may refuse the edge (*Groups and lanes*).
- floe orders the boxes and icons of a rank so that fewer edges cross, and keeps the order written where none cross. The same spec is always laid out the same way.
- `with: <name>` on a box or an icon stands it in the rank of the box or icon named, without drawing an edge. The two may be in different groups. Write it for a box or icon that should not go below what reaches it (a gateway that servers use, say), or that should not start a group's frame in the first rank. Edges keep the way they mean: `api → nat`, with `with: alb` on `nat`, runs up from `api`.

  ```yaml
  - diagram:
      title: 'A NAT gateway beside the load balancer, below the servers that use it'
      parts:
        - graph:
            x: 20
            y: 20
            nodes:
              - icon: { name: alb, icon: aws/res/elastic-load-balancing-application-load-balancer, label: ALB }
              - icon: { name: api, icon: aws/amazon-elastic-container-service, label: ECS }
              - icon: { name: nat, with: alb, icon: aws/res/amazon-vpc-nat-gateway, label: NAT }
            edges:
              - { from: alb, to: api }
              - { from: api, to: nat }
  ```

- `with` MUST name a box or icon of the same graph.
- `with` takes only one kind of edge out of the ranks: an edge from a box or an icon into the one that has `with`, which runs up or down as the ranks fall.
- An edge into or out of boxes or icons that `with` or a level edge stands in one rank may be refused; the refusal names the edge. Drop `with`, or write the edge by hand beside the graph (*With parts written by hand*).
- Other edges still decide ranks, and move the box or icon `with` names too: an edge from a group, an edge into the other end of a `level` edge, and an edge out of the one that has `with`. When they make a cycle, one edge of it is drawn back up, the way it is written. The order the boxes and icons are written in decides which, so write them in another order for another.
- `with` joins boxes or icons that no edge joins, and they are laid out together in one rank. A box or icon written between them, with no edge to either, stands after them, not between them. Boxes and icons joined by edges keep their order.

### Where edges end

- On a `box`, edges leave and enter at separate points of its side, so that no two share a run. A `box` widens (with `right`, grows taller) for many edges.
- Edges of one graph may cross one another. An edge of a graph that crosses another part is checked as any edge.
- An icon stays 48 square however many edges reach it. Do not put more than three edges on one side of an icon: four arrowheads touch at their bases, and five or more overlap.

### Labels

- A label stands on its edge, on the diagram's background, cutting the line. A graph with labels is larger than one without, so that its boxes and icons stand elsewhere.
- After you add a label to a graph, run `floe diagram inspect` again and move the parts written by hand beside the graph: its boxes and icons have moved, and floe tells a part written by hand that comes to overlap them, at that part.
- Do not draw a frame written by hand across the edge labels of a graph: floe does not check a label against it, and the label cuts the frame's line.

### Groups and lanes

- A `group` keeps its boxes and icons (and its groups) side by side in every rank, with nothing else between. floe draws a frame around them as a `frame` part, its `label` in the top left corner and `accent` as a frame's.
- The frame holds the group's boxes and icons and the edges between them, and no other box or icon. An edge between a box or icon inside and one outside crosses the frame's side.
- An edge with both ends outside a group goes round its frame where the ranks would take it through, which makes the edge longer. floe does not refuse an edge that still runs through a frame: look at the drawn diagram (`floe diagram render`), and change the group or the order of the nodes if one does.
- An edge between two lanes crosses the lanes between them.
- A group's `name` names the frame: an edge written by hand may end at it.
- An edge between a group and what it holds, from a group to itself, or to a group from a box, icon or group that stands neither above it nor below it (as one that `with` puts among its ranks does), may be refused. So may an edge to a box or icon of the group from one that `with` or a level edge stands in one rank with (*Ranks*). The refusal names the edge and says what to change: end the edge at a box or icon of the group, or drop `with`.
- An edge MUST NOT end at a lane, which no edge can reach from outside its ranks: end it at a box, an icon or a group in the lane.

### Level edges

- An edge written `level: true` stands its two boxes or icons in one rank and runs straight between their facing sides.
- Its two boxes or icons MUST be of one group (or of none), or directly in two lanes.
- Of the two, the one written first in `nodes` stands on the left (with `right`, on top), whichever way the edge runs. The boxes and icons that level edges join in one group stand side by side as written.
- A box or icon directly in a lane that has a level edge to another lane stands at its lane's end towards it (at its start when it has edges both ways).
- A level edge at a group, one with another box or icon standing between its two boxes or icons in their rank, one at the side of a loop, and an edge between two boxes or icons that level edges stand in one rank, may be refused. The refusal names the edge and says what to move or drop.
- Laid out `right`, a lane's heading reaches into the first rank. A level edge that runs through it may be refused: shorten the heading.
- A loop written `level` is a loop: its box or icon stands in one rank already.

### Loops

- An edge with the same box or icon in `from` and `to` is a loop. floe draws it out of the right side of the box or icon (with `right`, the bottom), and loops do not change how the graph is laid out.
- An edge that goes round a group's frame across the side the loops go out of may fail the diagram as floe's own failure (`room made for the loops of box <n> is not clear`): take the loops or the group off.

### With parts written by hand

- An edge that cannot be written `level` can be written by hand beside the graph: one between two zones that are each a group, say, or between two boxes or icons that `with` stands in one rank. It moves nothing of the graph, so its ends stay in the ranks their other edges give them:

  ```yaml
  - diagram:
      title: 'A copy between two zones, an edge written by hand beside the graph'
      parts:
        - graph:
            x: 20
            y: 20
            nodes:
              - group:
                  name: zone-a
                  label: 'Zone A'
                  nodes:
                    - box: { name: writer, rows: [Writer] }
              - group:
                  name: zone-b
                  label: 'Zone B'
                  nodes:
                    - box: { name: reader, rows: [Reader] }
            edges:
              - { from: zone-a, to: zone-b }
        - edge: { from: writer, to: reader, label: copy, fromSide: right, toSide: right, via: [{ x: 190 }] }
  ```
- A box or an icon of a graph, and a group's or a lane's frame, can be the end of an edge written by hand, which ends on a side as any edge written by hand does (*Sides of an end*).
- Run `floe diagram inspect` on the graph alone first: it prints where each box stands and the path of each edge, from which you read the sides the graph's edges end at and where to place the parts written by hand.
- When an edge written by hand ends at a box or icon of a graph, write `fromSide` or `toSide` on a side no edge of the graph ends at. Without it, the edge may run over the graph's edges, which floe does not tell, or cross the loops of the box or icon, which floe refuses.
- floe tells a mistake between a part that a graph or a tree laid out and one written by hand (an overlap, an edge through a box, a crossing) at the one written by hand, which can be moved.

### Limits

- A graph MUST NOT hold more than 200 boxes, icons, groups and lanes (counted together through its groups and lanes) or more than 400 edges, and the graphs of a diagram MUST NOT pass more than 4000 ranks between their edges' ends, all told. Split the graph, or the diagram, when floe refuses it.

## Edges written by hand

An edge without `via` and sides runs right-angled from the side of one end facing the other, with two turns at most; give `via` or a side to take another way. An edge from an icon to a part that stands beside it and reaches below the icon's label may leave the icon's bottom and run back through the label: floe refuses it (`runs through label "RDS"`). Write `fromSide: right`, or move the other part below the label.

### via

Write in `via` only what you choose of an edge's way, and `at` only when the middle of the edge's longest run, where the label stands without it, will not do. floe adds a turn between points off line and enters the last side square, so the corners it adds need no writing, and a number copied from where a part stands needs rewriting when the part moves. A point may give one axis, `{ x: 710 }` or `{ y: 218 }`: the other axis is taken from the point before it, and for the first point from the centre of the edge's start (the point itself, the centre of a dot, the middle of a part or a row). `{ x: 710, y: 218 }` is `[710, 218]`.

```yaml
- diagram:
    title: 'Edges that turn where they are told'
    parts:
      - box: { name: api-a, x: 40, y: 80, rows: ['API A'] }
      - box: { name: api-c, x: 40, y: 180, rows: ['API C'] }
      - box: { name: queue, x: 400, y: 80, rows: [Queue] }
      - box: { name: writer, x: 400, y: 120, rows: [Writer] }
      - edge: { from: api-a, to: queue, fromSide: top, toSide: left, via: [{ y: 20 }, { x: 300 }] } # M80,80 V20 H300 V94.5 H400
      - edge: { from: api-c, to: writer, fromSide: right, toSide: left, via: [{ x: 300 }] }         # M120,194.5 H300 V134.5 H400
```

Write the first point on the axis the edge leaves by: `{ y }` from a top or a bottom side, `{ x }` from a left or a right one. `fromSide: top, via: [{ x: 150 }]` takes the height of the box's centre and is refused as running through the box; `via: [{ y: 10 }, { x: 150 }]` leaves upward and then runs across.

Some points cannot be left out: without them the edge runs another way, and nothing tells you, since floe cannot know the way you wanted.

- The first point of an edge with no `fromSide`: it chooses the side the edge starts from. Write `fromSide`, or keep the point on one axis (`{ y: 10 }`).
- The first point of an edge leaving a point or a dot upward or downward: they leave across. Keep the point on one axis (`{ y: 120 }`).
- A turn the other way than the edge came: a turn floe adds goes on the way the edge came first, so after a run down, a corner that goes across first has to be written.
- The only point left in an edge's `via`: with no `via`, floe chooses the way and the edge shares the side with the others. Keep a `via` to stay in the middle of the side.

### Ends that share a side

- Edges written by hand between names, without `via` or `straight`, that share a side are spread along it by floe.
- A label written (with `at`) where an edge of a side would be spread to keeps the edges of that side together in the middle: move the label to part them.
- The ends of an edge with `via`, at a point or `straight` stay in the middle of the side. To branch edges from one point, write `via` or use a `tree`.
- Two edges that go to one name, or come from one, can share a label only where they run on the same line.
- If floe moves one of the two off the label, it tells the edge left on it: write `via` on the edge floe moves to keep it where it is.
- floe tells a label over a point of the `via` of its own edge: take that point out or move the label, as a `via` round it will not help.
- A dot, a row and a bar keep their edges in the middle.
- If floe tells a label with no `at` as a mistake, write its `at` or move what is in its way.
- An edge that runs through a frame's `label` is told: go round it by `via`.

### Sides of an end

- An end written with a side takes that side. A table's row takes left or right alone, and a dot takes no side.
- Write the `via` outside the side an edge ends at. Otherwise the edge goes on as it was going, and may run along the outline of the end, which floe does not tell, or through the end, which it refuses.

## What floe refuses

floe refuses a diagram that holds any of these, mostly at the line of the part to change. A refusal of the whole diagram (the ranks of a graph, icons not kept) may have no line. A refusal of the shape of the spec (a key that is missing, one no part takes, a value out of range) is told in the words of the JSON Schema library.

- Text too wide or too tall for its box (a frame's label for the frame's width).
- A part (an edge, a line or an axis too) sticking out of the canvas: left of or above 0, 0, or past a `width` or `height` written.
- A box, a mark or a label overlapping another box, mark or label, or a frame's label (`box "a" overlaps the heading of frame "F"`) or a frame's icon (`the icon of frame "G" overlaps box "a"`). floe checks a label by its box, 5 wider each side than its text, with no background too.
- A box, a mark or a label straddling the edge of a frame (`box "a" straddles the edge of frame "F" at x <n>, spanning <n> to <n>`). The labels checked are the labels written where they stand (a label at an `x` and `y`, an edge's label at `at`), and the labels of a part that may not straddle a frame itself (an icon's, by hand or in a `graph`, and a bar's). These labels are not checked against frames: an edge's with no `at` (*Placing parts by hand*), an axis's, a `graph` edge's, and a label in its region (`in` with no `x` and `y`).
- An edge running through a box, a label or a frame's label. A box is the shape drawn: an edge may pass a hex's points and a rounded corner, and run along its outline or within 1 of it.
- Two edges crossing like an X, unless one has `crossing: true`, or both are edges of one `graph`.
- An edge ending at a name no part has, a name that no edge can end at (a circle, say), a side its end does not have, or the same point at both ends.
- Two parts, rows or nodes of one name.
- A `label` in a Venn diagram that names a circle that is none, that stands where it is not all inside its circles and outside the others, or that has no space inside them: make the area larger or the text shorter.
- A `raw` SVG that holds an element a diagram may not hold.
- An icon named without its set, or not in the set (*Cloud icons*).
- An `example` that is not one of the kinds `floe diagram example` lists.
- The refusals of a graph (*Graph*): a name that no box, icon or group of it has, an edge that ends at a lane or joins a group with what it holds, `with` and `level` mistakes, a graph past its limits, and graphs that overlap, cross or straddle one another or stand off the canvas.

A straddle is told once for each edge crossed, with where that edge stands and where the part spans on that axis, both in the diagram's numbers (`at x <n>, spanning <n> to <n>`). To mend it, change one of these:

- what places the part: its `x` and `y`, a dot's `cx` and `cy`, a bar's `x1`, `x2` and `y`, or an edge label's `at` (an icon's label goes with its icon, a bar's with its bar);
- the `x` and `y` of the whole `graph` or `tree`, for what it lays out;
- the frame.

Frames are not checked against one another, so they may cross. A label or a box across the edge of one that another crosses is refused all the same.

A mistake names a part in this order:

1. by its `name`;
2. with none, by its text in quotes, in the language whose text is the widest;
3. with no text either, by where it stands, its top left and size (`frame 10,10 250x150`).

An edge names its two ends in the same way, a point by where it stands: `edge "a"→"b"`, `edge "a"→[60,120]`, and an edge of a `tree` its two nodes as their boxes are named (`edge "a"→"B"`). A frame's corner icon is `the icon of` its frame, and its heading `the heading of` its frame (`the icon of frame "F"`). `floe diagram inspect` names parts alike, and writes where each stands after its name; the diagram itself is named by its title.
