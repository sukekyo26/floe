# CLAUDE.md

floe はエージェントが書いたページをローカルサーバー（`http://127.0.0.1:7325`）で表示・管理する Go の単一バイナリです。データ（YAML）とテンプレートから文書を組み立て、検査付きの inline SVG 図を描きます。

- 検証は `just ci`（CI と同じ内容）。golangci-lint は `just setup` で入る。
- **worktree は本体の中に作る**：`~/work/floe` で起動し、ブランチの worktree は `.claude/worktrees/<branch>` に作る。ここでは、決まりは本体の `.claude/CLAUDE.md` と `.claude/rules/` だけが届き、ブランチのものは届かない。本体に無い決まりと、外の worktree では、誤りも警告も出ずに届かないので、worktree を作る前に本体の develop を pull する。
- 対応 OS は linux と darwin（amd64・arm64）。Windows はリンクのパス検査などの信頼境界を検証できないので対応しない。
- 決まりは、それを入れた PR がこのファイルに足す。足すのは、読まないと書くコードを誤ることだけで、1 つの決まりは数文にする。書き手に見える動きは docs に、理由と限界はコードのコメントと `LIMIT:` に、測れることはテストに書き、ここには書き写さない。触るときだけ要る決まりは `.claude/rules/` に置き、同じ決まりを 2 つのファイルに書かない。決まりや動きを変えたら docs も直す。

## 検査の方針

floe が入力を断る検査は、次の 4 つのどれかに当たるものだけにする。当たらない検査や特例は入れない。

- **ライブラリが返す誤り**：YAML・JSON Schema・html/template・expr などが返す誤りを、ライブラリの言葉のまま出す。言い直さない。場所は、ライブラリが返す分だけ付ける（JSON ポインタを YAML の行と列に写すのはよい）
- **信頼境界**：パス、CSP、サンドボックス、大きさなど、floe の外へ出る・外から入るものを守る。文脈によらない保守的な決まりで断る
- **floe 自身の仕組みの確かめ**：floe が置いた印や、floe がスタイルを入れる所など、floe の仕組みが前提にするものを確かめる。部品の値の形が部品の描き方の前提を破り、値が黙って消える所も当たる（今は `table` の行のセルの数と鍵、色付きのアイコン集のアイコンの `tone`、`--radius-small`・`--radius-pill` の値が 1 つの項であること、`stats` で `from` を書いた項目の `from`・`value` が数字だけであること（変化を数えるため。`kit_stat_change`）、値の中の誤りを返す部品の関数（bars・code・script・stats）が読む値の形（`kit_part_value`。chart は `kit_chart_shape`））。
- **ライブラリの黙った読み違い**：ライブラリが誤りを出さずに、書いたのと違う値を読む所を断る。今あるのは、YAML（2 つ目の文書、`#` で値が切れる（同じ行のほか、鍵より深いコメントの行）、`{a, b: 1}`、文字でない鍵、`.inf`・`.nan`、YAML 1.2 と読みが違う数、先頭のほかの BOM、UTF-8 でないバイト列、字下げの数を書かない `|`・`>` の字下げの後のタブ。`yamlpos.Parse`）、schema の JSON（同じ鍵、先頭のほかの BOM、孤立したサロゲートと UTF-8 でない字。`read`）、API の書き込みの本文（大文字小文字だけが違う鍵、同じ鍵、孤立したサロゲートと UTF-8 でない字。`decodeStrict`）、タスクを名指す問い合わせとページの `work.task`（UTF-8 でない字。`taskKey`・`TaskName`）、html/template の `ZgotmplZ`。決まりを 1 つにするため、goccy が正しく読む入力も同じ形なら断る（`{ }`・`[ ]` の中のコメント、1 行だけの `|` の文のタブ、値の後の、鍵より深いコメント。ユーザーが決めた）

- **警告**：上の 4 つは断る検査で、警告はその外にある。警告にしてよいのは、ページが崩れず、より良い書き方を勧めるだけのもの（勧め）だけで、直さないと困るもの（崩れる、誤った値になる、写すと動かない）は断る。警告は、勧めのとおりに直せる所にだけ出し、終了コードを変えない。見つけるのも断る検査と同じく本物のパーサやライブラリで（Markdown は goldmark の木で）、正規表現で読まない。今の警告は、部品 text の値の中のコードの囲み（`draw_text_fence`）だけで、見るのは `draw.drawItem` の 1 か所。例外：自分の schema が code の部品を置けない所で部品 text を指すと、勧めのとおりに直せないのに、そこにも警告が出る（`LIMIT:`。text の説明と docs が、ただの Markdown の欄に format markdown を勧める）

## 作りの決まり

- **使えない自分のものは止める**：描かずに分かる誤りで使えない自分の部品・テーマ・テンプレートが 1 つでもあれば、その出力を止めて理由を並べる。使った所だけで断る仕組みは作らない
- **集めるのは楽な所だけ**：1 つの段の中で、互いに関係の無い検査は集めて一度に出す。集めるために特例や推しを足さない。例外：ライブラリが最初の誤りで止まり、どれを返すかが決まらない（上流が直さない）ときは、ライブラリが型の付いた誤りで名指した所だけを、意味を作らない形で埋めて呼び直し、誤りを決まった順で集めてよい。回数には上限を置き、埋められない形が混ざると全部は言わない。今これに当たるのは、schema のコンパイルの、解けない `$ref`（`$defs` の直下の 1 段、断った URL、無い部品）だけ（`holes`）
- **外の振る舞いを真似ない**：HTML・CSS・URL・JSON Schema などは、本物のパーサやライブラリを使う。自分で読む作りにしない。仕様が規範として定める表をそのまま写すのは、真似でなく文脈の要らない決まりで、よい（今は yamlpos の数の読みが、YAML 1.2 の core schema の表の正規表現を写し、draw が select の中で断る要素の並びが、HTML の断片を読む algorithm の表を写す）
- **受け入れた退行**：上に当たらない漏れは、`LIMIT:` のコメントと PR の本文に書く。検査は足さない

- **ユーザーのパスはふつうのファイルだけ読む**：ユーザーが渡したパス（データファイル・spec・zip・ページ・publish のリンク）から読むのは、ふつうのファイルだけで、`files.Open`・`files.Read` を通す。開く前に種類を見て、FIFO・デバイス・socket は開かない（開くと止まる、開くこと自体に副作用がある）
- **自分のものの置き場所は重ねない**：部品・テーマ・テンプレートの置き場所の重なりは、`app.kit` が開く前に、字でなく OS の実体（`resolve`・`within`）で比べて `dir_overlap` で断り、重なった置き場所は開かない。`newOwn` も作る前に比べる
- **render が書くのはページだけ**：書くページのリンクはたどり、たどった先の名前が .html（`files.IsPage`）でなければ、何も書かずに断る。データファイルやリポジトリの外のファイルを書き換えない
- **YAML は yamlpos だけが読む**：YAML のバイト列を goccy に渡すのは `yamlpos` だけ。`yamlpos` の外（テストを除く）から import できる goccy のパッケージは `ast` だけ（depguard の規則 `goccy-in-yamlpos`）。値にするのも `yamlpos.Decode`
- **schema の JSON は schema だけが読む**：schema.json などのバイト列を読むのは `schema` の `read` だけで、外からは `Compile`・`NewResource` に渡す。構文の誤りと黙った読み違いは、`read` が値にする前に断る（ライブラリは後の値や U+FFFD で読む）。jsonschema を import できるのは `internal/schema` だけ（depguard の規則 `jsonschema-in-schema`）
- **Markdown は kit だけが読む**：goldmark と goldmark-cjk-friendly を import できるのは `internal/kit` だけ（depguard の規則 `goldmark-in-kit`）。生の HTML を出さない `markdown` を、どこも通る
- **floe の script は `page.Script` 1 つで組み立てる**：ページに入れる floe の script（図の拡大、写すボタン、目次の今の節）は、`page.Script` が関数と JSON の引数から組み立て、コメントだけの行を落とす
- **CSS は字句と括弧の入れ子だけで読む**：CSS は tdewolff の字句と、CSS Syntax の括弧の入れ子（`{`・`(`・関数・`[`）だけで読み、セレクタや値の意味を自前で読まない（`css.Parser` も使わない）。文字列・url・コメントの外のエスケープ（`\`）は断り、解かない。tdewolff を import できるのは `internal/css` だけ（depguard の規則 `tdewolff-in-css`）
- **保存先を書くのはサーバーだけ**：CLI は API で頼む。publish・link・rm・rebuild・管理ページの変更はどれも API
- エージェントの手引き docs/agents/guide.md は docs の要約で、`floe guide` が出す。docs の決まり・コマンド・鍵を変える PR は手引きも直す。部品やテンプレートの品揃えは書き写さず、list・show を教える。スキルは `floe guide` を指すだけ。

## floe の不具合や困ったこと

floe を使っていて、または直していて困ったこと（誤りの文が直し方を言わない、検査の漏れ、書きづらいなど）に気付いたら、その時点で報告し（回避したときも）、聞かずにページ `floe-sc-usage-issues`（データは `~/.local/share/floe-plans/floe-sc-usage-issues.yaml`）に S 番号で載せる。書き方はそのページの注に従い、floe 本体を直すかはユーザーが決める（今の PR に混ぜない）。
