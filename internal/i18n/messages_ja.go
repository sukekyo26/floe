package i18n

func init() {
	register("ja", messagesJA)
}

var messagesJA = map[string]string{
	"library": "%s",

	"yaml_documents":      "YAML のファイルには文書を 1 つだけ書きます。--- で区切らず、1 つの文書で書いてください",
	"yaml_no_anchors":     "アンカー・エイリアス・マージキー（<<）・タグは使えません。値をそのまま書いてください",
	"yaml_comment_cut":    "空白の後の # が YAML のコメントを始め、値が %s で切れたかもしれません。# が値の一部なら値を引用符で囲み、コメントなら次の行に書いてください",
	"yaml_flow_key_alone": "%q は { } の中の値の無いキーです。値を「:」の後ろに書くか、前の文が文の中の , で切れたのなら、その文を引用符で囲んでください",
	"yaml_key_not_text":   "キーは名前です。%s は引用符で囲んで文字列にしてください",
	"yaml_not_finite":     "%s は使えません。数は数字で書き、文字列なら引用符で囲んでください",

	"schema_dialect":            "$schema の %s は floe が受ける版ではありません。https://json-schema.org/draft/2020-12/schema か https://json-schema.org/draft/2019-09/schema を書くか、$schema を消してください",
	"schema_no_url":             "%s: schema はほかのファイルや URL を参照できません。$ref が指すものは、その schema のファイルの中に書いてください",
	"schema_empty":              `schema が空です。{"type": "object"} のような JSON Schema のオブジェクトを書いてください`,
	"schema_no_part":            "%s は部品ではありません。部品は floe:///parts/<名前>.json の形で参照してください",
	"schema_part_unreached":     "部品 %s はここでは項目を描きません。floe は根から properties と items で届く所と、横に描くものの無い $ref の先を描きます。ここの項目は検査されても描かれません",
	"schema_ref_unreached":      "%s は floe が描く部品か format の項目を持ちます。floe が辿る properties か items の下で参照してください",
	"schema_format_unreached":   "format %s は描く項目の印です。floe が辿る properties か items の下で宣言してください",
	"schema_part_into":          "%s は floe の部品か部品が共有する schema の中を指していて、検査はされても書いたとおりには描かれません。全体の %s を指してください",
	"schema_dynamic_ref":        "floe は %s をたどりません。$ref を書いてください",
	"schema_root_drawn":         "schema の根の全体を %s が描くことになります。ページの項目は properties の下に書き、部品はそのどれかの下で指してください",
	"schema_shared_declared":    "%s は floe が自分で読む鍵です。schema の properties から消し、決まりを足すなら checks.yaml の check で書いてください",
	"schema_shared_key":         "schema が %s を求めています。%[1]s は floe がどのデータファイルからも自分で読み、schema の検査から外す（lang・template・theme・title）ので、ほかの項目を求めてください",
	"schema_part_shared_format": "部品が %s を指しています。これは format %s の項目を持つ floe の schema で、部品の part.html は描けません。テンプレートの schema から指し、部品には自分の項目を持たせてください",
	"schema_part_format":        "部品の schema には \"format\": \"%s\" を書けません。部品の項目は part.html が描き、format の検査は届きません。その項目はテンプレートの schema に書くか、Markdown なら \"type\": \"string\" だけにして {{markdown .<名前>}} で描いてください",

	"doc_no_lang":  "最上位に lang: ja のようにページの言語を指定してください（en・ja・pt-BR のような BCP 47 の言語タグ）",
	"doc_not_text": "%s は文字列で指定してください",

	// rules: checks.yaml と computed.yaml
	"rules_checks_yaml":               "checks.yaml は検査のリストとして書き、各検査に expr・message（任意で for・field）を書いてください",
	"rules_checks_key":                "%s は検査のキーではありません。for・field・expr・message を使ってください",
	"rules_checks_value":              "%s には文字として 1 つの値を書いてください。リストやキーと値は書けません",
	"rules_checks_missing":            "検査に %s を書いてください",
	"rules_checks_for_empty":          "for が空です。検査するリストのパスを書くか、全体を検査するなら for を省いてください",
	"rules_checks_field_empty":        "field が空です。落ちたときに指す項目のパスを書くか、要素そのものを指すなら field を省いてください",
	"rules_checks_step":               "%s: %s に空の段があります。点と点の間に鍵を書き、頭と終わりには点を書かないでください（sections.parts のように）",
	"rules_checks_not_list":           "for: %s は %s の中でリストに着きません",
	"rules_check":                     "%s（「%s」）",
	"rules_item":                      "%s",
	"rules_check_error":               "%s（検査を実行できませんでした: %s）",
	"rules_check_not_finite":          "%s（検査を実行できませんでした: %s は有限の数になりません）",
	"rules_computed_yaml":             "computed.yaml は total: sum(items, .price) のように、名前と式で書いてください",
	"rules_computed_name":             "%s は式や page.html で使える名前ではありません。文字で始めて文字・数字・_ を続け、expr の語や関数と重ねないでください",
	"rules_computed_clash":            "%s はデータの項目にもあり、式から区別できません。名前を変えてください",
	"rules_computed_env":              "computed.yaml では $env を使えません。名前で参照してください",
	"rules_computed_cycle":            "値が互いを使って計算されています: %s",
	"rules_computed_error":            "%s を %s から計算できません: %s",
	"rules_computed_not_finite":       "%s を %s から計算できません: %s は有限の数になりません",
	"rules_computed_value_not_finite": "%s を %s から計算できません: 値 %s は有限の数ではありません",
}
