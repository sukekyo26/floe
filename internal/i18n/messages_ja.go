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

	"doc_no_lang":  "最上位に lang: ja のようにページの言語を指定してください（en・ja・pt-BR のような BCP 47 の言語タグ）",
	"doc_not_text": "%s は文字列で指定してください",
}
