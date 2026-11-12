package kit

import "fmt"

// The files of the smallest part and template Load takes, but those naming them, to fill in. A template's page
// keeps its language and title, without which a page reads as broken, and a viewport, without which a phone draws
// the page opened by itself 980px wide and shrinks it. No charset: floe gives it.
const (
	skeletonSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object"
}
`
	skeletonPage = `<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
</head>
<body>
</body>
</html>
`
)

// The skeletons are the files of the smallest part, theme and template of one's own that Load takes, to fill in, by
// their names in its directory. name goes in as written in the part's box and the example's template:, quoted
// ('...'), so that YAML reads a name like null or 017 as text; validName keeps it to characters neither HTML nor
// YAML escapes.

// PartSkeleton returns the files of a part named name: its schema, its box and an empty part.css.
func PartSkeleton(name string) map[string][]byte {
	return map[string][]byte{
		"schema.json": []byte(skeletonSchema),
		"part.html":   fmt.Appendf(nil, "<div data-floe-part=\"%s\"></div>\n", name),
		"part.css":    {},
	}
}

// ThemeSkeleton returns the files of a theme: a theme needs none, and an empty palette.css is where its colours go.
func ThemeSkeleton() map[string][]byte {
	return map[string][]byte{"palette.css": {}}
}

// TemplateSkeleton returns the files of a template named name: its page, an empty schema and an example.
func TemplateSkeleton(name string) map[string][]byte {
	return map[string][]byte{
		"page.html":   []byte(skeletonPage),
		"schema.json": []byte(skeletonSchema),
		// The title is not the name, which YAML might read as other than text (Null).
		"example.yaml": fmt.Appendf(nil, "template: '%s'\nlang: en\ntitle: Example\n", name),
	}
}
