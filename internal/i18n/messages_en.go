package i18n

func init() {
	register(English, messagesEN)
}

// messagesEN is the reference catalog: every other language has exactly its keys.
var messagesEN = map[string]string{
	// LIMIT: a library's own words, English in every language; floe words only what it checks itself
	"library": "%s",

	// yaml: reading a YAML file
	"yaml_documents":            "a YAML file holds one document: write it as one, without --- between parts",
	"yaml_no_anchors":           "anchors, aliases, merge keys (<<) and tags are not supported; write the value out",
	"yaml_comment_cut":          "a # after a space began a YAML comment, which may have cut the value short, to %s: put the value in quotes if the # is part of it, or write the comment on a line of its own",
	"yaml_flow_key_alone":       "%q is a key without a value in { }: write its value after a \":\", or, if a , in text ended the text before it, put that text in quotes",
	"yaml_key_not_text":         "a key is a name: write %s as text, in quotes",
	"yaml_not_finite":           "%s is not a value a YAML file can hold; write a number in digits, or quote it as text",
	"yaml_number_not_number":    "%[1]s reads as the number %[2]s here, but as text in YAML 1.2: write %[2]s for the number, or put %[1]s in quotes for text",
	"yaml_number_not_text":      "%[1]s reads as text here, but as the number %[2]s in YAML 1.2: write %[3]s for the number, or put %[1]s in quotes for text",
	"yaml_number_octal":         "%[1]s reads as the octal number %[2]s here, but as %[3]s in YAML 1.2: write %[3]s, or %[4]s for %[2]s",
	"yaml_number_too_big":       "%s is a number in YAML 1.2, too large for floe to read: put it in quotes to keep it as text",
	"yaml_number_outside_float": "%s is a number in YAML 1.2 that a float64 cannot hold: put it in quotes to keep it as text",

	// schema: compiling a schema and the shape floe draws by
	"schema_dialect":            "$schema %s names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out",
	"schema_no_url":             "%s: a schema refers to no other file or URL; write what a $ref refers to in its own file",
	"schema_empty":              `the schema is empty: write a JSON Schema object, such as {"type": "object"}`,
	"schema_no_part":            "%s is not a part; refer to one as floe:///parts/<name>.json",
	"schema_id_floe":            "this $id names %s, a floe: URL, which is floe's own: give the schema an $id of your own, like https://example.com/item.json, or leave $id out",
	"schema_id_relative":        "this $id, %s, has no scheme, which a part or a schema the parts share may not have, as it may land under floe:///parts/ and take the place of another: write an absolute URL of your own, like https://example.com/item.json, or leave $id out",
	"schema_id_root":            "this $id, %s, is not this file's URL as it is: write %s as it is, or leave $id out",
	"schema_ref_templates":      "%s is no file floe holds: a template refers to a part as floe:///parts/<name>.json, and inside its own schema.json as #/…",
	"schema_part_unreached":     "the part %s draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn",
	"schema_ref_unreached":      "%s holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows",
	"schema_format_unreached":   "the format %s marks an item to draw; declare it under properties or items, which floe follows",
	"schema_part_into":          "%s points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, %s",
	"schema_dynamic_ref":        "floe does not follow %s: write a $ref",
	"schema_root_drawn":         "the root of the schema is drawn whole by %s; declare the items of the page under properties, and name a part under one of them",
	"schema_shared_declared":    "%s is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml",
	"schema_shared_key":         "the schema requires %s, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items",
	"schema_part_shared_format": "the part refers to %s, a schema of floe's that holds items of the format %s, which a part's part.html cannot draw: refer to it from the template's schema instead, and give the part items of its own",
	"schema_part_format":        `a part's schema takes no "format": "%s": its part.html draws its items, which the checks of a format do not reach; declare the item in the template's schema instead, or, for Markdown, declare "type": "string" alone and draw it with {{markdown .<name>}}`,

	// doc: the keys every data file shares
	"doc_no_lang":  "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)",
	"doc_not_text": "%s must be text",

	// rules: checks.yaml and computed.yaml
	// LIMIT: the last %s of rules_check_error and rules_computed_error is expr's English
	"rules_checks_yaml":               "write checks.yaml as a list of checks, each with expr and message (and for and field if you like)",
	"rules_checks_key":                "%s is not a key of a check; use for, field, expr and message",
	"rules_checks_value":              "%s takes one value, written as text: not a list or keys and values",
	"rules_checks_missing":            "write %s in the check",
	"rules_checks_for_empty":          "for is empty: write the path of the list to check, or leave for out to check the whole",
	"rules_checks_field_empty":        "field is empty: write the path of the field a failure points at, or leave field out to point at the item",
	"rules_checks_step":               "%s: %s has an empty step: write a key between each two dots, and no dot at its start or end, like sections.parts",
	"rules_checks_not_list":           "for: %s does not lead to a list in %s",
	"rules_check":                     `%s ("%s")`,
	"rules_item":                      "%s",
	"rules_check_error":               "%s (the check could not run: %s)",
	"rules_check_not_finite":          "%s (the check could not run: %s is not a finite number)",
	"rules_computed_yaml":             "write computed.yaml as names and expressions, like total: sum(items, .price)",
	"rules_computed_name":             "%s is not a name an expression and page.html can use: write one name both read as a name, not starting with _, and not a function of expr",
	"rules_computed_clash":            "%s is an item of the data too, so an expression could not tell them apart; rename it",
	"rules_computed_env":              "$env is not for computed.yaml; use the names",
	"rules_computed_cycle":            "the values are computed from each other: %s",
	"rules_computed_error":            "%s cannot be computed from %s: %s",
	"rules_computed_not_finite":       "%s cannot be computed from %s: %s is not a finite number",
	"rules_computed_value_not_finite": "%s cannot be computed from %s: the value %s is not a finite number",

	// kit: the parts, looks, themes and templates read at stage 1
	"kit_name_twice":             "%s names both %s and %s: give one of them another name",
	"kit_look_over_part":         "%s is the name of the part %s, which its parts and schemas name as a part: give this look another name",
	"kit_part_over_look":         "%s is the name of the look %s, which its parts take with looks:: give this part another name",
	"kit_not_number":             "%s takes a number, not %s",
	"kit_num_digits":             "write num, or num and a count of significant digits from 1 before the number, like num 3",
	"kit_part_yaml":              "write part.yaml as keys and values: looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
	"kit_part_yaml_key":          "%s is not a key of part.yaml; write looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
	"kit_part_yaml_looks":        "write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
	"kit_part_yaml_description":  "a part's description is its schema.json's description; part.yaml's description: is a look alone's, which has no schema",
	"kit_part_yaml_about":        "write description: as text, what the look is",
	"kit_words_yaml":             "write words.yaml as language tags, each the names of the words and their text",
	"kit_words_lang_not_mapping": "%s: write the names of the words and their text",
	"kit_words_not_text":         "%s.%s: write text, in quotes where YAML would read a number, true, false or null",
	"kit_words_lang_twice":       "%s is the language %s again: a tag's case does not tell languages apart",
	"kit_words_unknown":          `words.yaml has no word "%s" in %s`,
	"kit_part_files":             "a part holds schema.json, part.html and part.css, and a look alone part.css without the other two (and part.yaml if it takes looks); add what it lacks, or remove what a look has no use for",
	"kit_part_unread":            "floe reads no file of this name in a part: a part holds schema.json, part.html and part.css, and part.yaml, checks.yaml and words.yaml if it likes; remove it",
	"kit_look_unread":            "floe reads no file but part.css and part.yaml in a look, a directory without a schema.json: remove it, or add the schema.json to make it a part",
	"kit_theme_unread":           "floe reads no file of this name in a theme: a theme holds base.css, palette.css and a <part>.css for each part or look it styles; rename it after one, or remove it",
	"kit_look_unknown":           "looks names %q, which is no part or look: name one floe or your own parts hold",
	"kit_look_cycle_part":        "the part %s takes its own look (%s), which floe would follow forever; fix its part.yaml",
	"kit_look_cycle_look":        "the look %s takes itself (%s), which floe would follow forever; fix its part.yaml",
	"kit_template_file":          "%s is missing: a template has page.html and schema.json",
	"kit_template_unread":        "floe reads no file of this name in a template: a template holds page.html and schema.json, and page.css, computed.yaml, checks.yaml, words.yaml and example.yaml if it likes; rename it after one, or remove it",

	// css: the CSS floe puts in a page
	"css_brace":    "this } closes no block: floe puts this CSS in a layer, which it would close, leaving the rules after it out of the layer and over the parts' styles",
	"css_import":   "floe puts this CSS in a layer, where a browser reads no @import: load the stylesheet with a <link> in page.html",
	"css_escape":   "an escape (\\) outside a string, a url() or a comment, which floe does not read: write the characters themselves",
	"css_misread":  "a browser reads this otherwise than floe: end each string on its own line, and in url() write the address alone or one quoted string",
	"css_unclosed": "this is still open at the end of the file: floe puts this CSS in a layer, whose } it would take with the rules after it; close it",

	// tmpl: the template and the theme a data file names
	"tmpl_no_template":   "name the template at the top level, like template: report (templates: %s)",
	"tmpl_unknown":       `unknown template "%s"; use %s`,
	"tmpl_unknown_theme": `there is no theme "%s"; use %s`,

	// draw: the page drawn
	"draw_style":        "the page drawn from this file has a <style>, which page.html or a part writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css",
	"draw_base":         "the page drawn from this file has a <base>, which page.html or a part writes and the page's CSP ignores (base-uri 'none'): write its links whole",
	"draw_part_unknown": "the page drawn from this file names the part %s, which there is none of; use %s",
	"draw_part_several": `the page has data-floe-part="%s", which is not the name of one part: write data-floe-part="<name>", as written, and take the look of others with data-floe-look="<name> …"`,
	"draw_look_box":     `the page has data-floe-part="%[1]s", but %[1]s is a look, which has no box: take it with data-floe-look="%[1]s"`,
	"draw_format":       "floe does not draw an item of format %s yet: give it another format, or none",

	// cli: the command line (English alone is told; the other languages hold the keys as every catalog does)
	// LIMIT: the %s of cli_usage is cobra's English, as cobra words what it cannot read
	"cli_usage":            "%s: run floe --help",
	"cli_path_empty":       "%s is an empty path; give %s, and check that a variable written for it is set",
	"cli_relative_no_cwd":  "the working directory cannot be read, so the relative path %s means nothing; run floe from a directory that exists, or give an absolute path",
	"cli_wants_data":       "a data file (.yaml or .yml)",
	"cli_wants_out_dir":    "the directory to write the pages in",
	"cli_check_not_data":   "%s is not a data file (.yaml or .yml)",
	"cli_data_no_name":     "%s has no name before its extension, which names its page: name it, like plan.yaml",
	"cli_render_same_file": "%s and %s would both write %s; rename one of them",
	"cli_render_missing":   "%s is missing: write it from %s with floe render",
	"cli_render_stale":     "%s is not what %s draws: write it again with floe render",
	// LIMIT: the %s of cli_cannot_write is the system's English, as the system words why it cannot write
	"cli_cannot_write": "cannot be written (%s): check that its directory exists and can be written to, and that its name is free and not too long",
}
