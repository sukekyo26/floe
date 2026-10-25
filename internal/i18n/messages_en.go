package i18n

func init() {
	register(English, messagesEN)
}

// messagesEN is the reference catalog: every other language has exactly its keys.
var messagesEN = map[string]string{
	// LIMIT: a library's own words, English in every language; floe words only what it checks itself
	"library": "%s",

	// yaml: reading a YAML file
	"yaml_documents":      "a YAML file holds one document: write it as one, without --- between parts",
	"yaml_no_anchors":     "anchors, aliases, merge keys (<<) and tags are not supported; write the value out",
	"yaml_comment_cut":    "a # after a space began a YAML comment, which may have cut the value short, to %s: put the value in quotes if the # is part of it, or write the comment on a line of its own",
	"yaml_flow_key_alone": "%q is a key without a value in { }: write its value after a \":\", or, if a , in text ended the text before it, put that text in quotes",
	"yaml_key_not_text":   "a key is a name: write %s as text, in quotes",
	"yaml_not_finite":     "%s is not a value a YAML file can hold; write a number in digits, or quote it as text",

	// doc: the keys every data file shares
	"doc_no_lang":  "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)",
	"doc_not_text": "%s must be text",
}
