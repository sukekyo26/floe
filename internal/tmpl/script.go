package tmpl

import (
	"encoding/json"
	"html/template"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// checkScripts draws the scripts of d, its items its schema s declares of the part script, into
// d.parts, by the path of keys to each, and reports each mistake in their shape at its line and
// column under root. What a function does is the browser's to tell.
func checkScripts(s *shape, d *Data, root ast.Node) error {
	found := partsNamed(s, d, "script")
	if found == nil {
		return nil
	}
	var problems yamlpos.Problems
	for _, f := range found {
		js, data, mistakes := readScript(d.File, root, f)
		if mistakes != nil {
			problems = append(problems, mistakes...)
			continue
		}
		d.drew(f.at, drawScript(js, data))
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// readScript reads the script f of the data file file under root: its js and data, or the mistakes in
// its shape.
func readScript(file string, root ast.Node, f itemAt) (js string, data any, mistakes yamlpos.Problems) {
	m := map[string]any{}
	if keys, ok := f.value.(map[string]any); ok {
		m = keys
	}
	at := func(k string) []string { return append(f.at[:len(f.at):len(f.at)], k) }
	for k := range m {
		if k != "js" && k != "data" {
			mistakes = append(mistakes, itemMistake(file, root, at(k), true, usererr.New("tmpl_script_key", k)))
		}
	}
	js, isText := m["js"].(string)
	_, given := m["js"]
	blank, end := !isText || strings.TrimSpace(js) == "", scriptEnd.FindString(js)
	switch {
	case blank && given:
		mistakes = append(mistakes, itemMistake(file, root, at("js"), false, usererr.New("tmpl_script_js")))
	case blank:
		mistakes = append(mistakes, itemMistake(file, root, f.at, false, usererr.New("tmpl_script_js")))
	case end != "":
		e := usererr.New("tmpl_script_end", end, end[1:])
		mistakes = append(mistakes, itemMistake(file, root, at("js"), false, e))
	}
	return js, m["data"], mistakes
}

// scriptEnd finds what of a script an HTML parser would read as the end of the script around it:
// </script, or <!--, past which a <script in a string moves the end elsewhere.
// LIMIT: every <!-- is refused, not only one a <script follows; tell them apart if a script needs one.
var scriptEnd = regexp.MustCompile(`(?i)</script|<!--`)

// drawScript draws the element a script draws in, and the call of its function js with that element
// and data. js is assigned, so that a ; or a comment after the function still parses, and the line
// break ends a // comment before the call. data goes in as the text of its JSON, whose < and > are
// escaped, so that no text of it ends the script, and is parsed, so that a key __proto__ is a key.
func drawScript(js string, data any) template.HTML {
	// Values read from YAML, without .inf or .nan or keys other than text, always encode.
	b, _ := json.Marshal(data)         //nolint:errcheck,errchkjson // see above
	text, _ := json.Marshal(string(b)) //nolint:errcheck,errchkjson // text always encodes
	html := "<div></div><script>{const floeScript = " + js + "\n" +
		";floeScript(document.currentScript.previousElementSibling, JSON.parse(" + string(text) + "));}</script>"
	return template.HTML(html) //nolint:gosec // G203: the data file's own script, its end checked by scriptEnd
}
