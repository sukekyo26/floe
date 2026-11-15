package browsertest_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"floe/internal/kit"
	"floe/internal/schema"
	"floe/internal/yamlpos"
)

// headingsMarkdown is the Markdown put before each of a template's own fields of format markdown, and in a text.
const headingsMarkdown = "# Heading one\n\n## Heading two\n\n### Heading three\n\n"

// measureHeadings returns, for each heading headingsMarkdown writes, the look of every one drawn: its size, weight,
// spacing, case, style, frame and padding (line), and what drew it (at): the part it is in, "" in a template's own
// Markdown (data-floe-markdown), else its tag. The size of h1 and h2, which base.css gives in em, is read against the
// box around them, so that a page.css may size the text of a field; h3's is in rem there, read as it is.
const measureHeadings = `(() => JSON.stringify(['Heading one', 'Heading two', 'Heading three'].flatMap((t) =>
  [...document.querySelectorAll('h1, h2, h3, h4, h5, h6')].filter((h) => h.textContent.trim() === t).map((h) => {
    const s = getComputedStyle(h), size = parseFloat(s.fontSize), part = h.closest('[data-floe-part]');
    const ems = (v) => (parseFloat(v) / size).toFixed(3) + 'em';
    return { at: part ? part.dataset.floePart : h.closest('[data-floe-markdown]') ? '' : h.tagName.toLowerCase(),
      line: t + ': ' + [h.matches('h1, h2') ? (size / parseFloat(getComputedStyle(h.parentElement).fontSize)).toFixed(3) + 'em' : s.fontSize,
      s.fontWeight, s.letterSpacing === 'normal' ? 'normal' : ems(s.letterSpacing), s.textTransform, s.fontStyle,
      ...['borderTopWidth', 'borderRightWidth', 'borderBottomWidth', 'borderLeftWidth', 'paddingTop', 'paddingRight', 'paddingBottom',
        'paddingLeft'].map((p) => s[p])].join(' ') };
  }))))()`

// heading is a heading measureHeadings measured.
type heading struct{ At, Line string }

// headingsOn returns measureHeadings of the page open.
func headingsOn(t *testing.T, s *site) []heading {
	t.Helper()

	var got []heading
	if err := json.Unmarshal([]byte(s.b.eval(measureHeadings)), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

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

// textHeadings returns the look of each heading of headingsMarkdown in a text, in theme, as measureHeadings reads it.
func textHeadings(t *testing.T, s *site, theme string) []string {
	t.Helper()

	text := "template: report\nlang: en\ntitle: T\nsections:\n  - heading: S\n    parts:\n      - text: |\n" +
		"          " + strings.ReplaceAll(strings.TrimSuffix(headingsMarkdown, "\n"), "\n", "\n          ") + "\n          said\n"
	s.floe(t, "publish", s.write(t, "text-"+theme+".yaml", text+"theme: "+theme+"\n"), "--slug", "text-"+theme)
	s.b.navigate("/a/text-" + theme + "/?raw")
	got := headingsOn(t, s)
	lines := make([]string, 0, len(got))
	for _, h := range got {
		lines = append(lines, h.Line)
	}
	return lines
}

// headings publishes data, a page of template whose fields fields of format markdown each begin with headingsMarkdown,
// as slug, and measures each heading drawn against want, the same in a text: the page holds them once a field, in the
// order of the fields. It returns the headings it measured and each drawn otherwise at what drew it, the template for
// its own Markdown; one missing is at the template.
func headings(t *testing.T, s *site, template, slug, data string, fields int, want []string) measured {
	t.Helper()

	s.floe(t, "publish", s.write(t, slug+".yaml", data), "--slug", slug)
	s.b.navigate("/a/" + slug + "/?raw")
	got := headingsOn(t, s)
	wanted := make([]string, 0, len(want)*fields)
	for _, line := range want {
		for range fields {
			wanted = append(wanted, line)
		}
	}
	m := measured{seen: len(got)}
	for i := range max(len(got), len(wanted)) {
		g, w := heading{At: template, Line: "none"}, "none"
		if i < len(got) {
			g = got[i]
			if g.At == "" {
				g.At = template
			}
		}
		if i < len(wanted) {
			w = wanted[i]
		}
		if g.Line != w {
			m.amiss = append(m.amiss, amiss{Page: slug, At: g.At, Text: g.Line + ", in a text " + w})
		}
	}
	return m
}

const lookTemplateMarkdownFix = "name the template's own headings in page.css by where they stand (header h1, section > h2), " +
	"not by the element alone, which reaches the headings of its Markdown"

// lookTemplateMarkdown draws the example of each floe template with headings at the head of each of its own fields
// of format markdown, in every theme: each heading is drawn as the same Markdown is in a text, its size, weight,
// frame and padding, not as the template's own headings, its title's and its sections'. A rule of the template's
// page.css that names its headings by the element alone reaches the author's too.
// LIMIT: a field the example leaves out goes unmeasured, and the look but of headings (a rule of page.css that
// reaches a paragraph in a field arranges the template's own Markdown, as it may).
func lookTemplateMarkdown(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, templates := k.Names()
	want := map[string][]string{}
	for _, theme := range themes {
		want[theme] = textHeadings(t, s, theme)
	}
	var m measured
	for _, name := range templates {
		tpl, ok, err := k.Template(name)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("floe has no template %s, though it names it", name)
		}
		_, values, err := yamlpos.ParseMap(name+".yaml", tpl.Example())
		if err != nil {
			t.Fatal(err)
		}
		at := markdownFields(t, tpl.Schema(), values)
		for _, a := range at {
			prepend(t, values, a, headingsMarkdown)
		}
		for _, theme := range themes {
			if len(at) == 0 {
				break
			}
			values["theme"] = theme
			// JSON is YAML, written with no HTML escaped.
			var data bytes.Buffer
			enc := json.NewEncoder(&data)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(values); err != nil {
				t.Fatal(err)
			}
			m.add(headings(t, s, name, "md-"+name+"-"+theme, data.String(), len(at), want[theme]))
		}
	}
	return m
}

// brokenTemplateMarkdown draws a template of one's own whose one field is of format markdown and whose page.css spaces
// every h2, its Markdown's too.
func brokenTemplateMarkdown(t *testing.T, s *site) (string, measured) {
	t.Helper()

	want := textHeadings(t, s, "default")
	s.own(t, map[string]string{
		"templates/tm/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\"><title>{{.Title}}</title></head>\n" +
			"<body><main>{{.Data.intro}}</main></body></html>\n",
		"templates/tm/page.css": "h2 { letter-spacing: 3px; }\n",
		"templates/tm/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", ` +
			`"properties": {"intro": {"type": "string", "format": "markdown"}}}`,
		"templates/tm/example.yaml": "lang: en\ntemplate: tm\nintro: x\n",
	})
	data := "template: tm\nlang: en\ntitle: T\nintro: |\n  " + strings.ReplaceAll(strings.TrimSuffix(headingsMarkdown, "\n"), "\n", "\n  ") + "\n  said\n"
	m := headings(t, s, "tm", "broken", data, 1, want)
	// A heading not drawn is amiss at the template too: the page must draw them, and one drawn otherwise.
	if m.seen == 0 {
		t.Fatal("tm's page draws no heading to measure")
	}
	if !slices.ContainsFunc(m.amiss, func(a amiss) bool { return !strings.HasPrefix(a.Text, "none,") }) {
		t.Error("tm's page draws no heading otherwise than a text: the measure no longer sees letter-spacing")
	}
	return "tm", m
}
