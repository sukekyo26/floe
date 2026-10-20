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

// tagsIn reads the tags of page in order: a comment, a doctype, and the text of a script, a noscript or a
// style hold none (a sandboxed page runs its scripts, so a noscript's text is never read as HTML). In a
// tag, a quoted value is read whole, a > in it too, and so is an action of a template ({{…}}) when page
// is one (actions): HTML that is drawn, or the data's, holds none, so {{ there is text.
// LIMIT: the tags are read as an HTML parser reads them but for the rarer states (the text of a textarea or
// a title holds tags here); follow the parser's states if a page comes to write tags in those.
func tagsIn(page string, actions bool) []tag {
	var found []tag
	for i := 0; i < len(page); {
		lt := strings.IndexByte(page[i:], '<')
		if lt < 0 {
			break
		}
		i += lt
		rest := page[i:]
		switch {
		case strings.HasPrefix(rest, "<!"), strings.HasPrefix(rest, "<?"):
			n := pastComment(rest)
			if n <= 0 {
				return found
			}
			i += n
		case len(rest) > 1 && isLetter(rest[1]), len(rest) > 2 && rest[1] == '/' && isLetter(rest[2]):
			t, next := readTag(page, i, actions)
			found = append(found, t)
			i = next
			// The text of a script, a noscript or a style runs to its end tag.
			if closing := endTag[t.name]; closing != nil && !t.end {
				loc := closing.FindStringIndex(page[i:])
				if loc == nil {
					return found
				}
				i += loc[0]
			}
		default:
			i++
		}
	}
	return found
}

// pastComment returns the length of the comment or doctype rest starts with, 0 for one never closed: a
// comment ends where an HTML parser ends it, and a doctype, or a comment of another kind, at its >.
func pastComment(rest string) int {
	if strings.HasPrefix(rest, "<!--") || strings.HasPrefix(rest, "<?") {
		return comment([]byte(rest))
	}
	return strings.IndexByte(rest, '>') + 1
}

// endTag finds the end tag of the elements whose text holds no tags.
var endTag = map[string]*regexp.Regexp{
	"script":   regexp.MustCompile(`(?i)</script[\s/>]`),
	"noscript": regexp.MustCompile(`(?i)</noscript[\s/>]`),
	"style":    regexp.MustCompile(`(?i)</style[\s/>]`),
}

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
