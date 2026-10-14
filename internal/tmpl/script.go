package tmpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// checkScripts draws the scripts of d, its items its schema s declares of format script, into
// d.scripts, by the path of keys to each, and reports each mistake in their shape at its line and
// column under root. What a function does is the browser's to tell.
func checkScripts(s *schema, d *Data, root ast.Node) error {
	var found []itemAt
	s.find("script", d.Values, nil, &found)
	if found == nil {
		return nil
	}
	d.scripts = make(map[string]template.HTML, len(found))
	var problems yamlpos.Problems
	report := func(at []string, onKey bool, e *usererr.Error) {
		key, value, path := yamlpos.Find(root, at)
		pos := yamlpos.OfValue(key, value)
		if onKey {
			pos = yamlpos.Of(key)
		}
		problems = append(problems, &yamlpos.Error{File: d.File, Pos: pos, Err: usererr.New("tmpl_item", path, e)})
	}
	for _, f := range found {
		mistakes, js, data := readScript(f.value)
		for _, m := range mistakes {
			at := f.at
			if m.key != "" {
				at = append(at[:len(at):len(at)], m.key)
			}
			report(at, m.onKey, m.err)
		}
		if mistakes != nil {
			continue
		}
		html, err := drawScript(js, data)
		if err != nil {
			return err
		}
		d.scripts[pathKey(f.at)] = html
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// scriptMistake is a mistake in the shape of a script, at its key key ("" for the script itself): at
// the key itself with onKey, else at its value.
type scriptMistake struct {
	key   string
	onKey bool
	err   *usererr.Error
}

// readScript reads the script v: its js and data, or the mistakes in its shape.
func readScript(v any) (mistakes []scriptMistake, js string, data any) {
	m := map[string]any{}
	if keys, ok := v.(map[string]any); ok {
		m = keys
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if k != "js" && k != "data" {
			mistakes = append(mistakes, scriptMistake{key: k, onKey: true, err: usererr.New("tmpl_script_key", k)})
		}
	}
	js, isText := m["js"].(string)
	_, given := m["js"]
	switch end := scriptEnd(js); {
	case !isText || strings.TrimSpace(js) == "":
		key := ""
		if given {
			key = "js"
		}
		mistakes = append(mistakes, scriptMistake{key: key, onKey: false, err: usererr.New("tmpl_script_js")})
	case end != "":
		mistakes = append(mistakes, scriptMistake{key: "js", onKey: false, err: usererr.New("tmpl_script_end", end, end[1:])})
	}
	return mistakes, js, m["data"]
}

// scriptEnd returns what of js an HTML parser would read as the end of the script around it, as js
// writes it: </script, or <!--, past which a <script in a string moves the end elsewhere. "" for none.
func scriptEnd(js string) string {
	lower := []byte(js)
	for i, c := range lower {
		if 'A' <= c && c <= 'Z' {
			lower[i] = c + 'a' - 'A'
		}
	}
	for _, end := range []string{"</script", "<!--"} {
		if i := bytes.Index(lower, []byte(end)); i >= 0 {
			return js[i : i+len(end)]
		}
	}
	return ""
}

// drawScript draws the element a script draws in, and the call of its function js with that element
// and data. data goes in as JSON, whose < and > are escaped, so that no text of it ends the script.
func drawScript(js string, data any) (template.HTML, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("write the data of a script: %w", err)
	}
	// The line break ends a // comment at the end of js before the call.
	html := "<div></div><script>(" + js + "\n)(document.currentScript.previousElementSibling, " + string(b) + ");</script>"
	return template.HTML(html), nil //nolint:gosec // G203: the data file's own script, its end checked by scriptEnd
}
