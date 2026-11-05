# CLAUDE.md

floe はエージェントが書いたページをローカルサーバー（`http://127.0.0.1:7325`）で表示・管理する Go の単一バイナリです。データ（YAML）とテンプレートから文書を組み立て、検査付きの inline SVG 図を描きます。

- 検証は `just ci`（CI と同じ内容）。golangci-lint は `just setup` で入る。
- 対応 OS は linux と darwin（amd64・arm64）。Windows はリンクのパス検査などの信頼境界を検証できないので対応しない。
- 決まりは、それを入れた PR がこのファイルに足す。足すのは、読まないと書くコードを誤ることだけで、1 つの決まりは数文にする。書き手に見える動きは docs に、理由と限界はコードのコメントと `LIMIT:` に、測れることはテストに書き、ここには書き写さない。決まりや動きを変えたら docs も直す。

## 検査の方針

floe が入力を断る検査は、次の 4 つのどれかに当たるものだけにする。当たらない検査や特例は入れない。

- **ライブラリが返す誤り**：YAML・JSON Schema・html/template・expr などが返す誤りを、ライブラリの言葉のまま出す。言い直さない。場所は、ライブラリが返す分だけ付ける（JSON ポインタを YAML の行と列に写すのはよい）
- **信頼境界**：パス、CSP、サンドボックス、大きさなど、floe の外へ出る・外から入るものを守る。文脈によらない保守的な決まりで断る
- **floe 自身の仕組みの確かめ**：floe が置いた印や、floe がスタイルを入れる所など、floe の仕組みが前提にするものを確かめる
- **ライブラリの黙った読み違い**：ライブラリが誤りを出さずに、書いたのと違う値を読む所を断る。今あるのは、YAML（2 つ目の文書、`#` で値が切れる、`{a, b: 1}`、文字でない鍵、`.inf`・`.nan`、YAML 1.2 と読みが違う数、先頭のほかの BOM、UTF-8 でないバイト列、字下げの数を書かない `|`・`>` の字下げの後のタブ。`yamlpos.Parse`）、schema の JSON（同じ鍵、先頭のほかの BOM、孤立したサロゲートと UTF-8 でない字。`read`）、API の書き込みの本文（大文字小文字だけが違う鍵、同じ鍵、孤立したサロゲートと UTF-8 でない字。`decodeStrict`）、html/template の `ZgotmplZ`。決まりを 1 つにするため、goccy が正しく読む入力も同じ形なら断る（`{ }`・`[ ]` の中のコメント、1 行だけの `|` の文のタブ。ユーザーが決めた）

- **警告**：上の 4 つは断る検査で、警告はその外にある。警告にしてよいのは、ページが崩れず、より良い書き方を勧めるだけのもの（勧め）だけで、直さないと困るもの（崩れる、誤った値になる、写すと動かない）は断る。警告は、勧めのとおりに直せる所にだけ出し、終了コードを変えない。見つけるのも断る検査と同じく本物のパーサやライブラリで（Markdown は goldmark の木で）、正規表現で読まない。今の警告は、部品 text の値の中のコードの囲み（`draw_text_fence`）だけで、見るのは `draw.drawItem` の 1 か所。例外：自分の schema が code の部品を置けない所で部品 text を指すと、勧めのとおりに直せないのに、そこにも警告が出る（`LIMIT:`。text の説明と docs が、ただの Markdown の欄に format markdown を勧める）

## 作りの決まり

- **使えない自分のものは止める**：描かずに分かる誤りで使えない自分の部品・テーマ・テンプレートが 1 つでもあれば、その出力を止めて理由を並べる。使った所だけで断る仕組みは作らない
- **集めるのは楽な所だけ**：1 つの段の中で、互いに関係の無い検査は集めて一度に出す。集めるために特例や推しを足さない。例外：ライブラリが最初の誤りで止まり、どれを返すかが決まらない（上流が直さない）ときは、ライブラリが型の付いた誤りで名指した所だけを、意味を作らない形で埋めて呼び直し、誤りを決まった順で集めてよい。回数には上限を置き、埋められない形が混ざると全部は言わない。今これに当たるのは、schema のコンパイルの、解けない `$ref`（`$defs` の直下の 1 段、断った URL、無い部品）だけ（`holes`）
- **外の振る舞いを真似ない**：HTML・CSS・URL・JSON Schema などは、本物のパーサやライブラリを使う。自分で読む作りにしない。仕様が規範として定める表をそのまま写すのは、真似でなく文脈の要らない決まりで、よい（今は yamlpos の数の読みが、YAML 1.2 の core schema の表の正規表現を写し、draw が select の中で断る要素の並びが、HTML の断片を読む algorithm の表を写す）
- **受け入れた退行**：上に当たらない漏れは、`LIMIT:` のコメントと PR の本文に書く。検査は足さない

- **schema の形はコンパイルした木から**：floe が要る形（どの項目がどの部品・format か）は、コンパイルした `*jsonschema.Schema` から取り、schema の JSON を自前で辿らない。生の JSON を見るのは、`$schema`（2020-12・2019-09 だけを受ける）と `$id` の確かめと、`read` の字句の確かめだけで、schema の鍵の意味は読まない。例外：「埋めて呼び直す」ときは、写しに名前を足すために、生の JSON の根と `$defs` の直下を読んで書いてよい（書くのは写しだけ。`holes.fill`）
- **木は Follow で歩く**：schema の木は、全部の欄を辿る `every` か、`schema.Follow` の繰り返しで歩き、検査ごとに歩き方を書き分けない。例外は、`$schema` と `$id` を見る生の JSON の歩きと、`sharedKeys`
- **項目の検査は checkNode 1 つ**：schema の 1 つずつの検査は `checkNode` 1 つで、テンプレートも部品も共有の schema も `checkOwn` の同じ歩きで走らせる。共有の schema は、指す側の歩きでなく、その根から確かめる（指す側は根しか指せない）。指す側の `$ref` が辿られない所（辿らない場所にある、横の format で止まる）は、指す側で言う。テンプレートに足す検査は `checkNode` に足し、部品に写さない
- **値を歩くのは Walk だけ**：データの値を schema に沿って歩くのは `schema.(*Compiled).Walk` だけで、値ごとに `Follow` を 1 回呼び、部品の値はその部品の schema の根から歩き続ける。描く値も、確かめる項目（部品の check を走らせる所）も、Walk から取る
- **part.html は元の値を `written` で読む**：part.html に届く `.` は描いた後の値。描く前の字と比べる部品（table の列の `pills`）は関数 `written` で読み、中の部品の形を自分の schema に写さない。`written` の字は比べるか字のまま描き、Markdown や HTML として描かない
- **部品もテンプレートと同じ format を取る**：部品の schema も floe の format（html・markdown・image）と、それを持つ共有の schema を、floe が辿る所で取る。部品だけの format の決まりは作らない。部品の html の項目も、描いたページの確かめを通る
- **部品は見出しを描かない**：部品は自分の段を知らないので、part.html に `<h1>`〜`<h6>` を書かず、題は `class="title"` の要素で書く。見出しを描くのは page.html と書き手の Markdown だけ。`TestEveryPartDrawsEveryPart`
- **描いたページは x/net/html で読んで書く**：描いたページの確かめ（「データの HTML 項目は描いたページの木で確かめる」が断るもの、部品の名前）は x/net/html の木で行い、スタイルを head に入れるのも、見出しに id を付けるのも、その木を直して `html.Render` で書く。正規表現は、CSS の `</style` の書き換えだけに使う
- **データの HTML 項目は描いたページの木で確かめる**：format html の項目は、描くとき、探りの空白と偽れない印のコメントの対で挟む。断る判断は描いたページの木（`html.Parse` 1 回）だけで決め、項目の文だけを読んで判断しない（svg・math の中でブラウザと読みが違う）。見た後に印と探りを外して書き出し、確かめた木と書き出す木を同じにする。断ると決めた後で原因を名指すためだけに `html.ParseFragment` で読み直してよいが（`named`・`placed`）、結果を断る判断に使わない。`<style>`・`<base>`・`<noscript>`、charset と viewport のほかの `<meta>`、`<select>` の中で中身を字として読む要素と svg・math、属性の `ZgotmplZ` は、描いたページ全体で断る（page.html・part.html・項目のどれが書いても）
- **ユーザーのパスはふつうのファイルだけ読む**：ユーザーが渡したパス（データファイル・spec・zip・ページ・publish のリンク）から読むのは、ふつうのファイルだけで、`files.Open`・`files.Read` を通す。開く前に種類を見て、FIFO・デバイス・socket は開かない（開くと止まる、開くこと自体に副作用がある）
- **自分のものの置き場所は重ねない**：部品・テーマ・テンプレートの置き場所の重なりは、`app.kit` が開く前に、字でなく OS の実体（`resolve`・`within`）で比べて `dir_overlap` で断り、重なった置き場所は開かない。`newOwn` も作る前に比べる
- **render が書くのはページだけ**：書くページのリンクはたどり、たどった先の名前が .html（`files.IsPage`）でなければ、何も書かずに断る。データファイルやリポジトリの外のファイルを書き換えない
- **YAML は yamlpos だけが読む**：YAML のバイト列を goccy に渡すのは `yamlpos` だけ。`yamlpos` の外（テストを除く）から import できる goccy のパッケージは `ast` だけ（depguard の規則 `goccy-in-yamlpos`）。値にするのも `yamlpos.Decode`
- **schema の JSON は schema だけが読む**：schema.json などのバイト列を読むのは `schema` の `read` だけで、外からは `Compile`・`NewResource` に渡す。構文の誤りと黙った読み違いは、`read` が値にする前に断る（ライブラリは後の値や U+FFFD で読む）。jsonschema を import できるのは `internal/schema` だけ（depguard の規則 `jsonschema-in-schema`）
- **段 1 は読むと確かめるに分ける**：kit の `read` は値を作るのに要ることだけをし、値の後の決まりは `check` が持つ。kit の外で Kit を作れるのは `Load` と `Floe()` だけ。実行のときは floe のものに `check` を走らせず、テストで走らせる。例外：後で一緒にコンパイルするものの読み方を変える入力は `read` で断る（今は部品と共有の schema の `$schema` と `$id`）
- **読んだ Kit は変えない**：`Kit` は欄を外へ出さず、`[]byte` も写しを返す。自分のものを重ねるときは、土台を書き換えずに新しい `Kit` を作る（`Floe()` の値をプロセス全体で共有するため）。描くときは、`Template.Page`・`Part.Page` が返す写しを Execute する（Execute は html/template にエスケープの結果を書き込み、その後は写しを作れない）
- **show が出すのは読んだファイル**：`floe part show` などが出すファイルは、読み手が読んだもの（`readFile` が書き留めたもの）だけで、順は種類ごとの並び 1 つ（`unreadIn` も同じ並びを使う）。show のために別に選ばない
- **stack は書いた部品を全部描く**：stack の part.html は部品の種類を書き並べず、report の page.html と同じ繰り返しで描く。置ける部品は stack の schema 1 か所で決める。`TestEveryPartDrawsEveryPart`
- **部品 text はコードの部品と並ぶ Markdown**：部品の並びの Markdown は部品 text で、Go の名前は `kit.TextPart` 1 つ。テンプレートや部品の自分の Markdown の欄は部品 text にしない。text は部品の箱でない外枠 `data-floe-text` に描き、外の面と tone を継ぐ。page.css とテーマの base.css は外枠で止まる
- **Markdown は kit だけが読む**：goldmark を import できるのは `internal/kit` だけ（depguard の規則 `goldmark-in-kit`）。生の HTML を出さない `markdown` を、どこも通る
- **行の余白は `--floe-row-pad` と `--floe-pad-inline`**：線で区切った行の上下と左右の余白はこの 2 つだけが決める。見た目 rows を借りる側は行の余白を書かず、テーマは替えない。`TestUIRowPad`
- **箱の中の余白は `--floe-pad-block` と `--floe-pad-inline`**：部品の間に地や枠を持つ箱を描くときは、縁から字までをこの 2 つで書き、数で書かない（箱の中の小さな升、行、code の行は外）。見た目 panel 自身は余白を持たない。テーマは置き直さず、箱の余白を書かない。`TestUIBoxPad`
- **部品の小さい字は 2 つの段**：本文より小さい字は数で書かず、`--floe-text-small`（札・印と名前・付記）か `--floe-text-aside`（添えの文・小さな題）で書く。code の本文と diagram の SVG は外。テーマは置き直さない。`TestTextScale`
- **折り返す並びは、並びの位置で描き分けない**：幅で折り返す並びの子の形（枠・余白）を `:first-child`・`:nth-child` などで描き分けない（どの子が行の先頭になるかは幅で決まる）。並んだ順に地の色を替えるのはよい。`TestUIWrappedAlike`
- **写すボタンは `copyButton` 1 つ**：part.html の `{{copyButton}}` だけが描き、写す要素は `data-floe-copy` で決める。部品ごとに script・ボタン・言葉・ボタンの形を書かない。部品の CSS が決めるのは置き場所だけで、大きさと間は base.css の `--floe-copy-*`
- **code の一族は code の変数で描く**：code の見た目を借りる部品（diff・terminal）は、行の余白と薄い字を code が根に置く `--floe-code-*` で描く。pre の子を要素の行にし、行に padding を書かず、数と `:has(> .floe-copy)` を写さない（行に書くと code の余白が黙って消える）。テーマは置き直さない。`TestCodeLookCopiesNothing`
- **opacity は 0 だけ**：floe の CSS は `opacity` を 0 にだけ使い、薄く描くのは色（`color-mix(… transparent)`）で書く。`TestOpacity`
- **CSS の記号は読ませず、数は読ませてよい**：`content` は `/` の後ろに読み上げを書く。記号は `/ ""`、数は前の `counter()` の並びで、字の文字列は書かない。記号だけが持つ意味は `.floe-hidden` の見えない字で書く（言葉は words.yaml）。`TestMarkSpeech`・`TestUISpeech`
- **floe の script は `page.Script` 1 つで組み立てる**：ページに入れる floe の script（図の拡大、写すボタン）は、`page.Script` が関数と JSON の引数から組み立て、コメントだけの行を落とす
- **CSS は字句と括弧の入れ子だけで読む**：CSS は tdewolff の字句と、CSS Syntax の括弧の入れ子（`{`・`(`・関数・`[`）だけで読み、セレクタや値の意味を自前で読まない（`css.Parser` も使わない）。文字列・url・コメントの外のエスケープ（`\`）は断り、解かない。tdewolff を import できるのは `internal/css` だけ（depguard の規則 `tdewolff-in-css`）
- **図の形は schema、配置は Go**：図の spec の形（鍵・型・値の一覧・要るもの）は部品 diagram の schema.json だけが決め、jsonschema が確かめる。Go は確かめた値を読むだけで、Go が持つのは次だけ：名前の解決、線の辺と道を決めること（`route.go`・`shares.go`）、配置の検査（図という機能そのもの。箱の形は `outlineOf` 1 つから取り、描くのも測るのも同じ）、raw の要素（x/net/html で読む）、tree と graph を箱と線に開くこと（`tree.go`・`graph.go`。決定的で、同点は書いた順。read が作った型の構造体を読むだけなので「値を歩くのは Walk だけ」に当たらない）、省いた `width`・`height` を描いた端から決めること（`reach`）。入口は `diagram.Drawer` 1 つで、渡された値をいつも schema で確かめてから読む。配置と形の誤りは spec の中の JSON ポインタで返し、行と列は呼ぶ側が `yamlpos.Place` で付ける。配置の誤りを置くのは `tell` 1 つ
- **graph のアイコンは見えない長方形**：graph のアイコンは、配置の間は見えない長方形（`iconCell`）で、線の道を決めるコードはアイコンを知らない。線の端を置くのは `icons.go` の `settleEnds` 1 か所。graph の `x`・`y` は描いたものの左上（`unseen`）。枠の見出しの場所は `headParts`・`headArea`
- **1 つの辺の線は等分**：1 つの辺に来る n 本の線は辺を n+1 等分した点に置き、点を決めるのは `sidePoint` 1 か所。手で書いた線は `shareEnds`、graph の線は `settleEnds`。dot・行・bar は真ん中のまま（ユーザーが決めた）
- **図の字は描く class で測る**：図の字の大きさ・太字・等幅を知るのは `faceOf(class)` 1 つで、part.css と 1 対 1 に書く（part.css の字を変えたら `faceOf` も直す）。描く所も測る所も同じ class の値を読み、呼ぶ側は太字かを選ばない。`TestUIDiagramFaces`
- **保存したページは描いた HTML を返す**：publish（か `store.Rebuild`）の時に `tmpl.Draw` が描いた HTML を写しに保存し、開くときはそれを返す。開くときに描くのは link したデータファイルだけ。CSP とサンドボックスは HTML に焼き込まない。版を増やすのは、publish で中身（データファイルと読んだ画像。theme は入れない）が変わったときだけで、rebuild・開くときの描き直し・edit・link は版も `updatedAt` も動かさず、前の版は描き直さない。番号は rm の後に戻りうるので、タブの印は番号と時刻で作る。縮め方を知るのは store だけで、縮めるのは lock の外
- **slug と写しのパスは 1 か所で作る**：slug をパスにするのは `store` の `dir`、meta.json の `source` を写しのパスにするのは `copyDir`、版の番号を版のパスにするのは `versionFile` だけで、そこで確かめ、呼ぶ側で別に確かめない。URL の `v` を版の番号として読むのは `versionParam`（保存したページの rest の無い URL だけ）、`diff` の `from`・`to` は `versionNumber`
- **publish は読んだ画像だけを書く**：publish が保存する画像は、描くときに `kit.ReadImage` が読んだ名前のものだけ。送られても読まなかった名前は書かない（信頼境界）。rebuild は前の写しの `images/` を全部引き継ぐ。版の中身の印も、読んだ画像だけから作る。版は画像を別に持たない（描いた HTML に埋め込まれている）
- **保存先を書くのはサーバーだけ**：CLI は API で頼む。publish・link・rm・rebuild・管理ページの変更はどれも API
- **サーバーを止めるのは `/api/stop` 1 つ**：`daemon.Stop` が `POST /api/stop` で、止めたいサーバーの pid を本文に書いて頼み、違えば 409。floe のコマンドはシグナルを送らない（名乗る pid は別のプロセスでありうる）。要求の ctx は `Serve` の ctx で切らない
- **アーカイブは meta.json の `archived` 1 つ**：一覧から外すのはこれだけで持ち、立てる・外すのは `Store.Edit` と publish し直し（`newMeta`）だけ。`GET /api/artifacts` は全部を返し、一覧から外すのは見せる側でする
- **未読は reads.json 1 つ**：未読を持つのは publish で保存したページだけ。書くのは store の lock の中だけ。読んだ印を足すのは `POST /api/artifacts/{slug}/read` 1 つで、本文はページを返したときにサーバーが作った版の印、slug は枠が自分の持つ値から決める（一覧から取った印は送らない）。戻すのは `DELETE` の同じ道 1 つで、枠の画面の操作だけが呼ぶ。新しい版かを決めるのは store の `news` 1 か所で、画面は比べない。トーストは人が開く操作をしたときだけ出す。`since` は 1 度だけ書き、その後は動かさない
- **フォルダの中は開いた所だけ読む**：フォルダは meta.json の `folder` で見分け、名前で種類を決めない（データファイルかは `Meta.IsData`）。中は `link.Dir` が開いたディレクトリ 1 つを `link.within` を通して返すだけで、全体を歩かず、中の HTML を読まず、HTML ごとの状態を持たない
- **ページの中身とタブの印は 1 か所ずつ**：中身は `server` の `content`、タブの印は `version`（ファイルの印は `files.Mark`）で、印は中身の前に取る。返すページはサーバーへのつなぎを持ち続けない（Chrome の 1 つのサーバーへのつなぎは 6 本まで）。ページから枠へ知らせるときは送り先の origin を名指し、枠は自分の iframe の、origin が `null` の知らせで、道が自分の開いた URL と合うものだけを受け、slug は自分の持つ値から決める
- **CSP とサンドボックスは返すときに付ける**：ページとその下のファイルはヘッダーで。ヘッダーの無い出口（render・ダウンロード）は、`CutOffPage` が描いたページに CSP の meta を足し、ページの charset の meta を全部外して、floe の charset 1 つを頭の先頭に置く（サーバーが返すページは、page.html の charset が残ってもヘッダーの utf-8 が勝つ）
- **描くものはまとまり 1 つ**：サーバーは `Config.Drawing` で読んだ Kit とアイコン集を 1 つ持ち、自分の 3 つの根とアイコン集を 1 つのタイマーで毎秒見張り、変われば丸ごと読み直す。見張りが後で直せない描き（ストアに書くものと download）は、描く前に同じ見張りをその場で 1 回走らせる（`latest`）。読めなければ `store.Drawing.Err` で描く出力を全部止める
- **API の誤りは `refuse` 1 つ**：usererr の Report を返し、409 は `store.Conflict` で見分ける。プロトコルの断り（forbidden host・origin・not found、`/api/stop` が名指す pid が自分でないときの 409 など。floe のコマンドが読み、ユーザーに見せない）は英語の固定の文
- **書き込みの本文は `decodeStrict` 1 つで読む**：API の書き込みの本文は、送る形の型に `decodeStrict` が encoding/json/v2 だけで読み（v1 で読み直さない）、誤りは v2 の言葉のまま 400 で断る。受ける鍵は型の欄にだけ書き、map で読んで鍵を拾わない。floe のコマンドの要求は `daemon.Request` 1 つで組み、本文は v2 の `Marshal` で作り、CLI はユーザーの値を `encode` で確かめる。送る型のポインタの欄は `omitempty` でなく `omitzero`
- **一覧の列の幅は `list.columns`、つまみは `dragEdge` 1 つ**：棚・一覧・枠の左の木・差分の目次のつまみは `dragEdge` 1 つで動かし、置き場所は `edgeAt`。幅と畳んだかは見る人の都合としてブラウザ（`localStorage`）にだけ持つ
- **管理ページと枠は `ui.Page` 1 つ**：`/` と `/a/{slug}/` は同じ管理ページ。ページそのものは `?raw` で、枠の iframe が読む。管理ページと枠には `uiCSP` を付け、枠の中のリンクは `leaveFrame` 1 つが扱う
- **開いている版は 1 か所**：右のページは `picked`、全面は `frameAt` の `v` で持ち、ページの URL を作るのは `pageHref`・`rawHref`・`diffHref`・`downloadHref` だけ。全面の `?v=` はサーバーの `frame` が読み、JS は読み方を持たない。版を選んで見ている間は最新を既読にしない
- **帯の操作は 1 つの決まり**：帯に見えている操作は「…」に出さず、帯から外れた操作は必ず「…」に出す。「…」はいつも全部の項目を作り、見えるかは CSS で決める。幅で変えるのは 40rem の 1 か所だけ。行には帯が無いので、行の「…」は全部を出す。`TestUIBarActions` が確かめる
- **管理ページのキーは `KEYS` 1 つ**：キーの表を 1 つ持ち、`pressKey` が引く。ページの中から渡すのは、移ることとモーダルを開くことだけのキー（`PAGE_KEYS`）で、枠は自分の iframe の、origin が `null` の知らせだけを、iframe にフォーカスがあるときだけ受ける
- **管理ページの色は明るい 1 組**：色はすべて `ui.html` の `:root` の変数から取る。暗い画面・その切り替え・`prefers-color-scheme` と `color-scheme` の決まりは作らない（ユーザーが決めた。複雑になるだけ）。`color-scheme` を付けないので、枠の中のページは OS の設定どおりに `prefers-color-scheme` を読む
- **管理ページの角と字は `:root` の目盛り**：ui.html の角は `--corner`・`--corner-menu`・`--corner-small` か形（`0`・`50%`・`999px`）で、字の大きさは `--text-label`・`--text-small`・`--text` で書く。`:root` の外に色を書かない。変数の名前はパレットの変数と分け、default から写さない。`TestManageSheet`
- **版の差分はページの字だけを持ち出す**：版を比べるのは `pagediff.Compare` 1 つで、答えは字だけ。管理ページは `textContent` で描き、ページの HTML を管理ページへ入れない。go-diff を import できるのは `internal/pagediff` だけ（depguard の規則 `go-diff-in-pagediff`）
- **前に読んだ版は `ReadBefore` 1 つ**：差分の起点と知らせの条件を決めるのは store の `ReadBefore` だけで、記録の `n` が最新の版でなければその版、最新の版なら `before`。どちらも今の版に無ければ 0
- **一覧の並びは `arrange` 1 つ、色の順は `projectOrder` 1 つ**：一覧も右のページの `j` `k` も `arrange` の順。全面の `j` `k` は sessionStorage の `artifacts.order` の順。色の順は全部の項目を数えた `projectOrder` だけが決め、絞り込みで変わらない
- **一覧は押している間に作り直さない**：問い合わせと読んだ印の `load` は、`#list` を押してから 2 秒の間（離すまで）は行を作り直さず、次の問い合わせで描く。押下と離す間に行を作り直すと、Chrome は `click` も `dblclick` も出さない。離したことが伝わらない押下があるので、2 秒で止める。操作が呼ぶ `load` は止めない
- **ブラウザのテストは `internal/browsertest` に置く**：サーバーは `internal/servetest` で本当のポートに起こし、publish は CLI（`cli.Run`）で行う。CI は justfile の `browser_looks` に並べた重いテストと、その残りの 2 つのジョブで走らせる。テストを足したら、2 つの時間の釣り合いを手で直す（`LIMIT:`）
- **偽の一覧はページより前に**：管理ページの一覧を偽のものにするのは `(*browser).stub` 1 つで、ページの script より前に走る script で `fetch` を替える。最初の `stub` は空のタブで、偽にするページを開く前に呼ぶ（開いた後に替えると、ページがもう出した読みが偽を上書きする）。本物の一覧に戻すのは `unstub`
- **動くスタイルは止まってから読む**：ブラウザのテストで、transition・animation のかかる性質の値を、期待を知らずに読んで記録するときは `settled` で読み、動きが止まるのを待つだけのときは `still` で待つ（どちらも、ページ全体の終わりのある動きを待つ）。期待の値まで待つ読みは `until` のまま。決まった時間の sleep で動きを待たない
- **Chrome は版を決めて入れる**：CI の Chrome の版は go-ci.yml の `CHROME_VERSION` 1 か所で、上げるのは手で
- **組み込みのテンプレートは汎用**：floe の組み込みのテンプレートは、どのプロジェクトでも使う一般的な文書の形にする。floe の開発の流れだけの段は決まった段にせず、任意の節に書かせる。見本（example.yaml）の中身も一般的な題材にする。必須は、その文書が成り立たない段だけにする
- **組み込みは schema だけで断る**：組み込みのテンプレートは checks.yaml を持たず、断るのは schema（必須・型・知らない鍵・空の字）だけ。見れば分かる誤り（判断に推しが 2 つ、など）は `LIMIT:` にする
- **本文の列は `.floe-column`**：テンプレートの本文の列は class で付け、列の字（max-width・margin-inline・padding-inline）を page.css に書かない。幅は `--column-width` で、置くのは page.css だけ（テーマは置かない）。`--page-width`・`--text-width`・`--page-pad` はパレット 1 か所だけ。字の大きさはページで決めない。列は見た目にしない
- **部品はパレットの変数をそのまま読む**：部品・見た目の CSS は `var(--accent)` のように既定なしで読み、値を写さない。パレットの根は `:scope`。パレットの無いページへ出す CSS は自分の根だけに当てる（`diagramAssets`。`TestUIDiagramFragment`）
- **図は自分の地を塗る**：部品 diagram の枠は地を `--bg`、字を `--fg` で塗り、下を隠すものも `--bg` で塗る。図の外の地は読まない。`TestUIDiagramGround`
- **tone は `data-floe-tone` と、floe が tone.json から組み立てる変数**：tone の印は class でなく `data-floe-tone`。名前を色に写すのは style.go の `Tones` だけで、部品・テーマ・テンプレートは `--tone`・`--tone-soft`・`--tone-fill` を読む。tone ごとのセレクタは tone の色でないものにだけ書く。テーマが自分の色で塗るときはパレットに `--<tone>-fill` を置く。`TestUITone`
- **部品の箱と見た目の並びは floe が外す**：`ul`・`ol`・`menu` の印・左の余白・項目の間は style.go の `boxes` が外し、grid で並べる。部品と見た目は並びの見た目を外す規則を書かない。`TestUILists`
- **色で塗った帯は見た目 `band` 1 つ**：テンプレートは、`--accent` で塗った帯を page.css に書かず、`data-floe-look="band"` で借りる。page.css が書くのは帯の余白と中の並びだけ
- **塗った面は `--face` と `--on-face`**：地と字の色を自分で決め、中にリンク・行の code・フォーカスの輪が来る面は、2 つの変数を置いて `background: var(--face); color: var(--on-face)` で塗る。中の色は base.css 1 か所が決め、面ごとに書かない。部品の中身は base.css が変数を外すので、部品の根を面にするときは根と根の子に置く。自分の地を塗る見た目・部品は、地と一緒に `color: var(--fg)` を書き、面の変数を外す。地を塗らずに字や輪の色を書くときは、面の中に来うるなら `var(--on-face, …)` で書く。部品の箱の字の色を外から直す規則は書かない。印刷の地は base.css の `print-color-adjust` 1 か所。`TestUIBand`・`TestUIContrast`
- **CSS の勝ち負けは層で決まる**：セレクタの詳しさでなく、floe が並べる層で決まる（順は `draw.pageStyle` のコメント。`TestUILayers`・`TestUINesting`・`TestUIPrint`）。テーマの base.css はページの段で部品の箱と text の外枠で止まり、Markdown の見た目は markdown.css に書く（ページ全体に届くので、部品が描かない要素だけ）。部品・見た目の名前に `base`・`palette`・`markdown` は使えない
- **状態と頭はテーマが変数で塗る**：表の頭、選んだタブ、code の頭は、部品が `var(--x, 既定)` で読む変数を、テーマがその部品のファイルの `:scope` に置く。テーマは状態（`:checked`）と印刷を知る規則を書かない。画面だけの状態は部品が `@media screen` で塗る
- **テンプレートが部品の形で描くものは、部品の箱として置く**：札や注意の囲みは見た目として借りず、page.html に部品の箱（`data-floe-part="cell"`・`"note"`）で置き、色は `data-floe-tone`。page.css は箱の置き場所だけを書く。書き手の自由な字は cell にしない（長さに上限が無い）。`TestUIPills`
- **番号と線の手順は見た目 `numbered` 1 つ**：丸い番号と線を part.css・page.css に書かず、`numbered` を借りる。丸の色は tone と `data-floe-mark` で決め、`::before`・`::after` の色を上書きしない
- **行の印は floe が floe.marks に 1 つ並べる**：左に印のある行は part.yaml の `marks:` 1 つ（`box`・`items`・`joined`）で言い、HTML に class を書かない。floe が書くのは置き場所と大きさだけで、色・丸さ・中身は部品・見た目・テンプレートが書く。上の層で行の `display`・`gap`・`grid-template` を書かない（黙って勝つ）。中身の間は `> * + *` の `margin-top`。テーマは印を `content: none` で消し、大きさと間は `--floe-mark`・`--floe-mark-gap` で変える。1 つの字・題の前の飾りと `::marker` は外。`TestUIMarks`
- **横に流れる箱は、画面で中身の幅を外へ言わない**：横に流れる箱は `overflow-x: auto` と `contain: inline-size` を `@media screen` の中の 1 つの規則に書き、紙では中身を折り返す。中で地を塗る要素は中身の端まで伸ばす。中身で幅が決まる所には置かない（画面で潰れる。例は base.css）。`TestUIWrap`・`TestUIPrint`
- **部品の間は `--floe-room`、箱の外へ出る飾りはそれ以下**：部品が並ぶ所と Markdown のブロックの間は `var(--floe-room)` で、数で書かない。部品の根は margin を書かない。部品の中の Markdown は `MarkdownFlow`、テンプレート自身のものは `kit.Markdown` が包むので、部品は流れの中の間を書かない。page.css は自分の見出しの規則を包みの外の形（`header h1`・`section > h2`）で指す。箱の外へ描く飾りは四方とも `--floe-room` 以下で、傾きや拡大を使わず、場所を空けない。`box-shadow` は base.css の 1 つの規則だけが書き（ほかで書くと影が黙って消える）、テーマはパレットの `--floe-shadow-*` と箱の `--floe-shadow-color`・`--floe-inset` で描く。`TestUIRoom`・`TestUIShadows`・`TestUITemplateMarkdown`
- **角の丸さはパレットの 3 つの変数**：角は `--radius`・`--radius-small`・`--radius-pill` を読むか、形（`0`・`50%`・`999px`）だけを書く。縁に沿う内の箱は `calc(var(--radius) - 間)`。3 つの値はどれも 1 つの長さで、4 つの角が違う形はテーマの自分の変数に置く。`TestRadius`
- **静かな地は `--quiet`**：箱の中の静かな地はパレットの `--quiet` を読み、部品が割合を混ぜない。テーマだけの変数はパレットか base.css の `:scope` に置く（default の palette.css はどのテーマの下にも入るので、default だけのものは base.css に）
- **テーマの飾りは場所を取らない**：padding を広げず、地の色や透明の枠で場所を作らず、今ある余白の上に重ねて描く（地、`--floe-inset`、`::before`）。部品が輪や印を outline で描く箱に、飾りの outline を書かない（輪に置き換わって消える）
- **テーマの飾りは傾けず、言葉を書かない**：テーマの CSS は受ける形だけを書く：変形は `translate` の類と `none` だけ、文字列は記号だけで、字は書体の名前にだけ（CSS の字は words.yaml を通らない）、数え方は decimal の類と形だけ。`TestThemeMarks`
- **目次の id はテンプレートが付ける**：テンプレートが自分の段へリンクするときは、page.html が言語によらない決まった id を見出しに付ける。floe が見出しの字から作る id に頼らない

## 誤りの出し方

- **作る・包む**：ユーザーが直せる誤りは `usererr.Error` にする。ライブラリの誤りは `usererr.Library` で包み、文を言い直さない。例外：ライブラリの文が、直し方も場所も言わないときだけ、別のライブラリの文か floe の文に替えてよい。今これに当たるのは、schema の `read`（空のファイル・値の後ろの余り）と、icons の取得と読み（公式ページ・URL・ダウンロード・zip・index.json・置き換えの失敗）だけ。`fmt.Errorf` は、floe の中のもの（埋め込みのファイル、floe の不具合）の失敗だけに使う（ユーザーのために使うパスは「読めない・書けない誤り」）
- **文を足さない**：usererr の誤りを `fmt.Errorf` で包んで文を足さない（`Tell` は包んだ側の文を言わない）。場所や名前は usererr の引数か場所に入れる
- **1 回だけ言う**：言葉にした後の場所と文が同じ誤りは、`Tell` が 1 回だけ言う。どこで作ったかによらない
- **出力の全体の誤り**：どの入力を直しても消えない誤り（アイコン集が無い、設定のディレクトリの誤り）は、作る時に `usererr.NewWhole` で作る。どの入力にも置かれず、場所の無いまま `Tell` が出力の中で最初に出た所で 1 回だけ言う。並べ替えの仕組みは足さない：1 つの入力の中では、ほかの場所の無い誤りと同じく `Sorted` が並べる（最初の場所の無い誤りの所にまとめ、その中は鍵と引数の順）
- **場所を付ける**：場所は、作る時（usererr の作る関数）か、`(*Error).PlacedAt`・`PlacedIn` でだけ付ける。印のある誤り（`NewWhole`）には付かない。`PlacedIn` は、ファイルの場所をもう持つ誤りにポインタを足さない。場所の欄は usererr の外から見えず、`File()`・`Line()`・`Column()`・`At()` で読む。誤りを `Key`・`Args` から作り直して場所を付けない（印が落ちる）
- **ファイル全体の誤りはファイルだけに**：パスが YAML の書いた値に着かない誤り（根を指す、1 歩目の鍵が書かれていない、空のファイル）と、1 つの所に置けない誤りは、行と列を付けず、ファイルだけに置く（行を付けるとその行の誤りに見える）。置き場所は `yamlpos.Locate` 1 つが決め、データファイルと図の spec の頭は `yamlpos.ParseMap` 1 つで読む。根の値の形そのものが違う誤りは、根の値の始まりに置く
- **描くときの誤りの場所**：html/template の Execute の誤りの場所は `draw` が置く。置く所は誤りの種類で決まる。floe の失敗（`kit.ErrFunc`）のほかは、文の頭を切り、後ろのライブラリの言葉は変えない
  - page.html の誤り：page.html の行と列（下の「テンプレートの誤りの場所」）
  - 部品の part.html の誤り（part.html の text/template の実行の誤り（`map has no entry` など）、words.yaml に無い言葉）：part.html の行と列。floe の部品も自分の部品も分けず、部品を何か所で使っても 1 回だけ言う。行を読めないときは、その部品が描く値のデータの位置に置く
  - 部品のファイルの位置を持たない誤り（floe の関数が値を断る誤り、行を読めない実行の誤り）：その部品が描く値のデータの位置（出力の全体の誤りは置かれない）。部品の関数が値の中の誤りを返すときは、その `At` を部品の値の根からの JSON ポインタにし（関数の引数の値からのポインタにしない）、draw が `yamlpos.Place` で部品の値の下の行と列に置く
  - floe の関数の中の floe の失敗（`kit.ErrFunc`）：floe の失敗のまま、文の頭も切らずに出す
  - html の項目を字や属性の中に置く誤り（`draw_html_placed`）：置いたのが page.html か部品の part.html かは印から分からない（分けるには部品の出力にも印が要る）ので、どちらでもデータファイルの `template:` に置き、文がどちらかと言う（`LIMIT:`）
- **テンプレートの誤りの場所**：text/template の文の頭を読むのは `textpos.OfTemplate` だけで、読んだ場所は `Line`・`Column` に入れ、文に残さない。html/template のエスケープの誤りは Node の位置を先に使い、無ければ `Error.Line` を `textpos.TemplateLine` で写す
- **CLI の誤り**：コマンド行の誤りは、cobra の文に `cli_usage` で「run floe --help」を添える（cobra の文は直し方を言わない）
- **読めない・書けない誤り**：floe がユーザーのために使うパス（コマンド行で渡したパス、設定のディレクトリ、既定のディレクトリ）の下のファイルを読めなければ `usererr.CannotRead`（システムの文のまま、そのパスに置く。`gone.yaml: no such file or directory`）、書けなければ `usererr.CannotWrite`（`cannot_write`：書けないことと確かめる所を言い、括弧の中にシステムの文を残して、そのパスに置く。ユーザーが決めた。書けない理由はディレクトリ・権限・名前の長さなどで、システムの文だけでは何を直すかが分からない）で言う。ふつうのファイルでもディレクトリでもないもの（FIFO・デバイスなど）は、`CannotRead` が `usererr.ErrNotFile` を見て `kit_not_file` で言う。呼び出し元は言い換えず、`CannotRead` に渡す
- **警告の出し方**：警告も `*usererr.Error` で作り、誤りと同じ関数で場所を付ける。`error` にも、`error` である `usererr.List` にも入れず、`[]*usererr.Error` の別の返り値で運ぶ（`tmpl.Draw` から `store.Entry`・`server.Saved` まで）。言葉にするのは `usererr.Warn`（並べて 1 回だけ言う）、並べるのは `Report.Render`（誤りの後に、頭に `warning: ` を付けて。floe の失敗のときは失敗だけ）。描けなかった入力の警告は無く、ほかの入力の誤りがあっても、描けた入力の警告は誤りと一緒に出す。描いた後に断るとき（publish の 409 など）は、`usererr.WithWarnings` で警告を断りに添え、`Tell` が誤りの後に言う（管理ページへの断りには入れない）。警告を添えた断りを `List.Add`・`Collect` に渡すと、警告を黙って落とさず、floe の失敗になる。CLI は警告を `Run` の出口で 1 回出す（手元の警告は `Warn` を 1 回だけ呼ぶ）。rebuild とページを開くときの描きは、警告を捨てる
- **出す**：誤りを言葉にするのは `usererr.Tell`、並べるのは `Report.Render` だけ
- **目録の鍵は英語と日本語の両方**：目録の鍵は、それを使うコードと同じ PR で、英語と日本語の両方に入れる

## floe の不具合や困ったこと

floe を使っていて、または直していて困ったこと（誤りの文が直し方を言わない、検査の漏れ、書きづらいなど）に気付いたら、その時点で報告し（回避したときも）、聞かずにページ「floe の不具合と困ったこと」（slug `floe-sc-usage-issues`、データは `~/.local/share/floe-plans/floe-sc-usage-issues.yaml`）に S 番号で載せる。書き方はそのページの注に従い、floe 本体を直すかはユーザーが決める（今の PR に混ぜない）。
