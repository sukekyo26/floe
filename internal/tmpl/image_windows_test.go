package tmpl_test

import (
	"strings"
	"testing"

	"floe/internal/tmpl"
)

// An image named with what Windows reads otherwise (a \ as a separator, a stream after :, a device) is no path floe
// reads under the data file's directory, though a file of that path is there.
func TestImageWindowsNames(t *testing.T) {
	t.Parallel()

	_, images := imagesIn(t, map[string][]byte{"sub/b.png": png, "a.png": png})
	src := `lang: en
template: report
sections:
  - parts:
      - image: { alt: one, file: 'sub\b.png' }
      - image: { alt: two, file: 'a.png:x' }
      - image: { alt: three, file: NUL.png }
`
	got := drawn(t, floeKit(t), src, tmpl.Options{Theme: "", Untitled: "x", Images: images, ImagesDir: ""})
	for _, name := range []string{`"sub\b.png" is not a path floe reads`, `"a.png:x" is not a path floe reads`} {
		if !strings.Contains(got, name) {
			t.Errorf("told\n%s\nwant %s", got, name)
		}
	}
	if strings.Contains(got, `"NUL.png" is not a path`) {
		t.Errorf("NUL.png, an ordinary name since Windows 11, refused:\n%s", got)
	}
}
