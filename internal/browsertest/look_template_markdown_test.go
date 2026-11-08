package browsertest_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// headingsMarkdown is the Markdown put before each of a template's own fields of format markdown, and in a text.
const headingsMarkdown = "# Heading one\n\n## Heading two\n\n### Heading three\n\n"

// measureHeadings returns, for each heading headingsMarkdown writes, the look of every one drawn: its size, weight,
// spacing, case, style, frame and padding, one heading to a line. The size of h1 and h2, which base.css gives in em, is
// read against the box around them, so that a page.css may size the text of a field; h3's is in rem there, read as it is.
const measureHeadings = `(() => ['Heading one', 'Heading two', 'Heading three'].map((t) =>
  [...document.querySelectorAll('h1, h2, h3, h4, h5, h6')].filter((h) => h.textContent.trim() === t).map((h) => {
    const s = getComputedStyle(h), size = parseFloat(s.fontSize);
    const ems = (v) => (parseFloat(v) / size).toFixed(3) + 'em';
    return t + ': ' + [h.matches('h1, h2') ? (size / parseFloat(getComputedStyle(h.parentElement).fontSize)).toFixed(3) + 'em' : s.fontSize,
      s.fontWeight, s.letterSpacing === 'normal' ? 'normal' : ems(s.letterSpacing), s.textTransform, s.fontStyle,
      ...['borderTopWidth', 'borderRightWidth', 'borderBottomWidth', 'borderLeftWidth', 'paddingTop', 'paddingRight', 'paddingBottom',
        'paddingLeft'].map((p) => s[p])].join(' ');
  }).join('\n')).join('\n'))()`

// markdownFields returns where the template's own fields of format markdown stand in values, as the schema's Walk
// finds them; not those a part's value holds.
func markdownFields(t *testing.T, c *schema.Compiled, values map[string]any) [][]string {
	t.Helper()

	var at [][]string
	if _, err := c.Walk(values, func(it schema.Item, ready any) (any, error) {
		if it.Format == "markdown" && !it.InPart {
			at = append(at, it.At)
		}
		return ready, nil
	}); err != nil {
		t.Fatal(err)
	}
	return at
}

// prepend puts text before the string at the path at in v.
func prepend(t *testing.T, v any, at []string, text string) {
	t.Helper()

	last := len(at) - 1
	for _, key := range at[:last] {
		switch c := v.(type) {
		case map[string]any:
			v = c[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil {
				t.Fatal(err)
			}
			v = c[i]
		default:
			t.Fatalf("%v: %q is in a %T, not in keys and values or a list", at, key, v)
		}
	}
	headed := func(old any) string {
		s, ok := old.(string)
		if !ok {
			t.Fatalf("%v: a field of format markdown that is not a string", at)
		}
		return text + s
	}
	switch c := v.(type) {
	case map[string]any:
		c[at[last]] = headed(c[at[last]])
	case []any:
		i, err := strconv.Atoi(at[last])
		if err != nil {
			t.Fatal(err)
		}
		c[i] = headed(c[i])
	default:
		t.Fatalf("%v: %q is in a %T, not in keys and values or a list", at, at[last], v)
	}
}

// lookTemplateMarkdown draws the example of each floe template with headings at the head of each of its own fields
// of format markdown, in every theme: each heading is drawn as the same Markdown is in a text, its size, weight,
// frame and padding, not as the template's own headings, its title's and its sections'. A rule of the template's
// page.css that names its headings by the element alone reaches the author's too.
// LIMIT: a field the example leaves out goes unmeasured, and the look but of headings (a rule of page.css that
// reaches a paragraph in a field arranges the template's own Markdown, as it may).
func lookTemplateMarkdown(t *testing.T) { //nolint:paralleltest,thelper // a subtest of TestLookRule; the browser tests share one Chrome
	s := newSite(t)
	b := s.b
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, templates := k.Names()
	text := "template: report\nlang: en\ntitle: T\nsections:\n  - heading: S\n    parts:\n      - text: |\n" +
		"          " + strings.ReplaceAll(strings.TrimSuffix(headingsMarkdown, "\n"), "\n", "\n          ") + "\n          said\n"
	want := map[string]string{}
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "text-"+theme+".yaml", text+"theme: "+theme+"\n"), "--slug", "text-"+theme)
		b.navigate("/a/text-" + theme + "/?raw")
		want[theme] = b.eval(measureHeadings)
	}
	fields := 0
	for _, name := range templates {
		tpl, ok := k.Template(name)
		if !ok {
			t.Fatalf("floe has no template %s, though it names it", name)
		}
		_, values, err := yamlpos.ParseMap(name+".yaml", tpl.Example())
		if err != nil {
			t.Fatal(err)
		}
		at := markdownFields(t, tpl.Schema(), values)
		if len(at) == 0 {
			continue
		}
		fields += len(at)
		for _, a := range at {
			prepend(t, values, a, headingsMarkdown)
		}
		for _, theme := range themes {
			values["theme"] = theme
			// JSON is YAML, written with no HTML escaped.
			var data bytes.Buffer
			enc := json.NewEncoder(&data)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(values); err != nil {
				t.Fatal(err)
			}
			slug := "md-" + name + "-" + theme
			s.floe(t, "publish", s.write(t, slug+".yaml", data.String()), "--slug", slug)
			b.navigate("/a/" + slug + "/?raw")
			// Each field draws the three headings: the page holds them once a field, in the order of the fields.
			got := strings.Split(b.eval(measureHeadings), "\n")
			wanted := make([]string, 0, len(got))
			for _, line := range strings.Split(want[theme], "\n") {
				for range at {
					wanted = append(wanted, line)
				}
			}
			if strings.Join(got, "\n") != strings.Join(wanted, "\n") {
				t.Errorf("%s in %s: a heading in its Markdown is not drawn as in a text:\n  got  %s\n  want %s", name, theme,
					strings.Join(got, "\n       "), strings.Join(wanted, "\n       "))
			}
		}
	}
	if fields == 0 {
		t.Fatal("no floe template has a field of format markdown in its example: nothing measured")
	}
}
