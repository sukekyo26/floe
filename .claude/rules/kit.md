---
paths:
  - "internal/{schema,kit,draw,tmpl,doc,rules,page}/**"
---

# 部品・テンプレート・kit・描く所

- **schema の形はコンパイルした木から**：floe が要る形（どの項目がどの部品・format か）は、コンパイルした `*jsonschema.Schema` から取り、schema の JSON を自前で辿らない。生の JSON を見るのは、`$schema`（2020-12・2019-09 だけを受ける）と `$id` の確かめと、`read` の字句の確かめだけで、schema の鍵の意味は読まない。例外：「埋めて呼び直す」ときは、写しに名前を足すために、生の JSON の根と `$defs` の直下を読んで書いてよい（書くのは写しだけ。`holes.fill`）
- **木は Follow で歩く**：schema の木は、全部の欄を辿る `every` か、`schema.Follow` の繰り返しで歩き、検査ごとに歩き方を書き分けない。例外は、`$schema` と `$id` を見る生の JSON の歩きと、`sharedKeys`
- **項目の検査は checkNode 1 つ**：schema の 1 つずつの検査は `checkNode` 1 つで、テンプレートも部品も共有の schema も `checkOwn` の同じ歩きで走らせる。共有の schema は、指す側の歩きでなく、その根から確かめる（指す側は根しか指せない）。指す側の `$ref` が辿られない所（辿らない場所にある、横の format で止まる）は、指す側で言う。テンプレートに足す検査は `checkNode` に足し、部品に写さない
- **値を歩くのは Walk だけ**：データの値を schema に沿って歩くのは `schema.(*Compiled).Walk` だけで、値ごとに `Follow` を 1 回呼び、部品の値はその部品の schema の根から歩き続ける。描く値も、確かめる項目（部品の check を走らせる所）も、Walk から取る
- **part.html は元の値を `written` で読む**：part.html に届く `.` は描いた後の値。描く前の字と比べる部品（table の列の `pills`）は関数 `written` で読み、中の部品の形を自分の schema に写さない。`written` の字は比べるか字のまま描き、Markdown や HTML として描かない
- **部品もテンプレートと同じ format を取る**：部品の schema も floe の format（html・markdown・image）と、それを持つ共有の schema を、floe が辿る所で取る。部品だけの format の決まりは作らない。部品の html の項目も、描いたページの確かめを通る
- **部品は見出しを描かない**：部品は自分の段を知らないので、part.html に `<h1>`〜`<h6>` を書かず、題は `class="title"` の要素で書く。見出しを描くのは page.html と書き手の Markdown だけ。`TestEveryPartDrawsEveryPart`
- **描いたページは x/net/html で読んで書く**：描いたページの確かめ（「データの HTML 項目は描いたページの木で確かめる」が断るもの、部品の名前）は x/net/html の木で行い、スタイルを head に入れるのも、見出しに id を付けるのも、その木を直して `html.Render` で書く。正規表現は、CSS の `</style` の書き換えだけに使う
- **データの HTML 項目は描いたページの木で確かめる**：format html の項目は、描くとき、探りの空白と偽れない印のコメントの対で挟む。断る判断は描いたページの木（`html.Parse` 1 回）だけで決め、項目の文だけを読んで判断しない（svg・math の中でブラウザと読みが違う）。見た後に印と探りを外して書き出し、確かめた木と書き出す木を同じにする。断ると決めた後で原因を名指すためだけに `html.ParseFragment` で読み直してよいが（`named`・`placed`）、結果を断る判断に使わない。`<style>`・`<base>`・`<noscript>`、charset と viewport のほかの `<meta>`、`<select>` の中で中身を字として読む要素と svg・math、属性の `ZgotmplZ` は、描いたページ全体で断る（page.html・part.html・項目のどれが書いても）
- **段 1 は読むと確かめるに分ける**：kit の `read` は値を作るのに要ることだけをし、値の後の決まりは `check` が持つ。kit の外で Kit を作れるのは `Load` と `Floe()` だけ。実行のときは floe のものに `check` を走らせず、テストで走らせる。例外：後で一緒にコンパイルするものの読み方を変える入力は `read` で断る（今は部品と共有の schema の `$schema` と `$id`）
- **読んだ Kit は変えない**：`Kit` は欄を外へ出さず、`[]byte` も写しを返す。自分のものを重ねるときは、土台を書き換えずに新しい `Kit` を作る（`Floe()` の値をプロセス全体で共有するため）。描くときは、`Template.Page`・`Part.Page` が返す写しを Execute する（Execute は html/template にエスケープの結果を書き込み、その後は写しを作れない）
- **show が出すのは読んだファイル**：`floe part show` などが出すファイルは、読み手が読んだもの（`readFile` が書き留めたもの）だけで、順は種類ごとの並び 1 つ（`unreadIn` も同じ並びを使う）。show のために別に選ばない
- **stack は書いた部品を全部描く**：stack の part.html は部品の種類を書き並べず、report の page.html と同じ繰り返しで描く。置ける部品は stack の schema 1 か所で決める。`TestEveryPartDrawsEveryPart`
- **部品 text はコードの部品と並ぶ Markdown**：部品の並びの Markdown は部品 text で、Go の名前は `kit.TextPart` 1 つ。テンプレートや部品の自分の Markdown の欄は部品 text にしない。text は部品の箱でない外枠 `data-floe-text` に描き、外の面と tone を継ぐ。page.css とテーマの base.css は外枠で止まる
- **組み込みのテンプレートは汎用**：floe の組み込みのテンプレートは、どのプロジェクトでも使う一般的な文書の形にする。floe の開発の流れだけの段は決まった段にせず、任意の節に書かせる。見本（example.yaml）の中身も一般的な題材にする。必須は、その文書が成り立たない段だけにする
- **組み込みは schema だけで断る**：組み込みのテンプレートは checks.yaml を持たず、断るのは schema（必須・型・知らない鍵・空の字）だけ。見れば分かる誤り（判断に推しが 2 つ、など）は `LIMIT:` にする
- **目次の id はテンプレートが付ける**：テンプレートが自分の段へリンクするときは、page.html が言語によらない決まった id を見出しに付ける。floe が見出しの字から作る id に頼らない
