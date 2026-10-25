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

	// schema: compiling a schema and the shape floe draws by
	"schema_dialect":          "$schema %s names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out",
	"schema_no_url":           "%s: a schema refers to no other file or URL; write what a $ref refers to in its own file",
	"schema_no_part":          "%s is not a part; refer to one as floe:///parts/<name>.json",
	"schema_part_unreached":   "the part %s draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn",
	"schema_ref_unreached":    "%s holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows",
	"schema_format_unreached": "the format %s marks an item to draw; declare it under properties or items, which floe follows",
	"schema_part_into":        "%s points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, %s",
	"schema_dynamic_ref":      "floe does not follow %s: write a $ref",
	"schema_root_drawn":       "the root of the schema is drawn whole by %s; declare the items of the page under properties, and name a part under one of them",
	"schema_shared_key":       "the schema requires %s, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items",
	"schema_part_format":      `a part's schema takes no "format": "%s": its part.html draws its items, which the checks of a format do not reach; declare the item in the template's schema instead, or, for Markdown, declare "type": "string" alone and draw it with {{markdown .<name>}}`,

	// doc: the keys every data file shares
	"doc_no_lang":  "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)",
	"doc_not_text": "%s must be text",
}
