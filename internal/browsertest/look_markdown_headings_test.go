package browsertest_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"floe/internal/kit"
	"floe/internal/schema"
	"floe/internal/yamlpos"
)

// measureMarkdownHeadings returns {seen, amiss} of the headings headingsMarkdown writes, wherever Markdown stands: each
// is amiss when it is larger than the template's own heading before it (one in no part, text or template's Markdown),
// or not smaller than the heading of the level above it in the same Markdown.
const measureMarkdownHeadings = `(() => {
  const box = '[data-floe-part], [data-floe-text], [data-floe-markdown]';
  const texts = ['Heading one', 'Heading two', 'Heading three'];
  const all = [...document.querySelectorAll('h1, h2, h3, h4, h5, h6')].filter((h) => h.checkVisibility());
  const size = (h) => parseFloat(getComputedStyle(h).fontSize);
  const own = all.filter((h) => !h.closest(box));
  const amiss = [];
  let seen = 0;
  for (const h of all) {
    const i = texts.indexOf(h.textContent.trim());
    if (i < 0 || !h.closest(box)) continue;
    seen++;
    const at = h.tagName.toLowerCase() + ' ' + texts[i];
    const before = own.filter((o) => o.compareDocumentPosition(h) & Node.DOCUMENT_POSITION_FOLLOWING).pop();
    if (before && size(h) > size(before)) {
      amiss.push({ at, text: size(h) + 'px, larger than the template\'s ' + before.tagName.toLowerCase() + ' "' +
        before.textContent.trim().slice(0, 30) + '" (' + size(before) + 'px) before it' });
    }
    const above = i > 0 && all.find((o) => o.textContent.trim() === texts[i - 1] && o.closest(box) === h.closest(box));
    if (above && size(h) >= size(above)) {
      amiss.push({ at, text: size(h) + 'px, not smaller than ' + texts[i - 1] + ' (' + size(above) + 'px) above it' });
    }
  }
  return JSON.stringify({ seen, amiss });
})()`

const lookMarkdownHeadingsFix = "size the headings of Markdown in base.css alone, in em: no larger than the template's heading " +
	"before them, and smaller by level"

// lookMarkdownHeadings draws the example of each floe template, in every theme, with headingsMarkdown at the head of
// each of its texts and fields of format markdown, its parts' too: each heading is no larger than the template's own
// heading before it, a title's, a section's or a step's, and smaller than the level above it.
// LIMIT: a place of Markdown the example leaves out goes unmeasured, a whole template when it holds none (handoff's)
func lookMarkdownHeadings(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, templates := k.Names()
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
		at := markdownPlaces(t, tpl.Schema(), values)
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
			slug := "mdh-" + name + "-" + theme
			s.floe(t, "publish", s.write(t, slug+".yaml", data.String()), "--slug", slug)
			s.b.navigate("/a/" + slug + "/?raw")
			m.add(s.b.measure(slug, measureMarkdownHeadings))
		}
	}
	return m
}

// markdownPlaces returns where values hold Markdown, as the schema's Walk finds it: every field of format markdown, a
// text's and a part's too.
func markdownPlaces(t *testing.T, c *schema.Compiled, values map[string]any) [][]string {
	t.Helper()

	var at [][]string
	if _, err := c.Walk(values, func(it schema.Item, ready any) (any, error) {
		if _, written := it.Value.(string); written && (it.Part == kit.TextPart || it.Format == "markdown") && !slices.ContainsFunc(at, func(a []string) bool { return slices.Equal(a, it.At) }) {
			at = append(at, it.At)
		}
		return ready, nil
	}); err != nil {
		t.Fatal(err)
	}
	return at
}

// brokenMarkdownHeadings draws a text in a theme of one's own whose markdown.css makes Markdown's h1 larger than the
// section's heading.
func brokenMarkdownHeadings(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/loud/markdown.css": "h1 { font-size: 3em; }\n"},
		themePage("loud", "- text: |\n    # Heading one\n\n    ## Heading two\n\n    ### Heading three\n"))
	return "h1 Heading one", s.b.measure("broken", measureMarkdownHeadings)
}
