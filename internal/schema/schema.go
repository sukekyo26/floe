// Package schema compiles a template's schema.json with the schemas floe holds (its parts and the schemas
// they share), checks a data file's values against it, and tells floe how it draws them: which item a part
// or one of floe's formats draws. The library's mistakes are told in its words; floe's own checks are those
// of what it draws by, and of the files a schema may read.
package schema

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Kind is what a schema floe holds by URL is to the template: a part, drawn by its part.html, or a schema
// the parts share (any.json, tone.json), which draws nothing itself.
type Kind int

// The kinds of a Resource.
const (
	Part Kind = iota
	Shared
)

// Resource is a schema floe holds, by its URL (floe:///parts/<name>.json): its JSON, its kind, and the file
// its mistakes are told in.
type Resource struct {
	Doc  any
	Kind Kind
	File string
}

// Formats are floe's formats: an item of one is drawn by floe, not by a part.
var Formats = []string{"html", "image", "markdown"}

// Compiled is a schema compiled with the resources.
type Compiled struct {
	Root *jsonschema.Schema
	url  string // the URL it was compiled under, the start of its walks
	file string
	res  map[string]Resource
}

// english words the library's mistakes, in English in every language (the catalog's library key).
var english = message.NewPrinter(language.English)

// Compile reads src (schema.json of file) as JSON and compiles it under url with res: the library's mistakes
// in its words at the file and JSON pointer they are in; a loader refusing every URL res does not hold; then
// floe's checks, all together.
// LIMIT: the library stops at the first $ref it cannot resolve, the first in the order of a map: with two
// or more, which one is told changes from run to run, and fixing it tells the next. The library's make; a
// golden file holds no input with two.
func Compile(url, file string, src []byte, res map[string]Resource) (*Compiled, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil {
		return nil, usererr.LibraryAt(file, 0, 0, err.Error())
	}
	c, err := build(url, file, doc, res)
	if err != nil {
		return nil, err
	}
	return c, c.checkTemplate()
}

// CheckPart compiles the part at url of res as a root, and refuses one of Formats in it or in a Shared
// schema it refers to: a trust boundary, as the checks of the data's HTML reach the template's items alone.
func CheckPart(url string, res map[string]Resource) error {
	r, ok := res[url]
	if !ok {
		return fmt.Errorf("schema: no resource %s", url) //nolint:err113 // floe's own mistake
	}
	c, err := build(url, r.File, r.Doc, res)
	if err != nil {
		return err
	}
	return c.checkPart()
}

// build compiles doc under url, after checking the drafts it names (check 0), with res and floe's formats.
func build(u, file string, doc any, res map[string]Resource) (*Compiled, error) {
	c := &Compiled{Root: nil, url: u, file: file, res: res}
	if err := c.dialects(doc); err != nil {
		return nil, err
	}
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	for _, f := range Formats {
		comp.RegisterFormat(&jsonschema.Format{Name: f, Validate: func(any) error { return nil }})
	}
	for ru, r := range res {
		if ru == u {
			continue
		}
		if err := comp.AddResource(ru, r.Doc); err != nil {
			return nil, fmt.Errorf("schema: add %s: %w", ru, err)
		}
	}
	if err := comp.AddResource(u, doc); err != nil {
		return nil, fmt.Errorf("schema: add %s: %w", u, err)
	}
	comp.UseLoader(loader{file: file})
	root, err := comp.Compile(u)
	if err != nil {
		return nil, c.compileMistake(err)
	}
	c.Root = root
	return c, nil
}

// loader reads no URL: a schema refers to no file or URL outside what floe holds (a trust boundary).
// LIMIT: the mistake is told in the file compiled, not in the file whose $ref it is: a part of one's own
// refers to what floe does not hold in its own file, which CheckPart, run before Compile, tells there.
type loader struct{ file string }

func (l loader) Load(u string) (any, error) {
	if strings.HasPrefix(u, "floe:///parts/") {
		return nil, usererr.NewAt(l.file, 0, 0, "schema_no_part", u)
	}
	return nil, usererr.NewAt(l.file, 0, 0, "schema_no_url", u)
}

// compileMistake is the mistake of a failed compiling, in the library's words, at the file it is in.
func (c *Compiled) compileMistake(err error) error {
	var sve *jsonschema.SchemaValidationError
	var load *jsonschema.LoadURLError
	switch {
	case errors.As(err, &sve):
		var ve *jsonschema.ValidationError
		if !errors.As(sve.Err, &ve) {
			return usererr.LibraryAt(c.fileOf(sve.URL), 0, 0, sve.Err.Error())
		}
		base, frag, _ := strings.Cut(sve.URL, "#")
		var found usererr.List
		leaves(ve, func(e *jsonschema.ValidationError) {
			at := "#" + unescape(frag)
			for _, step := range e.InstanceLocation {
				at += "/" + escape(step)
			}
			found = append(found, usererr.LibraryIn(c.fileOf(base), at, e.ErrorKind.LocalizedString(english)))
		})
		return found.Sorted().Err()
	case errors.As(err, &load):
		var refused *usererr.Error
		if errors.As(load.Err, &refused) {
			return refused
		}
		return fmt.Errorf("schema: load %s: %w", load.URL, load.Err)
	}
	// The URL some of the library's mistakes hold names the file they are in; its fragment is what they
	// did not find, not where the $ref is.
	return usererr.LibraryAt(c.fileOf(urlOf(err)), 0, 0, err.Error())
}

// urlOf returns the URL of the schema the library's mistake err is of, "" for one of no URL.
func urlOf(err error) string {
	var (
		pointer  *jsonschema.JSONPointerNotFoundError
		invalid  *jsonschema.InvalidJsonPointerError
		anchor   *jsonschema.AnchorNotFoundError
		id       *jsonschema.ParseIDError
		anchorOf *jsonschema.ParseAnchorError
		twiceID  *jsonschema.DuplicateIDError
		twiceA   *jsonschema.DuplicateAnchorError
		regex    *jsonschema.InvalidRegexError
		parse    *jsonschema.ParseURLError
		vocab    *jsonschema.UnsupportedVocabularyError
	)
	switch {
	case errors.As(err, &pointer):
		return pointer.URL
	case errors.As(err, &invalid):
		return invalid.URL
	case errors.As(err, &anchor):
		return anchor.URL
	case errors.As(err, &id):
		return id.URL
	case errors.As(err, &anchorOf):
		return anchorOf.URL
	case errors.As(err, &twiceID):
		return twiceID.URL
	case errors.As(err, &twiceA):
		return twiceA.URL
	case errors.As(err, &regex):
		return regex.URL
	case errors.As(err, &parse):
		return parse.URL
	case errors.As(err, &vocab):
		return vocab.URL
	}
	return ""
}

// fileOf returns the file of the schema at u (a fragment is left out): the one compiled, or a resource's;
// the file compiled for one floe does not hold.
func (c *Compiled) fileOf(u string) string {
	base, _, _ := strings.Cut(u, "#")
	if r, ok := c.res[base]; ok && base != c.url {
		return r.File
	}
	return c.file
}

// leaves calls leaf with each mistake of e that is told: the causes of a mistake that only gathers others
// (of the whole schema, a group, allOf, a $ref) are opened; anyOf and oneOf are one mistake each.
func leaves(e *jsonschema.ValidationError, leaf func(*jsonschema.ValidationError)) {
	switch e.ErrorKind.(type) {
	case *kind.Schema, *kind.Group, *kind.AllOf, *kind.Reference:
		for _, cause := range e.Causes {
			leaves(cause, leaf)
		}
		if len(e.Causes) > 0 {
			return
		}
	}
	leaf(e)
}

// unescape returns the JSON pointer frag, as the library writes it in a Location or a URL (each step
// percent-encoded: ^x as %5Ex), with each step as JSON pointers write it: ~0 and ~1 alone escaped.
func unescape(frag string) string {
	steps := strings.Split(frag, "/")
	for i, step := range steps {
		if s, err := url.PathUnescape(step); err == nil {
			steps[i] = s
		}
	}
	return strings.Join(steps, "/")
}

// escape returns step, a key or an index, as a step of a JSON pointer.
func escape(step string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(step)
}
