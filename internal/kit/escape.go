package kit

import (
	"bytes"
	"crypto/rand"
	"errors"
	"html/template"
	"io"
	"strconv"
	"unicode/utf8"

	"github.com/sukekyo26/floe/internal/usererr"
)

// stopFunc names the function the copy of a page calls first, before the page, which it stops.
const stopFunc = "floeEscapeStop"

// errStop ends the drawing that follows the escaping, before anything of the page runs.
var errStop = errors.New("stop")

// escapeMistake runs html/template's escaping of page, a page.html or part.html parsed from src and never
// drawn, shown as file, and returns what html/template refuses there, at the line and column of the node it
// names when it names one. Escaping takes no data: html/template does it on the first drawing, whatever the
// data, so a mistake of it is the file's, and every *template.Error is one, made by the escaping alone. It
// is run on a copy, as a page drawn once cannot be cloned for each language, from a template of the copy's
// own that calls the page after a function that stops it: html/template escapes the page as it is called,
// in the text context a page is drawn in, and then nothing of it runs, neither its functions nor its loops.
// LIMIT: html/template's English (ends in a non-text context: {stateCSS …}); word each ErrorCode if needed.
func escapeMistake(page *template.Template, src []byte, file string) error {
	c, err := page.Clone()
	if err != nil {
		return err //nolint:wrapcheck // a parsed page that was never drawn clones
	}
	stop := template.FuncMap{stopFunc: func() (string, error) { return "", errStop }}
	call := "{{" + stopFunc + "}}{{template " + strconv.Quote(page.Name()) + " .}}"
	// A name of its own each time: a page may define, or call, any name, which this one would take the place of.
	run, err := c.New("floe-escape-" + rand.Text()).Funcs(stop).Parse(call)
	if err != nil {
		return err //nolint:wrapcheck // floe's own text, which parses
	}
	err = run.Execute(io.Discard, nil)
	var escape *template.Error
	if !errors.As(err, &escape) {
		return nil
	}
	line, column := escape.Line, 0
	if escape.Node != nil {
		// Where the node html/template names is in src, the text of every template of the file, {{define}}d ones
		// too: the field of {{.x}}, the condition of {{if}}, the name of {{template}}, the start of a template's
		// body (its root, as ErrOutputContext names).
		if at := int(escape.Node.Position()); at >= 0 && at <= len(src) {
			start := bytes.LastIndexByte(src[:at], '\n') + 1
			line, column = 1+bytes.Count(src[:at], []byte("\n")), utf8.RuneCount(src[start:at])+1
		}
	}
	return usererr.LibraryAt(file, line, column, escape.Description)
}
