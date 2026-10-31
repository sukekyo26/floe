# CLAUDE.md

floe はエージェントが書いたページをローカルサーバー（`http://127.0.0.1:7325`）で表示・管理する Go の単一バイナリです。データ（YAML）とテンプレートから文書を組み立て、検査付きの inline SVG 図を描きます。

- 検証は `just ci`（CI と同じ内容）。golangci-lint は `just setup` で入る。
- 対応 OS は linux と darwin（amd64・arm64）。Windows はリンクのパス検査などの信頼境界を検証できないので対応しない。
- 決まりは、それを入れた PR がこのファイルに足す。

## 検査の方針

floe が入力を断る検査は、次の 4 つのどれかに当たるものだけにする。当たらない検査や特例は入れない。

- **ライブラリが返す誤り**：YAML・JSON Schema・html/template・expr などが返す誤りを、ライブラリの言葉のまま出す。言い直さない。場所は、ライブラリが返す分だけ付ける（JSON ポインタを YAML の行と列に写すのはよい）
- **信頼境界**：パス、CSP、サンドボックス、大きさなど、floe の外へ出る・外から入るものを守る。文脈によらない保守的な決まりで断る
- **floe 自身の仕組みの確かめ**：floe が置いた印や、floe がスタイルを入れる所など、floe の仕組みが前提にするものを確かめる
- **ライブラリの黙った読み違い**：ライブラリが誤りを出さずに、書いたのと違う値を読む所（YAML の 2 つ目の文書、`#` で値が切れる、`{a, b: 1}`、文字でない鍵、`.inf`・`.nan`、YAML 1.2 と読みが違う数（`017`・`1e3` など）、schema の JSON の 1 つの object の中の同じ鍵、html/template が値を誤りなしに `ZgotmplZ` に置き換える所、ファイルの先頭のほかの BOM（YAML と schema の JSON。引用符の中も）、YAML の UTF-8 でないバイト列、YAML の、字下げの数を書かない `|`・`>` の文の、空白だけでない最初の行の、字下げの空白の後のタブ（goccy が正しく読む 1 行だけの文なども、決まりを 1 つにするために断る）、schema の JSON の文字列の中の孤立したサロゲートと UTF-8 でない字（U+FFFD と読む）、API の書き込みの本文の、大文字小文字だけが違う鍵・同じ鍵の 2 つ目・文字列の中の孤立したサロゲートと UTF-8 でない字）を断る

- **警告**：上の 4 つは断る検査で、警告はその外にある。警告にしてよいのは、ページが崩れず、より良い書き方を勧めるだけのもの（勧め）だけで、直さないと困るもの（崩れる、誤った値になる、写すと動かない）は断る。警告は、勧めのとおりに直せる所にだけ出し、終了コードを変えない。見つけるのも断る検査と同じく本物のパーサやライブラリで（Markdown は goldmark の木で）、正規表現で読まない。今の警告は、部品 text の値の中のコードの囲み（`draw_text_fence`）だけで、見るのは `draw.drawItem` の 1 か所。例外：自分の schema が code の部品を置けない所で部品 text を指すと、勧めのとおりに直せないのに、そこにも警告が出る（`LIMIT:`。text の説明と docs が、ただの Markdown の欄に format markdown を勧める）

## 作りの決まり

- **使えない自分のものは止める**：描かずに分かる誤りで使えない自分の部品・テーマ・テンプレートが 1 つでもあれば、その出力を止めて理由を並べる。使った所だけで断る仕組みは作らない
- **集めるのは楽な所だけ**：1 つの段の中で、互いに関係の無い検査は集めて一度に出す。集めるために特例や推しを足さない。例外：ライブラリが最初の誤りで止まり、どれを返すかが決まらない（上流が直さない）ときは、ライブラリが型の付いた誤りで名指した所だけを、意味を作らない形で埋めて呼び直し、誤りを決まった順で集めてよい。回数には上限を置き、埋められない形が混ざると全部は言わない。今これに当たるのは、schema のコンパイルの、解けない `$ref`（`$defs` の直下の 1 段、断った URL、無い部品）だけ（`holes`）
- **外の振る舞いを真似ない**：HTML・CSS・URL・JSON Schema などは、本物のパーサやライブラリを使う。自分で読む作りにしない。仕様が規範として定める表をそのまま写すのは、真似でなく文脈の要らない決まりで、よい（今は yamlpos の数の読みが、YAML 1.2 の core schema の表の正規表現を写し、draw が select の中で断る要素の並びが、HTML の断片を読む algorithm の表を写す）
- **受け入れた退行**：上に当たらない漏れは、`LIMIT:` のコメントと PR の本文に書く。検査は足さない

- **schema の形はコンパイルした木から**：floe が要る形（どの項目がどの部品・format か）は、コンパイルした `*jsonschema.Schema` から取る。schema の JSON を自前で辿らない（生の JSON を見るのは `$schema` の確かめと、`$id` の scheme（`floe:` を名乗らないか。部品と共有の schema では scheme の無い相対も）と、根に `$id` が無いかの確かめ（根の URL は floe が付ける）と、`read` の構文・同じ鍵・読み違い・値の後ろの 2 つ目の値の確かめ（「schema の JSON は schema だけが読む」）だけで、schema の鍵の意味は読まない。`$id` を土台の URL に重ねることはしない）。`$schema` は 2020-12・2019-09 だけを受ける。例外：上の「埋めて呼び直す」ときは、写しに名前を足すために、生の JSON の根と `$defs` の直下を読んで書いてよい（書くのは写しだけ。今は `holes.fill`）。floe が要る形は、この例外の後もコンパイルした木から取る
- **木は Follow で歩く**：schema の木は、全部の欄を辿る `every` か、`schema.Follow` の繰り返しで歩く。描く歩きも確かめる歩きも、検査ごとに歩き方を書き分けない。生の JSON を読むのは「schema の形はコンパイルした木から」に並べた所だけ。例外は：`$schema` と `$id` を見る生の JSON の歩き（schema の意味を読まない）と、根の値にいつも当たる schema の required を見る `sharedKeys`。`read` の `keyWalk` は、値にする前の JSON の字句だけを読み、schema の木も生の JSON の値も歩かないので、この例外にも当たらない
- **項目の検査は checkNode 1 つ**：schema の 1 つずつの検査は `checkNode` 1 つで、テンプレートも部品も共有の schema も `checkOwn` の同じ歩きで走らせる。共有の schema は、指す側の歩きでなく、その根から確かめる（指す側は根しか指せない）。指す側の `$ref` が辿られない所（辿らない場所にある、横の format で止まる）は、指す側で言う。テンプレートに足す検査は `checkNode` に足し、部品に写さない
- **値を歩くのは Walk だけ**：データの値を schema に沿って歩くのは `schema.(*Compiled).Walk` だけで、値ごとに `Follow` を 1 回呼び、部品の値はその部品の schema の根から歩き続ける。描く値も、確かめる項目（部品の check を走らせる所）も、Walk から取る
- **part.html は元の値を `written` で読む**：part.html に届く値（`.`）は、中の部品と floe の format を描いた後のもの。描く前の字と比べる部品（table の列の `pills`）は、関数 `written`（draw が部品ごとに縛る、書かれたままの値）で読み、中の部品の形を自分の schema に写さない。`written` の字は、比べるか、pill の字のように字のまま描く（Markdown や HTML として描かない。描いた項目は `.` から取る）。書いていない鍵は無い（`.` と違い nil で埋まらない）
- **部品もテンプレートと同じ format を取る**：部品の schema も、テンプレートと同じく floe の format（html・markdown・image）と、それを持つ共有の schema を、floe が辿る所で取る。部品だけの format の決まりは作らない。部品の html の項目も、描いたページの確かめ（印と探り）を通る。部品の出力は頭と終わりの空白を `strings.TrimSpace` で削る（項目の探りは区切りのコメントの内にあるので削られない）
- **部品は見出しを描かない**：部品は自分が置かれる段（テンプレートの節、details・tabs、ほかの部品の中）を知らないので、part.html に `<h1>`〜`<h6>` を書かない。カードなどの題は `class="title"` の要素で書き、見た目 cards が題の形を持つ（見出しで題を書くテンプレートも `class="title"` を付ける）。見出しを描くのは、段を知っているテンプレートの page.html と、書き手が段を決める Markdown だけ。`TestEveryPartDrawsEveryPart` が、描いた every-part の木で、部品の中に見出しが無いことを確かめる（値で出ない枝と、部品の Markdown の見出しは見ない。`LIMIT:`）。自分の部品には docs で勧め、確かめない
- **描いたページは x/net/html で読んで書く**：描いたページの確かめ（「データの HTML 項目は描いたページの木で確かめる」が断るもの、部品の名前）は x/net/html の木で行い、スタイルを head に入れるのも、見出しに id を付けるのも、その木を直して `html.Render` で書く。正規表現は、CSS の `</style` の書き換えだけに使う
- **データの HTML 項目は描いたページの木で確かめる**：format html の項目は、描くとき、探りの空白と、偽れない印のコメントの対で挟み、探りの外側にも偽れない区切りのコメントを置く（探りをいつも探りだけの文にし、page.html の字と 1 つの文にしない。表の中では、空白でない字を含む文は字句ごと表の前に出される）。描いたページの木（`html.Parse` 1 回）で、印が同じ親の兄弟か、探りが動いていないか、印がコメントのまま残っているかを見る。見た後に印と探りを外して書き出し、確かめた木と書き出す木を同じにする。断る判断は描いたページの木だけで決め、項目の文だけを読んで判断しない（svg・math の中で、ブラウザと読みが違う）。ただし、断ると決めた後で、原因の要素を名指すためだけに、その項目を、ページの木で始まりの印がある要素を文脈にして `html.ParseFragment` で読み直してよい（今は、断った項目と、断った所より前に印のある項目の、写しのうち、パーサが印を動かしたものだけ：コメントとして残った k 番目の始まりの後ろに、コメントとして残った k 番目の終わりが無い写し。page.html が項目を字や属性の中に置いた誤り（始まりがコメントとして残らない）では読み直さない。`named`・`placed`）。読み直しの結果は断る判断に使わず、名指せなければ木から読んだ文のまま。`<style>`・`<base>`・`<noscript>` と、charset と viewport のほかの `<meta>` は、項目でなく、描いたページ全体で断る（page.html・部品の part.html・項目のどれが書いても）。HTML の `<select>` の中の、中身を字として読む要素（HTML の断片を読む algorithm の、context の要素で tokenizer の状態を決める段の要素）と svg・math も、描いたページ全体で断る（古い select の読み方のブラウザは、字として隠れた中のタグで select を閉じ、その後を要素として読む）。属性の名前か値に `ZgotmplZ`（html/template が、そこに置けない値の代わりに誤りなしに書く字）があるものも、描いたページ全体で断る。floe が描くのは静的なページで、表示に要らないものは断る
- **ユーザーのパスはふつうのファイルだけ読む**：ユーザーが渡したパス（データファイル・spec・zip・ページ・publish のリンク）から読むのは、ふつうのファイルだけで、`files.Open`・`files.Read` を通す。開く前に種類を見て、FIFO・デバイス・socket は開かない（開くと止まる、開くこと自体に副作用がある）
- **自分のものの置き場所は重ねない**：部品・テーマ・テンプレートの置き場所の重なり（同じ、一方がもう一方の中）は、`app.kit` が開く前に、OS の実体（`resolve` と `within`。symlink と `..` は OS が解き、まだ無い名前は字で、有るものは `os.SameFile` で）で比べ、組ごとに `dir_overlap` で 1 回断る。重なった置き場所は開かない。読むコマンドは、自分が読む種類の、設定したもの（無くても）と有る既定だけを比べ、有ってディレクトリでないものは比べない（`ownDir` が `dir_setting` で言う）。`newOwn` は、作るディレクトリをほかの種類のすべての置き場所と、名前を確かめた後に比べる。`dir_overlap` は、重なった 2 つの設定を、部品・テーマ・テンプレートの順（`ownKinds`）で名指す
- **render が書くのはページだけ**：書くページのリンクはたどり、たどった先の名前が .html（`files.IsPage`）でなければ、何も書かずに断る。データファイルやリポジトリの外のファイルを書き換えない
- **YAML は yamlpos だけが読む**：YAML のバイト列を goccy に渡すのは `yamlpos` だけ。`yamlpos` の外（テストを除く）から import できる goccy のパッケージは `ast` だけ（depguard の規則 `goccy-in-yamlpos`）。値にするのも `yamlpos.Decode`
- **schema の JSON は schema だけが読む**：schema.json などの JSON のバイト列を読むのは `schema` の `read` だけ。`schema` の外からは、バイト列を `Compile`・`NewResource` に渡す。構文の誤り（`encoding/json/jsontext` の字句で読み、その言葉で）と、ライブラリが黙って読み違えるもの（1 つの object の中の同じ鍵（名前だけを比べる）、先頭のほかの BOM、文字列の中の孤立したサロゲートと UTF-8 でない字）は、`read` が値にする前に断る（ライブラリは後の値や U+FFFD で読むので、構文の誤りと同じく読む所で断る）。値の後ろの 2 つ目の値も、`read` が字句で見て、floe の文で断る（ライブラリも断るが、場所を言わない）。jsonschema を import できるのは `internal/schema` だけ（depguard の規則 `jsonschema-in-schema`）
- **段 1 は読むと確かめるに分ける**：kit の `read` は値を作るのに要ることだけ（ファイルを読む、YAML・JSON・HTML を値にする、schema をコンパイルする）をし、値の後の決まり（CSS の形、部品の schema、エスケープ、見た目の名前など）は確かめる関数 `check` が持つ。kit の外で Kit を作れるのは、read と check を通す `Load` と、floe のものを読む `Floe()` だけで、確かめていない自分のもので描くことは、型で起きない。実行のときは floe のものに `check` を走らせず（読むだけ）、同じ `check` をテストで floe のものに走らせる。例外：後で一緒にコンパイルするものの読み方を変える入力は、値を作るのに要る確かめとして読む所で断る（確かめる関数に置くと、その前のコンパイルが別の誤りで失敗し、原因が言われない）。この確かめは `read` にあるので、floe のものも実行のときに通る。今これに当たるのは、部品と共有の schema の `$schema` と `$id`（`schema.NewResource`）だけ
- **読んだ Kit は変えない**：`Kit` は欄を外へ出さず、`[]byte` も写しを返す。自分のものを重ねるときは、土台を書き換えずに新しい `Kit` を作る（`Floe()` の値をプロセス全体で共有するため）。描くときは、`Template.Page`・`Part.Page` が返す写しを Execute する（Execute は html/template にエスケープの結果を書き込み、その後は写しを作れない）
- **show が出すのは読んだファイル**：`floe part show`・`template show`・`theme show` が出すファイルは、読み手が読んだもの（`readFile` が書き留めたもの）だけで、順は種類ごとの並び 1 つ（`partFiles`・`lookFiles`・`templateFiles`、テーマは palette・base・部品の名前の順）。読まないファイルの確かめ（`unreadIn`）も同じ並びを使う。show のために出すファイルを別に選ばない
- **stack は書いた部品を全部描く**：details・tabs の中身の部品 stack の part.html は、部品の種類を書き並べず、report の page.html と同じ繰り返しで描く。置ける部品は stack の schema 1 か所で決める。tabs は入れない（tabs は form で描き、form の中の form は HTML の読みで落ちて、2 つのタブの radio が 1 組になる。schema の `$comment`）。`TestEveryPartDrawsEveryPart` が、schema の部品が every-part の stack の中で描かれることを確かめる（箱を描かない text と、見本が diagram/ にある diagram を除く）
- **部品 text はコードの部品と並ぶ Markdown**：部品の並び（any.json と stack）の Markdown は floe の部品 text で、Go はその名前を `kit.TextPart` 1 つで持つ。テンプレートや部品の自分の Markdown の欄は部品 text にしない
- **Markdown は kit だけが読む**：goldmark を import できるのは `internal/kit` だけ（depguard の規則 `goldmark-in-kit`）。生の HTML を出さない `markdown` を、どこも通る
- **行の上下の余白は `--floe-row-pad` 1 つ**：線で区切った行が縦に並ぶもの（見た目 table と rows、facts のタイル）の行の上下の余白は、base.css の `--floe-row-pad` だけが決める。見た目 rows を借りるもの（部品・テンプレート）は、並びの印と余白は floe の `boxes` が外し（「部品の箱と見た目の並びは floe が外す」）、rows が行に余白を付け（箱が最後に `::after` で描く行も）、借りる側は行の左右だけを書く。見た目 table と facts は変数を読む。テーマは替えない（テーマの base.css は層で替えられてしまうので、`TestUIRowPad` が全テーマで測って守る。テーマが変えてよいのは、列の端と飾りのための空きで、表の見出しの行も行として変数に任せる）。決めるのは余白だけで、行の中の字の line-height は決めない
- **写すボタンは `copyButton` 1 つ**：部品の中身をクリップボードに写すボタンは、part.html の `{{copyButton}}`（`kit/copy.js`）1 つが描き、写す要素は part.html が `data-floe-copy` で決める。ボタンの形は floe の base（`.floe-copy`）1 か所で、大きさ・まわりの間・字がボタンのために右に空ける幅は、base.css の 3 つの変数（`--floe-copy-size`・`--floe-copy-gap`・`--floe-copy-room`）だけが決める。置き場所だけを部品の CSS が決める。ボタンを描く部品は、一番長い行の後ろ（行の箱の右）に `--floe-copy-room` の余白を取り、右端までスクロールすれば行の終わりがボタンの左に出るようにする（その決まりは `:scope:has(> .floe-copy)` でボタンを描く部品だけに当てる）。言葉は `Part.Page` がページの言語で縛り、script は言語ごとに 1 回だけ作る。部品ごとに script・ボタン・言葉・ボタンの形を書かない
- **floe の script は `page.Script` 1 つで組み立てる**：ページに入れる floe の script（図の拡大、写すボタン）は、`page.Script` が関数と JSON の引数から組み立て、コメントだけの行を落とす
- **CSS は字句と括弧の入れ子だけで読む**：CSS は tdewolff の字句と、CSS Syntax の括弧の入れ子（`{`・`(`・関数・`[`）だけで読み、セレクタや値の意味を自前で読まない（`css.Parser` も使わない）。文字列・url・コメントの外のエスケープ（`\`）は断り、解かない。tdewolff を import できるのは `internal/css` だけ（depguard の規則 `tdewolff-in-css`）
- **図の形は schema、配置は Go**：図の spec の形（鍵・型・値の一覧・要るもの）は floe の部品 diagram の schema（`internal/kit/floe/parts/diagram/schema.json`）だけが決め、jsonschema が確かめる。Go は確かめた値を読むだけで、Go が見るのは、名前の解決（部品と行の名前、アイコン集にそのアイコンがあるか）、配置の検査（図という機能そのもの：重なり・はみ出し・線が通る・交わる・ベン図の領域。箱の形は（icon の四角も）`outlineOf` 1 つが決め、描くのも線が通るかを測るのもそこから取る）、raw の要素（x/net/html で読む）、部品 tree を箱と線に開いて置き場所を決めること（`tree.go` の `expand`。read が作った型の付いた構造体を読むだけで、schema に沿って値を歩かないので、「値を歩くのは Walk だけ」に当たらない）、省いた `width`・`height` を描いたものの右と下の端から決めること（`layout.go` の `reach`。raw の SVG は測らない、`LIMIT:`）だけ。図を描く入口は `diagram.Drawer` の 1 つで、渡された値をいつも schema で確かめてから読む（どこから来た値でも、確かめていない値で floe の失敗にならない）。配置と形の誤りは spec の中の JSON ポインタで返し、行と列は呼ぶ側が `yamlpos.Place` で付ける
- **保存したページは描いた HTML を返す**：publish（か `store.Rebuild`）の時に `tmpl.Draw` が描いた HTML を、写し（`source-{id}/` の `data.yaml`・`page.html`・`images/`）に保存し、開くときはそれを返す。開くときに描くのは link したデータファイルだけ。CSP とサンドボックスは HTML に焼き込まず、返すとき（ダウンロードも）に付ける
- **slug と写しのパスは 1 か所で作る**：slug をパスにするのは `store` の `dir` だけ、meta.json の `source` を写しのパスにするのは `copyDir` だけで、そこで確かめる（`ValidSlug`、save が付ける名前か）。呼ぶ側で別に確かめない
- **publish は読んだ画像だけを書く**：publish が保存する画像は、描くときに `kit.ReadImage` が読んだ名前のものだけ。送られても読まなかった名前は書かない（信頼境界）。rebuild は前の写しの `images/` を全部引き継ぐ
- **保存先を書くのはサーバーだけ**：CLI は API で頼む。publish・link・rm・rebuild・管理ページの変更はどれも API
- **サーバーを止めるのは `/api/stop` 1 つ**：`floe stop` も別のビルドのサーバーの置き換えも、`daemon.Stop` が `POST /api/stop` で、止めたいサーバーの pid（health が名乗ったもの）を本文に書いて頼む。サーバーは自分の pid のときだけ 204 を返し、違えば 409 で断る（後に起きた別のサーバーを止めない）。floe のコマンドはシグナルを送らない（名乗る pid は、コンテナの中など、この機械の別のプロセスでありうる）。サーバーは、頼まれても SIGTERM・SIGINT でも、`Serve` の 1 つの終わり方（期限付きの `Shutdown`）で、途中の要求を終えてから止まる。要求の ctx は `Serve` の ctx で切らない
- **アーカイブは meta.json の `archived` 1 つ**：一覧から外すのは meta.json の `archived` だけで持ち、ディレクトリ・`/a/{slug}/`・ダウンロード・rebuild は見ない。立てる・外すのは `Store.Edit`（`PATCH`）だけで、ほかに外れるのは publish し直したとき（`newMeta` が引き継がない）だけ。`GET /api/artifacts` は全部を返し、一覧から外すのは見せる側（`floe list` と管理ページの `inView`）でする
- **フォルダの中は開いた所だけ読む**：フォルダのリンクは meta.json の `folder` で見分け、名前で種類を決めない（データファイルかは `Meta.IsData` 1 つ）。一覧（`GET /api/artifacts`）はフォルダを 1 項目のまま返し、中は `GET /api/artifacts/{slug}/pages?dir=`（`link.Dir`）だけが、開いたディレクトリ 1 つを、返すときと同じ門（`link.within`）を通るものだけ返す。フォルダ全体を歩かず、中の HTML は読まず（題は名前）、HTML ごとの状態を持たない。管理ページは開いた所を localStorage の `list.open`（`slug` か `slug:パス` の配列）に持ち、URL に書かない（`open` はフォルダの URL からの 1 回だけ受け、覚えて URL から外す）。中の行は `arrange` の外に置き、開け閉めでは、その下の行だけを足し外す（ほかの行は作り直さない）。枠の左の木も同じ `?dir=` で読み、幅と畳んだかは見る人の都合としてブラウザ（`localStorage` の `frame.side`）にだけ持つ
- **ページの中身と版は 1 か所ずつ**：中身は `server` の `content`、版は `version`（印は `files.Mark` 1 つ）。版は中身の前に取る。live reload は描かない
- **CSP とサンドボックスは返すときに付ける**：ページとその下のファイルはヘッダーで。ヘッダーの無い出口（render・ダウンロード）は、`CutOffPage` が描いたページに CSP の meta を足し、ページの charset の meta を全部外して、floe の charset 1 つを頭の先頭に置く（サーバーが返すページは、page.html の charset が残ってもヘッダーの utf-8 が勝つ）
- **描くものはまとまり 1 つ**：サーバーは `Config.Drawing` で読んだ Kit とアイコン集を 1 つ持ち、自分の 3 つの根とアイコン集を 1 つのタイマーで毎秒見張り、変われば丸ごと読み直す。見張りが後で直せない描き（ストアに書くものと download）は、描く前に同じ見張りをその場で 1 回走らせる（`latest`）。読めなければ `store.Drawing.Err` で描く出力を全部止める
- **API の誤りは `refuse` 1 つ**：usererr の Report を返し、409 は `store.Conflict` で見分ける。プロトコルの断り（forbidden host・origin・not found、`/api/stop` が名指す pid が自分でないときの 409 など。floe のコマンドが読み、ユーザーに見せない）は英語の固定の文
- **書き込みの本文は `decodeStrict` 1 つで読む**：API の書き込み（POST・PUT・PATCH）の本文は、送る形の型（`Link`・`Upload`・`Edit`・`Stop`、rebuild は欄の無い型）に、`decodeStrict` が encoding/json/v2 だけで読む（構文の確かめも v2 に任せ、v1 で読み直さない）。構文の誤り、型に無い鍵（大文字小文字だけが違うものも）、同じ鍵の 2 つ目（入れ子の map の中も）、文字列の中の孤立したサロゲートと UTF-8 でない字、v2 が読まない形の値（改行の入った base64、数の配列の `[]byte`）は、encoding/json/v2 の言葉のまま 400 で断り、object でない本文（null も）も断る。floe のコマンドがサーバーに送る要求は `daemon.Request` 1 つで組み（Origin・Accept・Accept-Language・Content-Type）、本文は送る形の型を encoding/json/v2 の `Marshal` で作る。CLI はユーザーの値を確かめる `encode` を通し、UTF-8 でない字は、publish がサーバーを起こす前に、渡した所の名前の誤りにする。nil の map は null で送り、CLI が意図して送る空の字は落とさない（送る型のポインタの欄は `omitempty` でなく `omitzero`）。値が null の欄は、送らないのと同じ（変えない）。受ける鍵は型の欄にだけ書き、本文を map で読んで鍵を拾わない
- **管理ページと枠は `ui.Page` 1 つ**：`/` と `/a/{slug}/` は、読み手の言語で描いた同じ管理ページ（無いページは 404 の枠）。ページそのものは `?raw` で、枠の iframe が読む。管理ページと枠には `uiCSP`（frame-ancestors）を付け、枠に入れられるのは自分と VS Code だけにする。枠の中のリンクは `leaveFrame` 1 つが扱う。枠の形を印刷すると、iframe は 1 画面分しか紙に出ないので、ページの一部を黙って印刷せず、バー・枠・トースト・ページの選びを隠して、ページだけ（`?raw`）を開いて印刷するよう言う（ui.html の `@media print` 1 か所。画面は変えない）
- **管理ページの色は明るい 1 組**：色はすべて `ui.html` の `:root` の変数から取る。暗い画面・その切り替え・`prefers-color-scheme` と `color-scheme` の決まりは作らない（ユーザーが決めた。複雑になるだけ）。`color-scheme` を付けないので、枠の中のページは OS の設定どおりに `prefers-color-scheme` を読む
- **一覧の並びは `arrange` 1 つ、段と色の順は `projectOrder` 1 つ**：一覧も枠の前後の矢印も `arrange` の順で、段（お気に入り・プロジェクト・記録なし）は `group` が決める（お気に入りの鍵は `null` で、プロジェクトの名前と重ならない）。段の順とプロジェクトの色（6 色を順に）は、全部の項目（ページとリンク、アーカイブも）を数えた `projectOrder` だけが決め、絞り込みやタブで変わらない
- **管理ページの字の濃さはブラウザで測る**：管理ページの字（字のノード・`::before`・`::after`、入力の値・placeholder・select の字）は、地との濃さの比が WCAG AA に届くことを、`TestUIColors` が測る。字でない印は測らず、data URI に写した色（チェックの印・並べ替えの矢印）だけを別に確かめる
- **ブラウザのテストは `internal/browsertest` に置く**：Chrome は 1 つを起こし、全部のテストが順に使う。サーバーは `internal/servetest` で本当のポートに起こし、publish は CLI（`cli.Run`）で行う。管理ページの動きと見た目でブラウザで確かめない所は、そのパッケージの `LIMIT:` にある
- **偽の一覧はページより前に**：管理ページの一覧を偽のものにするのは `(*browser).stub` 1 つで、ページの script より前に走る script で `fetch` を替える。最初の `stub` は空のタブで、偽にするページを開く前に呼ぶ（開いた後に替えると、ページがもう出した読みが偽を上書きする）。本物の一覧に戻すのは `unstub`
- **動くスタイルは止まってから読む**：ブラウザのテストで、transition・animation のかかる性質の値を、期待を知らずに読んで記録するときは `settled` で読み、動きが止まるのを待つだけのときは `still` で待つ（どちらも、ページ全体の終わりのある動きを待つ）。期待の値まで待つ読みは `until` のまま。決まった時間の sleep で動きを待たない
- **Chrome は版を決めて入れる**：テストは `FLOE_CHROME`、無ければ PATH で Chrome を探し、手元で見つからなければ飛ばす。CI では、版を決めた Chrome for Testing を `FLOE_CHROME` で渡し、無ければ落とす。版は go-ci.yml の `CHROME_VERSION` 1 か所で、上げるのは手で
- **組み込みのテンプレートは汎用**：floe の組み込みのテンプレートは、どのプロジェクトでも使う一般的な文書の形にする。floe の開発の流れだけの段は決まった段にせず、任意の節に書かせる。見本（example.yaml）の中身も一般的な題材にする。必須は、その文書が成り立たない段だけにする
- **組み込みは schema だけで断る**：組み込みのテンプレートは checks.yaml を持たず、断るのは schema（必須・型・知らない鍵・空の字）だけ。見れば分かる誤り（判断に推しが 2 つ、など）は `LIMIT:` にする
- **本文の列は 1 つの書き方**：どのテンプレートも、本文の列を `max-width: calc(var(--page-width) + 2 * var(--page-pad)); margin-inline: auto; padding-inline: var(--page-pad)` で書く。`--page-width` と `--page-pad` はパレットの 1 か所だけに置く。1 列のページは max-width の幅だけを `--text-width` から決める
- **部品はパレットの変数をそのまま読む**：部品・見た目の CSS は、パレットの変数を `var(--accent)` のように既定の値なしで読み、パレットの値を写さない。パレットの根は `:scope`（ページでは `:root` と同じ要素）。パレットの無いページへ出す floe の CSS（今は `floe diagram render` の断片の `diagramAssets`）は、default の palette.css を `draw.InScope` で自分の根（`diagram.Roots`）だけに当て、`draw.Sheet` で層 `floe.palette` に入れて持ち、貼った先のページの図の外を変えない。`@scope` を読まないブラウザでは色が付かない（`LIMIT:`）。`TestUIDiagramFragment` が確かめる
- **tone は `data-floe-tone` と、floe が tone.json から組み立てる 2 つの変数**：tone を描く要素は、class でなく `data-floe-tone` で印を付ける（部品の part.html、diagram の Go、テンプレート、データの html）。tone の名前を色に写すのは、draw の style.go の `Tones` が tone.json の enum から組み立てる規則だけで、層 `floe.palette` のテーマのパレットの後ろ（`floe diagram render` の断片では図の根に）に入り、印の要素とその中の部品の箱でないものに `--tone`・`--tone-soft`（パレットの `--<tone>`・`--<tone>-soft`）を置き、ページの根と部品の箱には accent のものを置く（部品は外の tone を継がない）。部品・テーマ・テンプレートは 2 つを読み、tone ごとのセレクタは、tone の色でないもの（印の字、ある tone だけの描き分け、テーマの個性の色）にだけ書く。テーマが tone の無いものを自分の色で描くときは `:scope:is(:not([data-floe-tone]), [data-floe-tone=accent])` に書き、名指さない tone を floe の色のままにする。tone の無いものを別の 1 色で描く部品は根に `--tone` を置き（stats）、形ごとに別の色で描く部品（今は diagram）だけが、根で 2 つを `initial` にして既定を `var()` の後ろに書く。tone に無い値は外の tone で描かれ、断らない（`LIMIT:`）。`TestUITone` が全テーマで確かめる
- **部品の箱と見た目の並びは floe が外す**：部品の箱か、見た目を借りる `ul`・`ol`・`menu` の印・左の余白・項目の間は、draw の style.go の `boxes` が、floe.parts・floe.looks の一番下の層（boxes、深さの層より下）で外し、grid で並べる。部品と見た目は、並びの見た目を外す規則を書かない。page.css とテーマの一般の並びの規則は、部品の箱の並びの 4 つの性質（display・印・左の余白・項目の間）に届かず、見た目を借りる要素の 4 つにはテーマのそれが届かない（page.css は届き、借りるテンプレートが整える）。margin や、項目（li）と `::marker` への規則は、今までどおり届く。`TestUILists` が確かめる
- **色で塗った帯は見た目 `band` 1 つ**：テンプレートは、`--accent` で塗った帯を page.css に書かず、`data-floe-look="band"` で借りる。page.css が書くのは帯の余白と中の並びだけ
- **塗った面は `--face` と `--on-face`**：地と字の色を自分で決め、中にリンク・行の code・フォーカスの輪が来る面（見た目 band、テーマの注意・表の見出し、ink・brutal の引用、部品 code など）は、`--face`（地）と `--on-face`（字の色）を置き、`background: var(--face); color: var(--on-face)` で塗る。中のリンク・code・輪の色は base.css 1 か所が 2 つの変数から決め、面ごとに書かない。部品の中身は base.css が変数を外す（`:where([data-floe-part]) > *`。部品は自分の地で描き、部品の箱は外の面の中で、輪も外の面の色）ので、部品の根を面にするときは根と根の子に置く。例外：明るい地で、その上で `--accent` のリンクが 4.5:1 に届く要素（default・soft の引用など）は、面にせず、子で変数を外して自分の地で描いてよい。届かない明るい地（swiss の注意の灰色、brutal の黄色の引用など）は面のまま。届くかは `TestUIContrast` が every-part の注意と引用の中のリンクで測る。地を塗らずに字の色や code の地・リンク・輪の色を書くときは、面の中に来うるなら `var(--face, …)`・`var(--on-face, …)` で書く。印刷でも地を画面どおりに出すのは、base.css の `:root` の `print-color-adjust`（と `-webkit-` の形）1 か所で、部品・テーマ・テンプレートは書かない
- **テーマの base.css はページの段に**：テーマの base.css は、`floe.base` の直後の層 `floe.theme` に、テンプレートの page.css と同じ範囲（部品の箱まで、中には入らない）で入る。テンプレートの page.css、見た目、部品はどれもそれに勝つ。部品の中の Markdown は floe の base.css で描く。テーマが部品の中を変えるのは、部品・見た目ごとのファイルだけ。`TestUILayers` が確かめる
- **入れ子は置く側が勝つ**：見た目の中の見た目、部品の箱の中の部品の箱では、外の要素に当たる part.css（floe のものも自分のものも）とテーマのファイルが、中の要素に当たるものに、セレクタによらず勝つ。floe が要素ごとの入れ子の深さで層を分けて決める（`draw.pageStyle`）。`TestUINesting` が確かめる
- **見た目と部品の間は層の順**：見た目は、テンプレートの page.css と部品に負ける（中の部品の箱にも）。同じ要素が借りる見た目は、テーマの見た目のファイルが、借りる部品の part.css に勝つ
- **部品の印刷はテーマが崩さない**：同じ深さの要素の上では、テーマの部品・見た目ごとのファイルは部品の part.css の後ろに入り、part.css の `@media print` にも勝つ。部品が印刷で外す画面だけの状態（選んだタブ）を塗るときは、`@media screen` に入れる。入れ子では「入れ子は置く側が勝つ」が先に効き、外の部品の part.css（`@media print` も）は、中の部品の箱へのテーマのファイルに勝つ（tabs の印刷の `:scope > div` は、テーマの stack.css に勝つ）。`TestUIPrint` が tabs を全テーマで確かめる
- **番号と線の手順は見た目 `numbered` 1 つ**：部品やテンプレートは、丸い番号と次への線を part.css・page.css に書かず、`numbered` を借りる。自分で書くのは、丸と線の色（状態など）と中の並びだけ
- **行の印は最初の行のベースラインに**：字でない行の印（丸・点・箱）を描く行の箱は grid にし、名前の付いた列 `mark`（幅 `auto`）と `text`、名前の付いた行 `mark` を持つ。印は `::before` を `grid-area: mark` に置き、大きさは枠を含めて `--floe-mark`（部品が印に `box-sizing: border-box` を書く）、字との間は印の `margin-inline-end: var(--floe-mark-gap)`。中身は `grid-column: text` で、印も中身も `align-self: first baseline`（印が行より高ければ行が伸びる）。中身の間は grid の gap でなく `> * + *` の `margin-top`。次の印への線は `::after` を、anchor positioning で印の下の縁から（印に `anchor-name: --floe-mark`、行に `anchor-scope: --floe-mark`）、上も下も 0.35rem あけて引く。`top` を行の余白を前提にした数で書かない。テーマは印を `content: none` で消し、大きさと間を 2 つの変数で変える。字だけの印は字の流れの中に置き、この決まりの外。`TestUIMarks` が全テーマで測る。直書きの字・字の無い最初の中身・anchor positioning を読まないブラウザは `LIMIT:`（numbered の part.css）
- **部品の間は `--floe-room`、箱の外へ出る飾りはそれ以下**：部品が並ぶ所（テンプレートの節の中、手順の中、stack、tabs の名前と中身の間、cards の間）は、どこも base.css の `--floe-room`（0.75rem、1 か所）をあけ、数で書かず `var(--floe-room)` を読む。部品 text の Markdown（段落・見出し・並び）も部品として並ぶので、その間も同じ。部品の中の Markdown の流れ（段落の間と、その中の部品の箱の前後）の間は、floe が 1 か所で持つまで、部品が決める（`LIMIT:`）。テーマは部品の間を書かない（テーマの cards.css・tabs.css が根の `gap` を書くと黙って勝つ。`LIMIT:`）。見出しの上の間（テンプレートの節の中の節など）は、部品の間でない。`TestUIRoom` が全テーマと全テンプレートで測る（テンプレートの、Markdown だけを置いた所と、部品の中の Markdown の流れは測らない。`LIMIT:`）。箱の外へ描く飾り（影、はみ出す `::before`、いつも描かれる outline）は、四方とも `--floe-room` 以下で、箱の大きさによらない大きさで描き（傾き・拡大のように箱の大きさで外へ出る分が変わる変形は使わない）、そのための場所（margin・padding）は空けない。影は、テーマがパレットに `--floe-shadow-x`・`-y`・`-blur`・`-spread`（px か rem）で 1 組だけ置き、影を落とす箱に `--floe-shadow-color`（継がない）を、箱の中の影は `--floe-inset`（継がない、初期値 `0 0 #0000`）を置く。部品の箱・見た目を借りる要素・引用の `box-shadow` は base.css の 1 つの規則だけが書く（テーマ・部品・テンプレートは書かない。書くと base に勝って影が黙って消える。`LIMIT:`）。部品の根は margin を書かない（部品の根になりうるブロック、`p`・`ul`・`ol`・`h1`〜`h6`・`hr`・`figure`・`dl`・`blockquote`・`pre`・`form`・`menu`・`fieldset` の margin は base.css が 0 にし、置く側の間が効くように）。部品の箱でない要素（kbd、diff の行、番号の丸、行の中の code、タブの名前）の `box-shadow`、行の中の字の飾り、フォーカスの輪は外。`TestUIShadows` が全テーマと全テンプレートで、4 つの幅で測る
- **目次の id はテンプレートが付ける**：テンプレートが自分の段へリンクするときは、page.html が言語によらない決まった id を見出しに付ける。floe が見出しの字から作る id に頼らない
- **組み込みの言葉はそろえる**：floe の words.yaml は、どれも en を持ち、どの言語も en と同じ鍵を持つ（テストが floe の下の words.yaml を全部探して確かめる）

## 誤りの出し方

- **作る・包む**：ユーザーが直せる誤りは `usererr.Error` にする。ライブラリの誤りは `usererr.Library` で包み、文を言い直さない。例外：ライブラリの文が、直し方も場所も言わないときだけ、別のライブラリの文か floe の文に替えてよい。今これに当たるのは、schema の `read`（空のファイル・値の後ろの余り）と、icons の取得と読み（公式ページ・URL・ダウンロード・zip・index.json・置き換えの失敗）だけ。`fmt.Errorf` は、floe の中のもの（埋め込みのファイル、floe の不具合）の失敗だけに使う（ユーザーのために使うパスは「読めない・書けない誤り」）
- **文を足さない**：usererr の誤りを `fmt.Errorf` で包んで文を足さない（`Tell` は包んだ側の文を言わない）。場所や名前は usererr の引数か場所に入れる
- **1 回だけ言う**：言葉にした後の場所と文が同じ誤りは、`Tell` が 1 回だけ言う。どこで作ったかによらない
- **出力の全体の誤り**：どの入力を直しても消えない誤り（アイコン集が無い、設定のディレクトリの誤り）は、作る時に `usererr.NewWhole` で作る。どの入力にも置かれず、場所の無いまま `Tell` が出力の中で最初に出た所で 1 回だけ言う。並べ替えの仕組みは足さない：1 つの入力の中では、ほかの場所の無い誤りと同じく `Sorted` が並べる（最初の場所の無い誤りの所にまとめ、その中は鍵と引数の順）
- **場所を付ける**：場所は、作る時（usererr の作る関数）か、`(*Error).PlacedAt`・`PlacedIn` でだけ付ける。印のある誤り（`NewWhole`）には付かない。`PlacedIn` は、ファイルの場所をもう持つ誤りにポインタを足さない。場所の欄は usererr の外から見えず、`File()`・`Line()`・`Column()`・`At()` で読む。誤りを `Key`・`Args` から作り直して場所を付けない（印が落ちる）
- **ファイル全体の誤りはファイルだけに**：パスが YAML のファイルの根の下の書いた値に 1 つも着かない誤り（根を指す：根の鍵が足りない、for の無い check、空のファイル。1 歩目の鍵が書かれていない：根に無い `field:`、図の見本の中）と、1 つの所に置けない誤り（propertyNames の名前が 0 か 2 つ以上）は、行と列を付けず、ファイルだけに置く（`d.yaml: …`）。最初の鍵や 1:1 に置くと、その行の誤りに見える。置き場所は `yamlpos.Locate` 1 つが決め（根は `*ast.MappingNode` で、空のファイルは nil。パスを辿る `find` は yamlpos の外へ出さない）、schema・doc（`PosOf`）・rules・`yamlpos.Place` はどれもそこを通る。データファイルと図の spec の頭は `yamlpos.ParseMap` 1 つで読む（空のファイルは `{}` と nil の根）。根の値の形そのものが違う誤り（データファイルと図の spec は goccy、floe の自分のファイルは checks.yaml・computed.yaml・words.yaml・part.yaml）は、書いた値の誤りなので、根の値の始まりに置く。ファイルを持つので、ファイルを持たない「出力の全体の誤り」（`NewWhole`）とは別で、`Sorted` がそのファイルの先頭に並べる
- **描くときの誤りの場所**：html/template の Execute の誤りの場所は `draw` が置く。置く所は誤りの種類で決まる。floe の失敗（`kit.ErrFunc`）のほかは、文の頭を切り、後ろのライブラリの言葉は変えない
  - page.html の誤り：page.html の行と列（下の「テンプレートの誤りの場所」）
  - 部品の part.html の誤り（part.html の text/template の実行の誤り（`map has no entry` など）、words.yaml に無い言葉）：part.html の行と列。floe の部品も自分の部品も分けず、部品を何か所で使っても 1 回だけ言う。行を読めないときは、その部品が描く値のデータの位置に置く
  - 部品のファイルの位置を持たない誤り（floe の関数が値を断る誤り、行を読めない実行の誤り）：その部品が描く値のデータの位置（出力の全体の誤りは置かれない）。部品の関数が値の中の誤りを返すときは、その `At` を部品の値の根からの JSON ポインタにし（関数の引数の値からのポインタにしない）、draw が `yamlpos.Place` で部品の値の下の行と列に置く
  - floe の関数の中の floe の失敗（`kit.ErrFunc`）：floe の失敗のまま、文の頭も切らずに出す
  - html の項目を字や属性の中に置く誤り（`draw_html_placed`）：置いたのが page.html か部品の part.html かは印から分からない（分けるには部品の出力にも印が要る）ので、どちらでもデータファイルの `template:` に置き、文がどちらかと言う（`LIMIT:`）
- **テンプレートの誤りの場所**：text/template の文の頭（実行の `name:行:列: `、構文の `name:行: `）を読むのは `textpos.OfTemplate` だけで、読んだ場所は `Line`・`Column` に入れ、文に残さない。構文の誤りは列を付けず、行の中の CR だけの改行の後は言えない（`LIMIT:`）。ライブラリが言うバイトの位置は `len(src)` より前だけを場所にする（例外は構文の誤りの行の頭で、src が空でなければ終わりも受ける）。html/template のエスケープの誤りは Node の位置を先に使い、Node が無ければ `Error.Line` を text/template の行として `textpos.TemplateLine` で写す
- **CLI の誤り**：コマンド行の誤りは、cobra の文に `cli_usage` で「run floe --help」を添える（cobra の文は直し方を言わない）
- **読めない・書けない誤り**：floe がユーザーのために使うパス（コマンド行で渡したパス、設定のディレクトリ、既定のディレクトリ）の下のファイルを読めなければ `usererr.CannotRead`（システムの文のまま、そのパスに置く。`gone.yaml: no such file or directory`）、書けなければ `usererr.CannotWrite`（`cannot_write`：書けないことと確かめる所を言い、括弧の中にシステムの文を残して、そのパスに置く。ユーザーが決めた。書けない理由はディレクトリ・権限・名前の長さなどで、システムの文だけでは何を直すかが分からない）で言う。ふつうのファイルでもディレクトリでもないもの（FIFO・デバイスなど）は、`CannotRead` が `usererr.ErrNotFile` を見て `kit_not_file` で言う。呼び出し元は言い換えず、`CannotRead` に渡す
- **警告の出し方**：警告も `*usererr.Error` で作り、誤りと同じ関数で場所を付ける。`error` にも、`error` である `usererr.List` にも入れず、`[]*usererr.Error` の別の返り値で運ぶ（`tmpl.Draw` から `store.Entry`・`server.Saved` まで）。言葉にするのは `usererr.Warn`（並べて 1 回だけ言う）、並べるのは `Report.Render`（誤りの後に、頭に `warning: ` を付けて。floe の失敗のときは失敗だけ）。描けなかった入力の警告は無く、ほかの入力の誤りがあっても、描けた入力の警告は誤りと一緒に出す。描いた後に断るとき（publish の 409 など）は、`usererr.WithWarnings` で警告を断りに添え、`Tell` が誤りの後に言う（管理ページへの断りには入れない）。警告を添えた断りを `List.Add`・`Collect` に渡すと、警告を黙って落とさず、floe の失敗になる。CLI は警告を `Run` の出口で 1 回出す（手元の警告は `Warn` を 1 回だけ呼ぶ）。rebuild とページを開くときの描きは、警告を捨てる
- **出す**：誤りを言葉にするのは `usererr.Tell`、並べるのは `Report.Render` だけ
- **目録の鍵は英語と日本語の両方**：目録の鍵は、それを使うコードと同じ PR で、英語と日本語の両方に入れる

## floe の不具合や困ったことは、ユーザーに伝える

floe はエージェントが使う道具なので、エージェントが使っていて困ったことがそのまま直すべき点になる。次のどちらの場面でも、次のようなことに気付いたら、ユーザーに伝える。

- **使っているとき**：データファイル・テンプレート・部品・テーマでページを作っているとき
- **直しているとき**：floe 本体を直していて、頼まれた作業とは別の不具合に気付いたとき（試した入力で検査が漏れた、など）

気付くこと:

- 誤りの文が、何をどう直せばよいかを言わない（推測で直した）。行と列が原因を指していない
- 検査を通ったのに、ページが崩れて描かれる、または後の段で失敗する（検査の漏れ）。逆に、正しいデータを断る
- 同じものを何度も書き写す、回り道をしないと書けないなど、書きづらい

伝え方:

- 気付いた時点で、作業の報告の中で伝える。作業の終わりまでためない。
- 回避して作業を続けたときも、回避したことと、その方法を伝える。黙って回避しない。
- データファイルの断片と floe の出力をそのまま添え、どこで困ったか、直し方の案があればそれも書く。
- floe 本体を直すかはユーザーが決める。頼まれた作業の中で、断りなく直さない。直している途中で見つけた別の不具合も、今の PR に混ぜない。

ページにも書く:

- 載せるかはユーザーに聞かない。使っているときに見つけたものも、直しているときに見つけたものも、気付いた時点で載せ、報告の中で載せたことを伝える。
- 伝えた問題は、floe のページ「floe の不具合と困ったこと」（slug は `floe-sc-usage-issues`、データファイルは `~/.local/share/floe-plans/floe-sc-usage-issues.yaml`）に追記し、`floe publish` で公開し直す。問題ごとにページを増やさない。番号は `S` を付けて見つけた順にする。
- 置き換える前の develop の問題は「floe の不具合と困ったこと（旧 develop）」（`floe-usage-issues`）にあり、もう追記しない。
- 先頭のまとめの表に、今の列のとおり 1 行足し、項番は S を付けた番号を問題の文と分けて書く。直す PR には、直す予定の PR か「未定」を書く。問題の節を 1 つ足して「何が起きるか」（データの断片と floe の出力）、「なぜ困るか」、「どう直すか」を書く。
- 表の行と問題の節は、項番の順に並べる。
- 種別は 1 つの pill で、表の下の注「種別と場所の見方」の言葉と色（バグ・改善・リファクタリング・ドキュメント・案）から選ぶ。場所は直す所を書く。
- すでに載っている問題なら、その節に新しい例を足す。直ったら、「残っている問題」の表の行と問題の節を消し、「直した問題」の表に、今の列のとおり 1 行足す（直した PR には何をしたかを一言）。
