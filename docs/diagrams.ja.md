# 図

`diagram` の部品は、節にも `details`・`tabs` の中にも置ける、その場に書く spec です。キャンバスと、その上に絶対座標（左上が 0, 0）で置く部品から成ります。ページを描く前に確かめ（誤りはデータファイルの行と列で言う）、拡大できる inline SVG で描きます。`floe diagram render` は、ファイルに書いた spec を同じように描きます（[commands.ja.md](commands.ja.md#floe-diagram-render-specyaml)）。

```yaml
- diagram:
    width: 640
    height: 200
    # スクリーンリーダーが読み上げる
    title: API の配置
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, w: 200, h: 56, tone: accent, rows: [[ALB, main code], 公開の入口] }
      - box: { name: api, x: 400, y: 72, w: 200, h: 56, rows: [API サーバー, ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

- ほかの鍵の代わりに `example: <種類>` を書くと、組み込みの見本を描きます（`floe diagram example` が並べる）。ページは図だけを見せ、spec は見せません。見本の spec を読む・写すときは、`floe diagram example <種類>` で出します。
- 文（`title`・行・ラベル）は、文か、言語ごとの言葉（英語を含む）です：`{ en: Store, ja: 保存先 }`。ページは `lang:` の言葉を描き、spec は書いた言語ごとに並べて確かめます。
- 部品は、種類を名指す鍵 1 つで書きます。`floe part show diagram schema.json` がすべての鍵を出し、`floe diagram example <種類>` が種類ごとの spec を、格子を説明するコメント付きで出します。

| 部品 | 鍵 |
|---|---|
| `frame` | `x` `y` `w` `h`、`label`、`name`、`accent`、`square`（角を丸めない）、`icon`（`aws/group/region` のようなアイコン。AWS のグループの形で、角にアイコン）。ラベルは左上の内側に置くので、中身は上から 32 以上（アイコンがあれば 40）下に置く。 |
| `box` | `x` `y`、`rows`、`w` `h`（省けば行に要る大きさ）、`name`、`origin`（`left`。`x` を真ん中にするなら `centre`）、`tone`、`shape`（`rect`。`pill`、判断の `hex`、`state`）。行は文か `[文, 形]` で、形は `main`・`sub`・`main code`・`sub code`。形が無ければ最初の行が `main`、残りが `sub`。 |
| `table` | `x` `y`、`title`、`rows`、`w`、`name`、`origin`、`tone`。行は `[名前]`、`[名前, 型]`、区切りの `---`、または線が行に着く `{ name, cells: [名前, 型] }`。 |
| `icon` | `x` `y`（左上）、`icon`、`label`（アイコンの下）、`size`（48）、`name`。`icon` はセットとその中の名前で `aws/amazon-ec2` のように書く（`floe icons search` で探す）。 |
| `edge` | `from` と `to`（部品・名前付きの行・`tree` の節の `name`、または点 `[x, y]`）、`fromSide` と `toSide`（`top`・`right`・`bottom`・`left`）、`via`（曲がる点）、`straight`（直角でなく点から点へ）、`label` と `at: [x, y]`、`dashed`、`bold`、`tone`、`start`（`none`）、`end`（`arrow`）、`crossing`。端は `arrow`・`triangle`・`diamond`・`odiamond`・`one`・`many`・`zero-one`・`zero-many`・`none`。`crossing: true` でこの線はほかと交われる。 |
| `dot` | `cx` `cy`、`r`（7）、`name`、`tone`、`hollow`。線は中心まで届いてよい。 |
| `bar` | `x1` `x2` `y`、`name`、`tone`、`label`（棒の右）。 |
| `axis` | `x1` `x2` `y`、`ticks: [[x, 文], ...]`、`grid`（格子の線の上端）。 |
| `circle` | `cx` `cy` `r`、`name`、`tone`。円どうしは重なってよい。 |
| `label` | `text`、`x` `y`、`anchor`（`middle`。`start`・`end`）、`plain`（背景なし）、`strong`、`in`（ベン図で、内側に置く円の名前。ほかの円の外。`x` と `y` を省けば floe が置く）。 |
| `line` | `x1` `y1` `x2` `y2`、`kind`（`line`。`lifeline`・`divider`）。ほかの部品と交わってよいが、キャンバスから出ない。 |
| `tree` | `x` `y`（根の中心と上）、`root`：節。節は `box` と同じ `rows`・`name`・`tone`・`shape` と、子の節を並べた `children` を持つ。floe が各節を親の下に置き（子は横に並べ、深さごとに 1 段にそろえる）、親から子へ線を引く。節の `name` は、手で書いた線の端にできる。 |
| `raw` | `svg`：どの部品でも描けない SVG。SVG の要素だけで（script や `foreignObject` は無し）、配置も確かめもしない。 |

`via` も辺も無い線は、相手に向いた辺から、直角に、曲がり 2 つまでで進みます。別の道にするには `via` か辺を書きます。

確かめること：

- 箱に対して広すぎる・高すぎる字（frame のラベルは frame の幅に対して）。
- キャンバスからはみ出す部品（線・line・axis も）。
- 箱・印・ラベル・frame のラベルどうしの重なりと、frame の縁をまたぐ箱と印。frame どうしは比べないので、交わってよい。
- 箱・ラベル・frame のラベルを通る線と、X の形に交わる線（片方に `crossing: true` があればよい）。
- どの部品も持たない名前や、端の部品に無い辺に着く線。
