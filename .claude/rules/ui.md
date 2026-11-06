---
paths:
  - "internal/ui/**"
  - "internal/pagediff/**"
  - "internal/server/pages.go"
  - "internal/server/api.go"
  - "internal/store/reads.go"
---

# 管理ページと枠

- **未読は reads.json 1 つ**：未読を持つのは publish で保存したページだけ。書くのは store の lock の中だけ。読んだ印を足すのは `POST /api/artifacts/{slug}/read` 1 つで、本文はページを返したときにサーバーが作った版の印、slug は枠が自分の持つ値から決める（一覧から取った印は送らない）。戻すのは `DELETE` の同じ道 1 つで、枠の画面の操作だけが呼ぶ。新しい版かを決めるのは store の `news` 1 か所で、画面は比べない。トーストは人が開く操作をしたときだけ出す。`since` は 1 度だけ書き、その後は動かさない
- **ページの中身とタブの印は 1 か所ずつ**：中身は `server` の `content`、タブの印は `version`（ファイルの印は `files.Mark`）で、印は中身の前に取る。返すページはサーバーへのつなぎを持ち続けない（Chrome の 1 つのサーバーへのつなぎは 6 本まで）。ページから枠へ知らせるときは送り先の origin を名指し、枠は自分の iframe の、origin が `null` の知らせで、道が自分の開いた URL と合うものだけを受け、slug は自分の持つ値から決める
- **一覧の列の幅は `list.columns`、つまみは `dragEdge` 1 つ**：棚・一覧・枠の左の木・差分の目次のつまみは `dragEdge` 1 つで動かし、置き場所は `edgeAt`。幅と畳んだかは見る人の都合としてブラウザ（`localStorage`）にだけ持つ
- **管理ページと枠は `ui.Page` 1 つ**：`/`・`/tasks`・`/a/{slug}/` は同じ管理ページ。ページそのものは `?raw` で、枠の iframe が読む。管理ページと枠には `uiCSP` を付け、枠の中のリンクは `leaveFrame` 1 つが扱う
- **開いている版は 1 か所**：右のページは `picked`、全面は `frameAt` の `v` で持ち、ページの URL を作るのは `pageHref`・`rawHref`・`diffHref`・`downloadHref` だけ。全面の `?v=` はサーバーの `frame` が読み、JS は読み方を持たない。版を選んで見ている間は最新を既読にしない
- **帯の操作は 1 つの決まり**：帯に見えている操作は「…」に出さず、帯から外れた操作は必ず「…」に出す。「…」はいつも全部の項目を作り、見えるかは CSS で決める。幅で変えるのは 40rem の 1 か所だけ。行は ★ とアーカイブを出し、それ以外を「…」に出す。`TestUIBarActions` が確かめる
- **管理ページのキーは `KEYS` 1 つ**：キーの表を 1 つ持ち、`pressKey` が引く。ページの中から渡すのは、移ることとモーダルを開くことだけのキー（`PAGE_KEYS`）で、枠は自分の iframe の、origin が `null` の知らせだけを、iframe にフォーカスがあるときだけ受ける
- **管理ページの色は明るい 1 組**：色はすべて `ui.html` の `:root` の変数から取る。例外はプロジェクトの pill の `.proj-tint` 1 つで、行ごとの色 `--c` は `:root` で決まらないので、`:root` の外で `:root` の変数と混ぜる（`TestManageSheet` が名指しで許す）。暗い画面・その切り替え・`prefers-color-scheme` と `color-scheme` の決まりは作らない（ユーザーが決めた。複雑になるだけ）。`color-scheme` を付けないので、枠の中のページは OS の設定どおりに `prefers-color-scheme` を読む
- **管理ページの角と字は `:root` の目盛り**：ui.html の角は `--corner`・`--corner-menu`・`--corner-small` か形（`0`・`50%`・`999px`）で、字の大きさは `--text-label`・`--text-small`・`--text` で書く。`:root` の外に色を書かない（例外は上の `.proj-tint`）。変数の名前はパレットの変数と分け、default から写さない。`TestManageSheet`
- **版の差分はページの字だけを持ち出す**：版を比べるのは `pagediff.Compare` 1 つで、答えは字だけ。管理ページは `textContent` で描き、ページの HTML を管理ページへ入れない。go-diff を import できるのは `internal/pagediff` だけ（depguard の規則 `go-diff-in-pagediff`）
- **一覧の並びは `arrange` 1 つ、色の順は `projectOrder` 1 つ**：一覧も右のページの `j` `k` も `arrange` の順。全面の `j` `k` は `keepOrder` が覚えた順。色の順は全部の項目を数えた `projectOrder` だけが決め、絞り込みで変わらない。タスク一覧の並びは `renderTaskList` 1 つで、状態の表の `oldest` で決める
- **行は drawRows 1 つで描く**：どの画面の行も `drawRows` で描く。`moveRow` は描いた行を読み、画面ごとに分岐を足さない。タスク一覧の問い合わせは `TASKS_KEY`、`LIST_KEY` はページ一覧だけ。`keepOrder` も描いた行を読む。全面の順と戻り先は `keepOrder` の `{order, shown, back}` 1 つで持つ
- **親のページの子の並びは `#list` の外**：`drawRows` の決まりに入らない
- **タスク一覧は 1 つのプロジェクトだけ**：`/tasks` は `project` を持ち、無ければ `openingProject` 1 か所が選んで URL に書く。あるプロジェクトは `taskProjects` 1 か所が決め、`projectOrder` は色と順にだけ使う。読めない間は選び直さず、覚えも書かない
- **一覧は押している間に作り直さない**：問い合わせと読んだ印の `load` は、`#list` を押してから 2 秒の間（離すまで）は行を作り直さず、次の問い合わせで描く。押下と離す間に行を作り直すと、Chrome は `click` も `dblclick` も出さない。離したことが伝わらない押下があるので、2 秒で止める。操作が呼ぶ `load` は止めない
