package kit

import "fmt"

// The files of the smallest part and template Load takes, but those naming them, to fill in. A template's page
// keeps its language and title, without which a page reads as broken, and a viewport, without which a phone draws
// the page opened by itself 980px wide and shrinks it. No charset: floe adds it as it draws the page.
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

// Skeleton returns the files of a part (KindPart) or a template of one's own named name, as small as Load
// takes, by their names in its directory. name goes in as written in the part's box and the example's
// template:, quoted, so that YAML reads a name like null or 017 as text; validName keeps it to characters
// neither HTML nor YAML escapes.
func Skeleton(kind Kind, name string) map[string][]byte {
	if kind == KindTemplate {
		return map[string][]byte{
			"page.html":   []byte(skeletonPage),
			"schema.json": []byte(skeletonSchema),
			// The title is not the name, which YAML might read as other than text (Null).
			"example.yaml": fmt.Appendf(nil, "template: \"%s\"\nlang: en\ntitle: Example\n", name),
		}
	}
	return map[string][]byte{
		"schema.json": []byte(skeletonSchema),
		"part.html":   fmt.Appendf(nil, "<div data-floe-part=\"%s\"></div>\n", name),
		"part.css":    {},
	}
}
