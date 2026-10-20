package tmpl

import (
	"regexp"
	"strings"
)

// tag is a tag of a page, a start tag or an end tag: its name in lower case, where it starts, and the
// attributes of a start tag.
type tag struct {
	name  string
	end   bool
	start int
	attrs []attr
}

// attr is an attribute of a tag: its name in lower case, its value without its quotes, and where the value
// starts as written, a quote included, -1 for one without a value.
type attr struct {
	name    string
	value   string
	valueAt int
}

// tagsIn reads the tags of page in order: a comment, a doctype, and the text of the elements an HTML parser
// reads as text to their end tag hold none (script, style, textarea, title…; a sandboxed page runs its
// scripts, so a noscript's text is never read as HTML). In a tag, a quoted value is read whole, a > in it
// too. When page is a template (actions), an action ({{…}}) is read whole, in a tag or out of one: its
// text is no markup of the page's. HTML that is drawn, or the data's, holds none, so {{ there is text.
// LIMIT: the elements of SVG and MathML are read as HTML's (a noscript in an svg holds tags to a browser,
// not here); follow the parser's foreign content if a page comes to write markup so.
func tagsIn(page string, actions bool) []tag {
	var found []tag
	action := nextAction(page, actions)
	for i := 0; i < len(page); {
		lt := strings.IndexByte(page[i:], '<')
		if lt < 0 {
			break
		}
		if at := action(i); at < i+lt {
			i = pastAction(page, at)
			continue
		}
		t, next := readMarkup(page, i+lt, actions)
		if t != nil {
			found = append(found, *t)
		}
		if next < 0 {
			break
		}
		i = next
	}
	return found
}

// nextAction returns a function that tells where the next action of page is from an offset on, past the
// end for none, and none at all unless actions. It looks ahead again only once past the one it found.
func nextAction(page string, actions bool) func(from int) int {
	at := -1
	return func(from int) int {
		if !actions {
			return len(page)
		}
		if at < from {
			at = len(page)
			if i := strings.Index(page[from:], "{{"); i >= 0 {
				at = from + i
			}
		}
		return at
	}
}

// readMarkup reads what the < at at in page starts: a comment or a doctype, which it passes, a tag, which
// it returns, past the text of an element read as text, or a < of text. It returns where to read on, -1
// past a comment never closed.
func readMarkup(page string, at int, actions bool) (*tag, int) {
	rest := page[at:]
	switch {
	case strings.HasPrefix(rest, "<!"), strings.HasPrefix(rest, "<?"):
		n := pastComment(rest)
		if n <= 0 {
			return nil, -1
		}
		return nil, at + n
	case len(rest) > 1 && isLetter(rest[1]), len(rest) > 2 && rest[1] == '/' && isLetter(rest[2]):
		t, next := readTag(page, at, actions)
		return &t, pastText(page, t, next)
	}
	return nil, at + 1
}

// pastText returns where the text of the element whose tag t ends at next ends: at its end tag for one an
// HTML parser reads as text (a script's, a textarea's…), at the end of page for a plaintext or one never
// ended, and at next for the others.
func pastText(page string, t tag, next int) int {
	closing := endTag[t.name]
	switch {
	case t.end:
		return next
	case t.name == "plaintext":
		return len(page)
	case closing == nil:
		return next
	}
	if loc := closing.FindStringIndex(page[next:]); loc != nil {
		return next + loc[0]
	}
	return len(page)
}

// pastComment returns the length of the comment or doctype rest starts with, 0 for one never closed: a
// comment ends where an HTML parser ends it, and a doctype, or a comment of another kind, at its >.
func pastComment(rest string) int {
	if strings.HasPrefix(rest, "<!--") || strings.HasPrefix(rest, "<?") {
		return comment(rest)
	}
	return strings.IndexByte(rest, '>') + 1
}

// endTag finds the end tag of the elements whose text an HTML parser reads as text, holding no tags: raw
// text and escapable raw text (a textarea's and a title's).
var endTag = func() map[string]*regexp.Regexp {
	ends := map[string]*regexp.Regexp{}
	for _, name := range []string{
		"script", "noscript", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes",
	} {
		ends[name] = regexp.MustCompile(`(?i)</` + name + `[\s/>]`)
	}
	return ends
}()

// styleOrLink finds a <style> or a <link>, however the HTML around it reads.
var styleOrLink = regexp.MustCompile(`(?i)<(style|link)[\s/>]`)

// tagStart finds HTML that ends where a tag or a comment starts, which what comes after it would go on.
var tagStart = regexp.MustCompile(`<(?:/|!-?|\?)?$`)

// headEnd finds a </head>, however the HTML around it reads.
var headEnd = regexp.MustCompile(`(?i)</head[\s/>]`)

// readTag reads the tag of page at at, a < before a letter or a /, and returns it and where it ends; an
// action of a template is read whole when actions.
func readTag(page string, at int, actions bool) (tag, int) {
	t := tag{name: "", end: false, start: at, attrs: nil}
	i := at + 1
	if page[i] == '/' {
		t.end, i = true, i+1
	}
	name := i
	for i < len(page) && !isSpace(page[i]) && page[i] != '/' && page[i] != '>' {
		i++
	}
	t.name = strings.ToLower(page[name:i])
	for i < len(page) {
		switch c := page[i]; {
		case c == '>':
			return t, i + 1
		case isSpace(c), c == '/':
			i++
		case actions && strings.HasPrefix(page[i:], "{{"):
			i = pastAction(page, i)
		default:
			var a attr
			a, i = readAttr(page, i, actions)
			t.attrs = append(t.attrs, a)
		}
	}
	return t, len(page)
}

// readAttr reads the attribute of page at at, its name and its value after an =, and returns it and where
// it ends, an action of a template read whole when actions. An = before any name is one of its own, as an
// HTML parser reads it.
func readAttr(page string, at int, actions bool) (attr, int) {
	i := at + 1
	for i < len(page) && !isSpace(page[i]) && !strings.ContainsRune("/>=", rune(page[i])) &&
		(!actions || !strings.HasPrefix(page[i:], "{{")) {
		i++
	}
	a := attr{name: strings.ToLower(page[at:i]), value: "", valueAt: -1}
	if j := pastSpace(page, i); j < len(page) && page[j] == '=' {
		a.valueAt = pastSpace(page, j+1)
		i = pastValue(page, a.valueAt, actions)
		a.value = strings.Trim(page[a.valueAt:i], `"'`)
	}
	return a, i
}

// pastValue returns where the value of an attribute that starts at at in page ends: at its closing quote,
// or before a space or a >, an action of a template read whole when actions.
func pastValue(page string, at int, actions bool) int {
	quote := byte(0)
	if at < len(page) && (page[at] == '"' || page[at] == '\'') {
		quote, at = page[at], at+1
	}
	for at < len(page) {
		switch c := page[at]; {
		case actions && strings.HasPrefix(page[at:], "{{"):
			at = pastAction(page, at)
		case quote != 0 && c == quote:
			return at + 1
		case quote == 0 && (isSpace(c) || c == '>'):
			return at
		default:
			at++
		}
	}
	return at
}

// pastAction returns where the action of a template that starts at at in page, {{, ends.
func pastAction(page string, at int) int {
	if end := strings.Index(page[at+2:], "}}"); end >= 0 {
		return at + 2 + end + 2
	}
	return len(page)
}

// pastSpace returns where the white space at at in page ends.
func pastSpace(page string, at int) int {
	for at < len(page) && isSpace(page[at]) {
		at++
	}
	return at
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func isLetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// tags are the offsets of the first </head>, <style> and <link> of a page, -1 for one it lacks.
type tags struct{ head, style, link int }

// tagsOf finds the tags of page, of found, the tags tagsIn reads.
func tagsOf(found []tag) tags {
	at := tags{head: -1, style: -1, link: -1}
	for _, t := range found {
		var first *int
		switch {
		case t.end && t.name == "head":
			first = &at.head
		case !t.end && t.name == "style":
			first = &at.style
		case !t.end && t.name == "link":
			first = &at.link
		default:
			continue
		}
		if *first < 0 {
			*first = t.start
		}
	}
	return at
}
