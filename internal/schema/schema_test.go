package schema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

const template = "floe://templates/t/schema.json"

// resources are small parts and shared schemas, as floe's are given: note (title, text, a tone), tone (shared,
// an enum), any (shared: a markdown text, a note, a stack), stack (a part holding any), html (a part with a
// format html).
func resources(t *testing.T) map[string]schema.Resource {
	t.Helper()

	res := map[string]schema.Resource{}
	for name, k := range map[string]schema.Kind{"note": schema.Part, "stack": schema.Part, "html": schema.Part, "tone": schema.Shared, "any": schema.Shared} {
		src, err := os.ReadFile(filepath.Join("testdata", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		r, err := schema.NewResource("floe:///parts/"+name+".json", "parts/"+name+".json", src, k)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/"+name+".json"] = r
	}
	return res
}

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

const head = `"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"`

func compile(t *testing.T, body string) (*schema.Compiled, error) {
	t.Helper()

	return schema.Compile(template, "schema.json", []byte("{"+head+", "+body+"}"), resources(t))
}

func schemaCompile(t *testing.T, src string) (*schema.Compiled, error) {
	t.Helper()

	return schema.Compile(template, "schema.json", []byte(src), resources(t))
}

func lines(s ...string) string { return strings.Join(s, "\n") }

// checkPart compiles the part at url of res as a root and checks it, as kit does.
func checkPart(url string, res map[string]schema.Resource) error {
	c, err := schema.CompilePart(url, res)
	if err != nil {
		return err //nolint:wrapcheck // its mistakes, as the tests compare them
	}
	return c.CheckPart() //nolint:wrapcheck // as above
}
