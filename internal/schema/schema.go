// Package schema compiles a template's schema.json with the schemas floe holds (its parts and the schemas
// they share), checks a data file's values against it, and tells floe how it draws them: which item a part
// or one of floe's formats draws. The library's mistakes are told in its words; floe's own checks are those
// of what it draws by, and of the files a schema may read.
package schema

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sukekyo26/floe/internal/bom"
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
// its mistakes are told in. It is made by NewResource only, so that its JSON is read as read reads it.
type Resource struct {
	doc  any
	made bool // by NewResource: a doc of nil is the JSON null, not a Resource left zero
	Kind Kind
	File string
}

// NewResource reads src (the schema of file) as read does, a Resource of k.
func NewResource(file string, src []byte, k Kind) (Resource, error) {
	doc, err := read(file, src)
	if err != nil {
		return Resource{}, err
	}
	return Resource{doc: doc, made: true, Kind: k, File: file}, nil
}

// read reads src, the JSON of file, as RFC 8259 does, past a byte order mark at its start, and as jsonschema
// reads it (numbers as json.Number): the only place a schema's bytes are read.
// LIMIT: other marks and UTF-16/32 are refused without saying so (S6); rework here when S6 decides them.
func read(file string, src []byte) (any, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(bom.Trim(src)))
	if err != nil {
		return nil, usererr.LibraryAt(file, 0, 0, err.Error())
	}
	return doc, nil
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
// floe's checks, all together. The $refs it cannot resolve are told as compile tells them.
func Compile(url, file string, src []byte, res map[string]Resource) (*Compiled, error) {
	doc, err := read(file, src)
	if err != nil {
		return nil, err
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
	c, err := build(url, r.File, r.doc, res)
	if err != nil {
		return err
	}
	return c.checkPart()
}

// build compiles doc under url, after checking the drafts it names (check 0), with res and floe's formats.
func build(u, file string, doc any, res map[string]Resource) (*Compiled, error) {
	for _, ru := range slices.Sorted(maps.Keys(res)) {
		if !res[ru].made {
			return nil, fmt.Errorf("schema: %s was not made by NewResource", ru) //nolint:err113 // floe's own mistake
		}
	}
	c := &Compiled{Root: nil, url: u, file: file, res: res}
	if err := c.dialects(doc); err != nil {
		return nil, err
	}
	root, err := c.compile(doc)
	if err != nil {
		return nil, err
	}
	c.Root = root
	return c, nil
}

// The most mistakes compile tells, and the most times it compiles.
const (
	mostMistakes = 100
	mostCompiles = 201
)

// compile compiles doc under c.url with c.res. The library stops at the first $ref it cannot resolve, the
// first in the order of a map, so compile fills the holes the library names in two shapes alone, where
// filling decides no meaning of the schema, and compiles again: a #/$defs/<name> missing from an object the
// template or res holds, filled with true; and a URL the loader refused, held as {}, its #/$defs/<name>
// filled unsaid. Any other mistake is told and stops it; one inside a URL it refused, which {} made, stops
// it unsaid. The files of the mistakes come in the order of their URLs, and Sorted orders each file's.
// LIMIT: with a stopping mistake, or at mostMistakes or mostCompiles, what is told changes from run to run.
// LIMIT: each hole compiles the whole again, at most mostCompiles times; rework if that comes to be slow.
func (c *Compiled) compile(doc any) (*jsonschema.Schema, error) {
	docs := map[string]any{c.url: doc}
	for ru, r := range c.res {
		if ru != c.url {
			docs[ru] = r.doc
		}
	}
	h := &holes{docs: docs, copied: map[string]bool{}, refused: map[string]bool{}}
	type told struct {
		url string
		err error
	}
	var found []told
	for compiles := 1; ; compiles++ {
		comp, failed := h.compiler(c.file)
		if failed != nil {
			return nil, failed
		}
		root, err := comp.Compile(c.url)
		if err == nil {
			if len(found) == 0 {
				return root, nil
			}
			break
		}
		at, tell, again := h.hole(err)
		if tell {
			found = append(found, told{url: at, err: c.compileMistake(err)})
		}
		if !again || len(found) >= mostMistakes || compiles >= mostCompiles {
			break
		}
	}
	slices.SortStableFunc(found, func(x, y told) int { return strings.Compare(x.url, y.url) })
	var all usererr.List
	for _, f := range found {
		if failed := all.Add(f.err); failed != nil {
			return nil, failed //nolint:wrapcheck // floe's failure, as it is
		}
	}
	return nil, all.Sorted().Err()
}

// holes are the docs compile compiles, each copied before it is filled, and the URLs the loader refused.
type holes struct {
	docs    map[string]any
	copied  map[string]bool
	refused map[string]bool
}

// compiler returns a compiler of its own holding every doc of h; its error is floe's failure.
func (h *holes) compiler(file string) (*jsonschema.Compiler, error) {
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	for _, f := range Formats {
		comp.RegisterFormat(&jsonschema.Format{Name: f, Validate: func(any) error { return nil }})
	}
	for du, d := range h.docs {
		if err := comp.AddResource(du, d); err != nil {
			return nil, fmt.Errorf("schema: add %s: %w", du, err)
		}
	}
	comp.UseLoader(loader{file: file})
	return comp, nil
}

// hole reads err, a mistake of compiling: the URL it is at, whether it is told, and whether h filled the
// hole so that compiling again goes on.
func (h *holes) hole(err error) (at string, tell, again bool) {
	var load *jsonschema.LoadURLError
	var refused *usererr.Error
	if errors.As(err, &load) && errors.As(load.Err, &refused) {
		h.docs[load.URL] = map[string]any{}
		h.copied[load.URL] = true
		h.refused[load.URL] = true
		return load.URL, true, true
	}
	at = urlOf(err)
	base, frag, _ := strings.Cut(at, "#")
	var pointer *jsonschema.JSONPointerNotFoundError
	if !errors.As(err, &pointer) {
		return at, !h.refused[base], false
	}
	return at, !h.refused[base], h.fill(base, unescape(frag))
}

// fill puts true at frag of the doc at base, a #/$defs/<name> alone, in a copy of the doc; false for any
// other place, or a doc whose root or $defs is not an object. It reads and writes the copy's root and $defs
// alone (CLAUDE.md: the exception to taking the schema's shape from the compiled tree).
func (h *holes) fill(base, frag string) bool {
	name, ok := strings.CutPrefix(frag, "/$defs/")
	if !ok || name == "" || strings.ContainsAny(name, "/~") {
		return false
	}
	root, ok := h.docs[base].(map[string]any)
	if !ok {
		return false
	}
	defs := map[string]any{}
	if v, has := root["$defs"]; has {
		if defs, ok = v.(map[string]any); !ok {
			return false
		}
	}
	if !h.copied[base] {
		root, defs = maps.Clone(root), maps.Clone(defs)
		h.copied[base] = true
	}
	defs[name] = true
	root["$defs"] = defs
	h.docs[base] = root
	return true
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
