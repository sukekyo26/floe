package tmpl

import (
	"os"
	"testing"
	"testing/fstest"
)

// cwd is the package's directory, the tests' working directory: what a relative file of theirs is under.
var cwd = func() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}()

// Each YAML file a template or part gives tells what reading it refuses and a top of the wrong shape in
// the same run.
func TestReadTop(t *testing.T) {
	t.Parallel()

	const (
		key    = "a key is a name: write 1 as text, in quotes"
		anchor = "anchors, aliases, merge keys (<<) and tags are not supported; write the value out"
	)
	part := func(file string, src []byte) error {
		_, err := readPartYAML(fstest.MapFS{"part.yaml": {Data: src}}, file, false)
		return err
	}
	for _, tt := range []struct {
		file, src, want string
		read            func(string, []byte) error
	}{
		{"checks.yaml", "1: a\n", "checks.yaml:1:1: " + key + "\nchecks.yaml:1:1: write checks.yaml as a list of checks, each with expr and message (and for and field if you like)", func(f string, s []byte) error { _, err := readChecks(f, s); return err }},
		// A tag that holds nothing is told alone: removed, it leaves an empty file, which checks.yaml may be.
		{"checks.yaml", "!!seq\n", "checks.yaml:1:1: " + anchor, func(f string, s []byte) error { _, err := readChecks(f, s); return err }},
		{"computed.yaml", "- 1: a\n", "computed.yaml:1:1: write computed.yaml as names and expressions, like total: sum(items, .price)\ncomputed.yaml:1:3: " + key, func(f string, s []byte) error { _, _, err := readExprs(f, s); return err }},
		{"template.yaml", "- 1: a\n", "template.yaml:1:1: write template.yaml as one key, floe:, and the version of floe the template needs, like floe: 1.2\ntemplate.yaml:1:3: " + key, func(f string, s []byte) error { _, err := needsFloe(f, s); return err }},
		{"part.yaml", "- 1: a\n", "part.yaml:1:1: write part.yaml as keys and values: looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:\npart.yaml:1:3: " + key, part},
		{"words.yaml", "- 1: a\n", "words.yaml:1:1: write words.yaml as language tags, each the names of the words and their text\nwords.yaml:1:3: " + key, func(f string, s []byte) error { _, err := readWords(f, s); return err }},
	} {
		// The same each run, though both are told at 1:1 of checks.yaml.
		for range 5 {
			if err := tt.read(tt.file, []byte(tt.src)); err == nil || err.Error() != tt.want {
				t.Errorf("reading %s %q = %v\nwant %s", tt.file, tt.src, err, tt.want)
				break
			}
		}
	}
}
