package tmpl

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	neturl "net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// formats are the formats floe gives a meaning to, beyond JSON Schema's own: how to read a text, as
// HTML, the path of an image or Markdown. Validating them is a no-op: they mark an item for drawing.
var formats = []string{"html", "image", "markdown"}

// validate checks the values of d against sch, its template's schema.json as compiled, doc as read, which
// refers to the parts of set, and reports every mistake at its line and column under root, the data
// file's top mapping.
func validate(set *partSet, sch *jsonschema.Schema, doc any, d *Data, root ast.Node) error {
	err := sch.Validate(d.Values)
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err //nolint:wrapcheck // nil, or not a mistake in the data
	}
	s := &schemaCheck{file: d.File, template: d.Template, root: root, schema: doc, set: set, problems: nil}
	leaves(ve, s.leaf)
	if len(s.problems) == 0 {
		// A failure without a mistake to point at is not one the data can fix.
		return fmt.Errorf("check %s against its schema: %w", d.File, ve)
	}
	return s.problems
}

// schemaJSON reads src, a schema.json shown as file in its mistakes, as jsonschema reads it.
func schemaJSON(file string, src []byte) (any, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil {
		return nil, usererr.NewAt(file, 0, 0, "tmpl_json", err.Error())
	}
	return doc, nil
}

// compileSchema compiles doc, the schema of the template name as jsonschema reads it, which refers to the
// parts of set; file is where it is, in its mistakes.
func compileSchema(set *partSet, name, file string, doc any) (*jsonschema.Schema, error) {
	url := schemaURL(name)
	c, err := set.compiler(nil)
	if err != nil {
		return nil, err
	}
	if err = c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", name, err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		return nil, compileMistakes(url, file, err)
	}
	return sch, nil
}

// compileMistakes returns err, what stopped the compiling of the schema at url, file, as the mistakes of
// file (ownMistakes), a $ref of it that finds nothing (strayRefMistake), or what noLoader refused
// (loadedOrFailed): of a template's schema, or of a part's, the other parts standing as true
// (compileOwn). One of another schema, or of a kind floe does not know, is a failure.
func compileMistakes(url, file string, err error) error {
	if own := ownMistakes(url, file, err); own != nil {
		return own
	}
	if e := strayRefMistake(url, file, err); e != nil {
		return e
	}
	return loadedOrFailed(file, err)
}

// loadedOrFailed returns err, what stopped the compiling of the schema file, as the mistake noLoader made
// of a URL it refused, placed in file, or else as floe's failure. jsonschema wraps noLoader's mistake in an
// error of its own, which does not unwrap: floe's own is told, in the reader's language.
func loadedOrFailed(file string, err error) error {
	var (
		load *jsonschema.LoadURLError
		e    *usererr.Error
	)
	if errors.As(err, &load) && errors.As(load.Err, &e) {
		return e.Placed(file, 0, 0, "")
	}
	return fmt.Errorf("compile the schema %s: %w", file, err)
}

// ownMistakes returns err, what stopped the compiling of the schema at url, file, as the mistakes of file:
// those the metaschema refuses of the file, and those of a kind floe knows (compileMistake), each at its
// JSON pointer. It returns nil for one of another schema or of a kind floe does not know. The metaschema
// refuses what a $ref points at, at a pointer under a file, for the $ref, whosever it is: refMistake tells
// it at the $ref.
func ownMistakes(url, file string, err error) error {
	var invalid *jsonschema.SchemaValidationError
	if errors.As(err, &invalid) && strings.TrimSuffix(invalid.URL, "#") == url {
		if refused := metaschemaMistakes(file, invalid.Err); refused != nil {
			return refused
		}
	}
	if e := compileMistake(url, file, err); e != nil {
		return e
	}
	return nil
}

// strayRefMistake returns err, what stopped the compiling of the schema at url, file, as the mistake of a
// $ref that finds nothing in it, at the top of file, and nil for another. Where a $ref leads is all
// jsonschema says: compileMistakes calls it where none but the schema's own $refs lead into it, a
// template's, or a part's, the other parts standing as true (compileOwn).
// LIMIT: told at the top of the file, of a $ref refMistake did not check (its LIMITs), by where it leads
// as jsonschema resolves it: under a nested $id, a pointer not as written. Check it there if one comes.
func strayRefMistake(url, file string, err error) *usererr.Error {
	key, schema, leads := missingTarget(err)
	if _, ok := pointerIn(url, schema); key == "" || !ok {
		return nil
	}
	to, _ := pointerIn(url, leads)
	return notFound(key, to, path.Base(file), true).Placed(file, 0, 0, "#")
}

// compileMistake returns err, what stopped the compiling of the schema at url, file, as a mistake of what
// file holds at its JSON pointer, and nil for one of another schema or of a kind floe does not know. A $ref
// that finds nothing is not one: jsonschema says where it leads, which may be the file of another's $ref.
func compileMistake(url, file string, err error) *usererr.Error {
	in := func(u string) (string, bool) { return pointerIn(url, u) }
	var (
		id      *jsonschema.ParseIDError
		anchor  *jsonschema.ParseAnchorError
		anchors *jsonschema.DuplicateAnchorError
		ids     *jsonschema.DuplicateIDError
		regex   *jsonschema.InvalidRegexError
	)
	switch {
	case errors.As(err, &id):
		if at, ok := in(id.URL); ok {
			return usererr.NewIn(file, at, "tmpl_schema_id")
		}
	case errors.As(err, &anchor):
		if at, ok := in(anchor.URL); ok {
			return usererr.NewIn(file, at, "tmpl_schema_anchor")
		}
	case errors.As(err, &anchors):
		if _, ok := in(anchors.URL); ok {
			return usererr.NewIn(file, "#"+anchors.Ptr2, "tmpl_schema_anchor_twice", anchors.Anchor, "#"+anchors.Ptr1)
		}
	case errors.As(err, &ids):
		if _, ok := in(ids.URL); ok {
			return usererr.NewIn(file, "#"+ids.Ptr2, "tmpl_schema_id_twice", "#"+ids.Ptr1)
		}
	case errors.As(err, &regex):
		if at, ok := in(regex.URL); ok {
			return usererr.NewIn(file, at, "tmpl_schema_regex", regex.Regex, regex.Err)
		}
	}
	return nil
}

// refTargetMistake returns err, what stopped the compiling of what the $ref ref points at, as the mistake
// of the $ref, a $ref to `to` in `in`, the user's own file when own, when jsonschema found nothing there: no
// JSON pointer, nothing at it, or no $anchor of its name. It returns nil for none, and for a stop
// elsewhere, another $ref's or schema's.
func refTargetMistake(ref string, err error, to, in string, own bool) *usererr.Error {
	key, _, leads := missingTarget(err)
	if key == "" || !samePlace(ref, leads) {
		return nil
	}
	return notFound(key, to, in, own)
}

// refNoSchemaMistake returns err, what stopped the compiling of what the $ref ref points at, as the mistake
// of the $ref, a $ref to `to`, when what it points at is no schema JSON Schema takes, such as a value of an
// enum, or one the metaschema refuses, each why at its pointer under it, and nil for another.
// LIMIT: why, in jsonschema's English.
func refNoSchemaMistake(ref, to string, err error) *usererr.Error {
	var invalid *jsonschema.SchemaValidationError
	base, _, _ := strings.Cut(ref, "#")
	// To the root, it is what the metaschema refuses of the file, which compileMistakes tells where it is.
	if at, _ := pointerIn(base, ref); at == "#" || !errors.As(err, &invalid) || !samePlace(ref, invalid.URL) {
		return nil
	}
	whys := []string{invalid.Err.Error()}
	var ve *jsonschema.ValidationError
	if errors.As(invalid.Err, &ve) {
		whys = nil
		leaves(ve, func(e *jsonschema.ValidationError) {
			why := e.ErrorKind.LocalizedString(english)
			if len(e.InstanceLocation) > 0 {
				why = pointerTo(e.InstanceLocation)[1:] + ": " + why
			}
			whys = append(whys, why)
		})
	}
	return usererr.New("tmpl_schema_ref_no_schema", to, strings.Join(whys, "; "))
}

// samePlace reports whether the URL u, as jsonschema says it, is where ref points: the same schema and
// JSON pointer, its fragment escaped or not.
func samePlace(ref, u string) bool {
	base, _, _ := strings.Cut(ref, "#")
	want, _ := pointerIn(base, ref)
	got, ok := pointerIn(base, u)
	return ok && got == want
}

// missingTarget reads err as jsonschema's word that a $ref finds nothing where it points: no JSON pointer,
// nothing at it, or no $anchor of its name. It returns the key of floe's words for it, the URL of the
// schema jsonschema looked in, and where the $ref leads; "" for another error.
func missingTarget(err error) (key, schema, leads string) {
	var (
		nothing  *jsonschema.JSONPointerNotFoundError
		badPtr   *jsonschema.InvalidJsonPointerError
		noAnchor *jsonschema.AnchorNotFoundError
	)
	switch {
	case errors.As(err, &nothing):
		return "tmpl_schema_ref_nothing", nothing.URL, nothing.URL
	case errors.As(err, &badPtr):
		return "tmpl_schema_ref_pointer", badPtr.URL, badPtr.URL
	case errors.As(err, &noAnchor):
		return "tmpl_schema_ref_anchor", noAnchor.URL, noAnchor.Reference
	}
	return "", "", ""
}

// notFound is the mistake of key (missingTarget) of a $ref to `to`, which finds nothing in `in`, the user's
// own file when own, where what it refers to may be written.
func notFound(key, to, in string, own bool) *usererr.Error {
	switch {
	case key == "tmpl_schema_ref_pointer":
		return usererr.New(key, to) // no pointer: nothing was looked for
	case !own && key == "tmpl_schema_ref_nothing":
		return usererr.New("tmpl_schema_ref_nothing_there", to, in)
	case !own && key == "tmpl_schema_ref_anchor":
		return usererr.New("tmpl_schema_ref_anchor_there", to, in)
	}
	return usererr.New(key, to, in)
}

// drafts are the drafts of JSON Schema a $schema may name.
var drafts = []*jsonschema.Draft{
	jsonschema.Draft4, jsonschema.Draft6, jsonschema.Draft7, jsonschema.Draft2019, jsonschema.Draft2020,
}

// checkDraft refuses the schema doc when its $schema names no draft of JSON Schema: jsonschema would take a
// schema of floe's there, a part's, as the rules of the schema, and load any other.
func checkDraft(doc any) *usererr.Error {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	named, ok := m["$schema"].(string)
	if !ok || isDraft(named) {
		return nil
	}
	return usererr.New("tmpl_schema_draft", named)
}

// isDraft reports whether url names a draft of JSON Schema, as jsonschema reads it, which it holds itself.
func isDraft(url string) bool {
	return slices.ContainsFunc(drafts, func(d *jsonschema.Draft) bool { return sameDraft(d.String(), url) }) ||
		sameDraft("https://json-schema.org/schema", url)
}

// sameDraft reports whether the $schema named is the draft at url, as jsonschema reads it: by http or https,
// with an empty fragment or none.
func sameDraft(url, named string) bool {
	bare := func(u string) string {
		u = strings.TrimSuffix(u, "#")
		if rest, ok := strings.CutPrefix(u, "http://"); ok {
			return rest
		}
		return strings.TrimPrefix(u, "https://")
	}
	return bare(url) == bare(named)
}

// schemaURL is the URL of the schema of the template name, which a relative $ref is read from.
func schemaURL(name string) string {
	return "floe:///templates/" + name + "/schema.json"
}

// newCompiler returns a compiler of a schema that refers to the parts of set.
func newCompiler(set *partSet) *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	// A template's schema is one file: a URL it refers to, file:// above all, could read one out of the
	// template.
	c.UseLoader(noLoader{set: set})
	c.AssertFormat()
	for _, f := range formats {
		c.RegisterFormat(&jsonschema.Format{Name: f, Validate: func(any) error { return nil }})
	}
	return c
}

// compiler returns a compiler that holds the schemas of the parts of set, true, a schema anything fits, for
// those at the URLs stands has, and false, which nothing fits, for each part of one's own that cannot be
// used: a schema of floe's that refers to one compiles, and the data that takes it is refused there
// (schemaCheck.leaf), while one's own schema that names it is refused by its $ref (partMistake).
func (set *partSet) compiler(stands map[string]bool) (*jsonschema.Compiler, error) {
	c := newCompiler(set)
	docs := maps.Clone(set.docs)
	for name := range set.broken[kindPart] {
		if set.standsFalse(name) {
			docs[partsURL+name+".json"] = false
		}
	}
	for url, doc := range docs {
		if stands[url] {
			doc = true
		}
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("read the schema %s: %w", url, err)
		}
	}
	return c, nil
}

// knownFormat reports whether floe knows the format f: one of JSON Schema's that it checks, or one of
// its own. jsonschema passes a value of a format it does not know, so a template that uses a format of
// a later floe would be drawn as if it had none. Each is asked once, as every schema read asks again.
func knownFormat(f string) bool {
	if known, ok := knownFormats.Load(f); ok {
		b, isBool := known.(bool)
		return b && isBool
	}
	s, err := compileAlone(map[string]any{"format": f})
	known := err == nil && s.Format != nil
	knownFormats.Store(f, known)
	return known
}

// knownFormats are the formats knownFormat was asked of, by name, and whether floe knows each.
var knownFormats sync.Map

// notURIReference returns why ref, a $ref as written, is no URL, as uri, the schema of the format
// uri-reference, reads it, the one the metaschemas from draft-06 check a $ref with, and "" for one that is:
// draft-04's checks none.
// LIMIT: why, in jsonschema's English.
func notURIReference(uri *jsonschema.Schema, ref string) string {
	var ve *jsonschema.ValidationError
	if !errors.As(uri.Validate(ref), &ve) {
		return ""
	}
	why := ve.ErrorKind.LocalizedString(english)
	leaves(ve, func(e *jsonschema.ValidationError) {
		if f, ok := e.ErrorKind.(*kind.Format); ok && f.Err != nil {
			why = f.Err.Error()
			var bad *neturl.Error
			if errors.As(f.Err, &bad) {
				why = bad.Err.Error()
			}
		}
	})
	return why
}

// uriReference compiles, once, the schema of a URI reference, as JSON Schema's format uri-reference checks
// it.
var uriReference = sync.OnceValues(func() (*jsonschema.Schema, error) {
	return compileAlone(map[string]any{"type": "string", "format": "uri-reference"})
})

// heldByJSONSchema reports whether jsonschema holds the schema at url itself, as it holds the drafts and
// their vocabularies, so that a $ref to it reads no other file.
func heldByJSONSchema(url string) bool {
	_, err := compileAlone(map[string]any{"$ref": url})
	return err == nil
}

// compileAlone compiles the schema doc, which refers to no part: none are given, and any other file it
// refers to is refused as it is in a template.
func compileAlone(doc any) (*jsonschema.Schema, error) {
	const url = "floe:///alone.json"
	c := newCompiler(nil)
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("read a schema alone: %w", err)
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile a schema alone: %w", err)
	}
	return s, nil
}

// unknownPart is the mistake of a $ref, ref, under floe:///parts/ that names no part of set whole.
func unknownPart(set *partSet, ref string) *usererr.Error {
	if whole, into := set.pointsInto(ref); into {
		return usererr.New("tmpl_part_into", ref, whole)
	}
	name, ok := partName(ref)
	switch {
	case ok && set.broken[kindPart][name] != nil:
		return set.brokenErr(kindPart, name)
	case ok && set.parts[name] != nil && set.parts[name].isLook():
		return usererr.New("tmpl_part_look_ref", ref, name)
	}
	return usererr.New("tmpl_part_unknown", ref, set.names(drawing))
}

// pointerIn returns the JSON pointer u, a URL, holds in the schema at url, as the schema writes it, not
// escaped as a URL's fragment is, and false for another schema.
func pointerIn(url, u string) (string, bool) {
	base, ptr, _ := strings.Cut(u, "#")
	if written, bad := neturl.PathUnescape(ptr); bad == nil {
		ptr = written
	}
	return "#" + ptr, base == url
}

// metaschemaMistakes returns the mistakes of the schema file that err, the metaschema's, holds, each at its
// JSON pointer, in the order of the pointers, such as a pattern that is no regular expression; nil when it
// holds none. A $ref that is no URL is left out: its caller checks the schema's $refs (partSet.mistakes)
// before, which tells it at the $ref.
// LIMIT: why, in jsonschema's English.
func metaschemaMistakes(file string, err error) error {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	var mistakes usererr.List
	leaves(ve, func(e *jsonschema.ValidationError) {
		loc := e.InstanceLocation
		if _, format := e.ErrorKind.(*kind.Format); format && len(loc) > 0 && loc[len(loc)-1] == "$ref" {
			// A $ref no URL: partSet.mistakes tells it at the $ref, by the same check (notURIReference).
			return
		}
		why := e.ErrorKind.LocalizedString(english)
		mistakes = append(mistakes, usererr.NewIn(file, pointerTo(loc), "tmpl_schema_invalid", why))
	})
	slices.SortStableFunc(mistakes, func(a, b *usererr.Error) int { return strings.Compare(a.At, b.At) })
	return mistakes.Err()
}

// pointerTo is the JSON pointer of the place of steps, the keys and indexes jsonschema gives, under the root.
func pointerTo(steps []string) string {
	at := "#"
	for _, step := range steps {
		at += "/" + pointerKey.Replace(step)
	}
	return at
}

// leaves calls leaf with each mistake of e that is not only the mistakes under it, in an order of their
// own: jsonschema gives them in the order of its maps. A branch of anyOf or oneOf would say why each
// choice failed; it is one leaf instead. A mistake of propertyNames is given its place (namesPlace) before.
func leaves(e *jsonschema.ValidationError, leaf func(*jsonschema.ValidationError)) {
	leavesUnder(e, nil, leaf)
}

// leavesUnder is leaves of e, under the place above of the nearest mistake it is under.
func leavesUnder(e *jsonschema.ValidationError, above []string, leaf func(*jsonschema.ValidationError)) {
	switch e.ErrorKind.(type) {
	case *kind.Schema, *kind.Group, *kind.AllOf, *kind.Reference:
		causes := slices.SortedStableFunc(slices.Values(e.Causes), func(a, b *jsonschema.ValidationError) int {
			return cmp.Or(strings.Compare(a.SchemaURL, b.SchemaURL),
				strings.Compare(a.ErrorKind.LocalizedString(english), b.ErrorKind.LocalizedString(english)),
				slices.Compare(a.InstanceLocation, b.InstanceLocation))
		})
		for _, c := range causes {
			leavesUnder(c, e.InstanceLocation, leaf)
		}
		return
	case *kind.PropertyNames:
		e.InstanceLocation = namesPlace(e, above)
	}
	leaf(e)
}

// namesPlace is the place of e, a mistake of propertyNames, under above, the place of the nearest mistake
// it is under: jsonschema gives e a place the later checks write over, right in its length alone. One a
// metaschema finds is at the keyword of metaNames under the schema above; another is told at above.
// LIMIT: jsonschema v6.0.3 does not copy the place of propertyNames (issue #276, PR #269 of
// santhosh-tekuri/jsonschema); drop this once floe takes a version that does. On an upgrade, grep
// 'InstanceLocation = vd.vloc' for more: contentSchema's is the same, which floe does not assert.
// LIMIT: one of a data file's schema is told at what holds it until then, the top level, say, for a key
// of an item under it.
func namesPlace(e *jsonschema.ValidationError, above []string) []string {
	at := slices.Clone(above)
	if len(e.InstanceLocation) != len(above)+1 {
		return at
	}
	for _, m := range metaNames {
		if e.SchemaURL == m.metaschema+"#/properties/"+m.keyword+"/propertyNames" {
			return append(at, m.keyword)
		}
	}
	return at
}

// metaNames are the keywords of a schema whose keys the metaschemas of JSON Schema check by propertyNames,
// each with the metaschema that does: those of patternProperties, regular expressions, and of $vocabulary,
// URLs. Draft-06 and draft-04 check none.
var metaNames = []struct{ metaschema, keyword string }{
	{"https://json-schema.org/draft/2020-12/meta/applicator", "patternProperties"},
	{"https://json-schema.org/draft/2020-12/meta/core", "$vocabulary"},
	{"https://json-schema.org/draft/2019-09/meta/applicator", "patternProperties"},
	{"https://json-schema.org/draft/2019-09/meta/core", "$vocabulary"},
	{"http://json-schema.org/draft-07/schema", "patternProperties"},
}

// noLoader loads no schema: those of the parts of set are added to the compiler before.
type noLoader struct{ set *partSet }

func (n noLoader) Load(url string) (any, error) {
	if strings.HasPrefix(url, partsURL) {
		return nil, unknownPart(n.set, url)
	}
	return nil, usererr.New("tmpl_schema_url", url)
}

type schemaCheck struct {
	file, template string
	root           ast.Node
	schema         any      // schema.json as read, to list the items a mapping may hold
	set            *partSet // the parts it refers to
	problems       usererr.List
}

// leaf turns e, a leaf of the mistakes of the data (leaves), into problems.
func (s *schemaCheck) leaf(e *jsonschema.ValidationError) {
	key, value, at := yamlpos.Find(s.root, e.InstanceLocation)
	where := yamlpos.OfValue(key, value)
	if name, ok := s.unusablePart(e); ok {
		// As a theme the data picks that cannot be used: at the item, why under it, once.
		s.problems = append(s.problems, s.set.brokenErr(kindPart, name).Placed(s.file, where.Line, where.Column, ""))
		return
	}
	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		for _, name := range k.Missing {
			s.add(where, "tmpl_schema_required", join(at, name))
		}
	case *kind.AdditionalProperties:
		s.unknown(e, k.Properties, where)
	case *kind.Type:
		args := []any{label(at), usererr.New("tmpl_type_" + k.Got), Types(k.Want)}
		if k.Got == "object" && slices.Contains(k.Want, "string") && yamlpos.ColonText(value) {
			s.problems = append(s.problems, yamlpos.NewHinted(s.file, where, "yaml_hint_colon", "tmpl_schema_type", args...))
			return
		}
		s.add(where, "tmpl_schema_type", args...)
	case *kind.Enum:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = jsonText(w)
		}
		if k.Got == nil {
			s.add(where, "tmpl_schema_enum_empty", label(at), strings.Join(want, ", "))
			return
		}
		s.add(where, "tmpl_schema_enum", label(at), jsonText(k.Got), strings.Join(want, ", "))
	default:
		if key, args, ok := wording(e.ErrorKind); ok {
			s.add(where, key, append([]any{label(at)}, args...)...)
			return
		}
		// LIMIT: the rest (anyOf, oneOf, const, dependencies…) keep the library's English. Word one
		// in the catalogs when a template's schema comes to use it.
		s.add(where, "tmpl_item", label(at), e.ErrorKind.LocalizedString(english))
	}
}

// unusablePart returns the part of one's own that cannot be used that e stopped at, the false that stands for
// it (partSet.compiler), and false for another e.
func (s *schemaCheck) unusablePart(e *jsonschema.ValidationError) (string, bool) {
	if _, isFalse := e.ErrorKind.(*kind.FalseSchema); !isFalse {
		return "", false
	}
	base, _, _ := strings.Cut(e.SchemaURL, "#")
	name, whole := partName(base)
	return name, whole && s.set.standsFalse(name)
}

// standsFalse reports whether the part name, left out, stands as false in compiler: one that cannot be used
// of a name a part can take, whose schema set holds no longer.
func (set *partSet) standsFalse(name string) bool {
	return set.brokenNamed(name) && set.docs[partsURL+name+".json"] == nil
}

// brokenNamed reports whether name is a part that cannot be used, of a name a part can take, which a $ref
// can name: not one refused for its name (tmpl_part_name), nor floe's that only floe may take.
func (set *partSet) brokenNamed(name string) bool {
	return set.broken[kindPart][name] != nil && validName(name) && !set.reserved(name)
}

// wording words a mistake the catalog has a message for, after the path: a bound (how many items or
// keys, how long, how large), a pattern or a duplicate.
func wording(k jsonschema.ErrorKind) (key string, args []any, ok bool) {
	switch k := k.(type) {
	case *kind.MinItems:
		return "tmpl_schema_min_items", []any{k.Want}, true
	case *kind.MaxItems:
		return "tmpl_schema_max_items", []any{k.Want}, true
	case *kind.MinProperties:
		return "tmpl_schema_min_keys", []any{k.Want}, true
	case *kind.MaxProperties:
		return "tmpl_schema_max_keys", []any{k.Want}, true
	case *kind.MinLength:
		return "tmpl_schema_min_length", []any{k.Want}, true
	case *kind.MaxLength:
		return "tmpl_schema_max_length", []any{k.Want}, true
	case *kind.Minimum:
		return "tmpl_schema_minimum", []any{number(k.Want)}, true
	case *kind.Maximum:
		return "tmpl_schema_maximum", []any{number(k.Want)}, true
	case *kind.ExclusiveMinimum:
		return "tmpl_schema_above", []any{number(k.Want)}, true
	case *kind.ExclusiveMaximum:
		return "tmpl_schema_below", []any{number(k.Want)}, true
	case *kind.Pattern:
		return "tmpl_schema_pattern", []any{k.Got, k.Want}, true
	case *kind.UniqueItems:
		// The library counts from 0; people count items from 1.
		return "tmpl_schema_unique", []any{k.Duplicates[0] + 1, k.Duplicates[1] + 1}, true
	}
	return "", nil, false
}

var english = message.NewPrinter(language.English)

// unknown reports the items of the mapping at e that the template does not have, each at its key
// (or at the mapping, where), with the items it has.
func (s *schemaCheck) unknown(e *jsonschema.ValidationError, extras []string, where yamlpos.Pos) {
	names := s.properties(e.SchemaURL)
	if len(e.InstanceLocation) == 0 {
		names = append(names, shared...)
		slices.Sort(names)
	}
	for _, name := range extras {
		extra, _, path := yamlpos.Find(s.root, append(slices.Clone(e.InstanceLocation), name))
		at := where
		if extra != nil {
			at = yamlpos.Of(extra)
		}
		if len(names) == 0 {
			// Items the schema allows by pattern or through $ref have no list to give.
			s.add(at, "tmpl_schema_unknown_bare", path, s.template)
			continue
		}
		s.add(at, "tmpl_schema_unknown", path, s.template, strings.Join(names, ", "))
	}
}

func (s *schemaCheck) add(at yamlpos.Pos, key string, args ...any) {
	s.problems = append(s.problems, yamlpos.New(s.file, at, key, args...))
}

// properties lists the items the mapping at the schema location url declares, for a misspelt one: in
// the template's schema, or in a part's of floe's.
func (s *schemaCheck) properties(url string) []string {
	base, frag, _ := strings.Cut(url, "#")
	v := s.schema
	if strings.HasPrefix(base, partsURL) {
		v = s.set.docs[base]
	}
	for step := range strings.SplitSeq(frag, "/") {
		if step == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[pointerStep.Replace(step)]
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	if parent, ok := m["properties"].(map[string]any); ok {
		names := make([]string, 0, len(parent))
		for n := range parent {
			names = append(names, n)
		}
		slices.Sort(names)
		return names
	}
	return nil
}

// pointerStep unescapes a step of a JSON pointer.
var pointerStep = strings.NewReplacer("~1", "/", "~0", "~")

// join adds name to the path at.
func join(at, name string) string {
	if at == "" {
		return name
	}
	return at + "." + name
}

// label names the item at in a message: its path, or the top level.
func label(at string) any {
	if at == "" {
		return usererr.New("tmpl_top_level")
	}
	return at
}

func number(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Types words JSON types (string, array…) as alternatives in a message: text or a number. A name
// JSON does not have stays as written.
func Types(types []string) i18n.Localizer {
	words := make(orList, len(types))
	for i, t := range types {
		words[i] = t
		if slices.Contains(jsonTypes, t) {
			words[i] = usererr.New("tmpl_type_" + t)
		}
	}
	return words
}

var jsonTypes = []string{"array", "boolean", "integer", "null", "number", "object", "string"}

// jsonText writes a value of JSON as JSON does, text as it is: null rather than Go's <nil>.
func jsonText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// andList words its items as a list: page.html, schema.json.
type andList []string

func (l andList) Localize(c *i18n.Catalog) string {
	return strings.Join(l, c.Msg("tmpl_and"))
}

// orList words its items as alternatives: text or a number.
type orList []any

func (l orList) Localize(c *i18n.Catalog) string {
	words := make([]string, len(l))
	for i, w := range l {
		if loc, ok := w.(i18n.Localizer); ok {
			w = loc.Localize(c)
		}
		words[i] = fmt.Sprint(w)
	}
	return strings.Join(words, c.Msg("tmpl_or"))
}
