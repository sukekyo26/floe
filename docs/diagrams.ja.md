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

- `width` と `height` は、それぞれ省けます。省くと、キャンバスは描いたものの右（下）の端まで広がり、さらに 20 空けます。左上は 0, 0 のままなので、座標は書いたとおりです。`raw` の SVG は測らないので、`raw` を使う図は `width` と `height` を書きます（書かないと切れることがあります）。
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
| `graph` | `x` `y`（全体の左上）、`nodes`、`edges`、`direction`（`down`。`right`）。節は `box: { name, rows, tone, shape }` で、`box` と同じ鍵を持ち、`name` は要る。線は `{ from, to }`（その graph の箱の名前 2 つ）で、`label`・`dashed`・`bold`・`tone`・`start`・`end`（`arrow` か `none`）を持てる。floe が箱を段に分けて並べ、箱の間に線を引く（下）。 |
| `raw` | `svg`：どの部品でも描けない SVG。SVG の要素だけで（script や `foreignObject` は無し）、配置も確かめもしない。 |

`graph` には、箱のつながりだけを書きます。置き場所は floe が決めます：

```yaml
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

- 箱は、線が来る箱より下の段に置きます（輪を作る戻りの線も、書いた向きのまま描きます）。段の中の箱は、線の交わりが少ない順に並べ、交わりが無ければ書いた順のままです。同じ spec は、いつも同じに並びます。
- 線は、箱の辺の別々の点から、形の輪郭の上で出入りするので、2 本の線が同じ所を通りません。線の多い箱は広くなります（`right` では高くなります）。同じ graph の線どうしは交わってよく、graph の線とほかの部品は、ほかの線と同じく確かめます。
- 線の `label` は、graph の並び方を変えません。線を引いた後に、書いた from の箱に近い、線のまっすぐな区間の 1 列横の空き（縦の線は右、無ければ左。横の線は下、無ければ上）に、地の色なしで立ちます。空きが無ければ、floe がそこで図を広げて場所を作り（箱が広がることもあります）、線はラベルが無いときと同じにまっすぐです。ラベルの場所はいちばん広い言語の字で決めるので、どの言語でも同じに並びます。graph の `x`・`y` は、ラベルも含めた全体の左上です（1 列の内の差はあります。ラベルが箱より左や上に出ると、箱の格子を保つために、graph を 1 列の倍だけずらします）。
- 1 つの graph の箱は 200、線は 400 までです。図の全部の graph の線が端と端の間で越える段は、合わせて 4000 までです。
- graph の箱は、手で書いた線の端にもできます。その線は辺の真ん中で終わるので、graph の線が使わない辺を `fromSide`・`toSide` で選びます（graph の線の上を走らないように）。
- graph の部品と手で書いた部品の間の誤り（重なり、線が箱を通る、交差）は、動かせる、手で書いた方の行と列に言います。graph の部品だけが起こす誤り（2 つの graph の重なりや交差、キャンバスからはみ出す graph）は、graph の場所に、その `x`・`y` を動かすよう言います。
- 箱から同じ箱へ戻る線は断ります。

`via` も辺も無い線は、相手に向いた辺から、直角に、曲がり 2 つまでで進みます。別の道にするには `via` か辺を書きます。

確かめること：

- 箱に対して広すぎる・高すぎる字（frame のラベルは frame の幅に対して）。
- キャンバスからはみ出す部品（線・line・axis も）：0, 0 より左か上、または書いた `width`・`height` の外。
- 箱・印・ラベル・frame のラベルどうしの重なりと、frame の縁をまたぐ箱と印。frame どうしは比べないので、交わってよい。
- 箱・ラベル・frame のラベルを通る線と、X の形に交わる線（片方に `crossing: true` があるか、どちらも 1 つの `graph` の線ならよい）。箱は描いた形で測るので、線は hex の尖りの外や丸い角の外を通ってよく、縁の上や縁から 1 以内を走ってもよい。
- どの部品も持たない名前や、端の部品に無い辺に着く線。
