package tmpl

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"unicode/utf8"

	"github.com/sukekyo26/floe/internal/usererr"
)

// errStub stops the escaping run at the first function a page calls.
var errStub = errors.New("stub")

// escapeMistake runs html/template's escaping of page, a page.html or part.html parsed from src and never
// drawn, shown as file, whose functions are those of fns, and returns what html/template refuses there, at
// the line and column of the action it names. Escaping takes no data: html/template does it on the first
// drawing, whatever the data, so a mistake of it is the file's, and every *template.Error is one, made by
// the escaping alone. It is run on a copy, as a page drawn once cannot be cloned for each language, with no
// data and every function stopping it, so that nothing is drawn nor read: the errors of drawing are not its.
// LIMIT: html/template's English (ends in a non-text context: {stateCSS …}); word each ErrorCode if needed.
func escapeMistake(page *template.Template, src []byte, file string, fns ...template.FuncMap) error {
	c, err := page.Clone()
	if err != nil {
		return err //nolint:wrapcheck // a parsed page that was never drawn clones
	}
	stubs := template.FuncMap{}
	for _, m := range fns {
		for name := range m {
			stubs[name] = func(...any) (any, error) { return nil, errStub }
		}
	}
	err = c.Funcs(stubs).Execute(io.Discard, nil)
	var escape *template.Error
	if !errors.As(err, &escape) {
		return nil
	}
	line, column := escape.Line, 0
	if escape.Node != nil {
		// Where the action starts in src, the text of every template of the file, {{define}}d ones too.
		if at := int(escape.Node.Position()); at >= 0 && at <= len(src) {
			start := bytes.LastIndexByte(src[:at], '\n') + 1
			line, column = lineAt(src, at), utf8.RuneCount(src[start:at])+1
		}
	}
	return usererr.NewAt(file, line, column, "tmpl_go_template", escape.Description)
}
