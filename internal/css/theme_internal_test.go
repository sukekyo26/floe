package css

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/tdewolff/parse/v2/css"
)

// numbers are the counter styles a theme may write: digits, the same in every language.
var numbers = []string{"decimal", "decimal-leading-zero"}

// unescape reads a CSS string's text: its quotes off and each escape (\hex or \char) as its character.
func unescape(s string) string {
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && j < i+7 && strings.IndexByte("0123456789abcdefABCDEF", s[j]) >= 0 {
			j++
		}
		if j == i+1 {
			b.WriteByte(s[j])
			i = j
			continue
		}
		r, err := strconv.ParseUint(s[i+1:j], 16, 32)
		if err == nil {
			b.WriteRune(rune(r))
		}
		if j < len(s) && s[j] == ' ' {
			j++
		}
		i = j - 1
	}
	return b.String()
}

// TestThemeMarks keeps a theme's decorations out of words and tilts: the text a theme's CSS writes in content is
// marks alone (no letter, which is a word of a language the page may not be in), its counters digits, and no
// transform of a theme turns, skews or scales.
func TestThemeMarks(t *testing.T) {
	t.Parallel()

	themes := os.DirFS("../kit/floe/themes")
	found := 0
	err := fs.WalkDir(themes, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(themes, name)
		if err != nil {
			return err
		}
		for _, d := range declarations(src, func(p string) bool { return p == "content" })["content"] {
			found++
			for i, tok := range d.value {
				switch {
				case tok.tt == css.StringToken && strings.ContainsFunc(unescape(string(tok.data)), unicode.IsLetter):
					t.Errorf("%s: content %s writes a word; write a mark", name, text(src, d.value))
				case tok.tt == css.FunctionToken && strings.EqualFold(string(tok.data), "counter(") && i+3 < len(d.value) &&
					d.value[i+2].tt == css.CommaToken && !slices.Contains(numbers, string(d.value[i+3].data)):
					t.Errorf("%s: content %s counts in %s; count in %s", name, text(src, d.value), d.value[i+3].data,
						strings.Join(numbers, " or "))
				}
			}
		}
		for _, d := range declarations(src, func(p string) bool { return p == "transform" })["transform"] {
			if slices.ContainsFunc(d.value, func(tok token) bool {
				f := strings.ToLower(string(tok.data))
				return tok.tt == css.FunctionToken && (strings.HasPrefix(f, "rotate") || strings.HasPrefix(f, "skew") ||
					strings.HasPrefix(f, "scale") || strings.HasPrefix(f, "matrix"))
			}) {
				t.Errorf("%s: transform %s turns, skews or scales; a theme draws upright", name, text(src, d.value))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("no content found in floe's themes")
	}
}

func TestUnescape(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{`"QUOTE"`: "QUOTE", `"\6CE8"`: "注", `"\201C"`: "“", `"\6CE8 x"`: "注x", `'\"'`: `"`} {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%s) = %q, want %q", in, got, want)
		}
	}
}
