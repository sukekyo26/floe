package i18n

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// The tests share the package-level catalogs, which withCatalogs swaps, so none of them runs in parallel.

// withCatalogs replaces the registered catalogs for the duration of a test.
func withCatalogs(t *testing.T, tables map[Lang]map[string]string) {
	t.Helper()

	saved := catalogs
	catalogs = map[Lang]map[string]string{}
	for lang, table := range tables {
		register(lang, table)
	}
	t.Cleanup(func() { catalogs = saved })
}

func testCatalogs(t *testing.T) {
	t.Helper()

	withCatalogs(t, map[Lang]map[string]string{
		English: {"hello": "hello %s", "only_en": "english only", "pair": "%s then %d", "percent": "100%%"},
		"ja":    {"hello": "こんにちは %s", "pair": "%[2]d の前に %[1]s"},
		"pt":    {"hello": "olá %s"},
		"pt-br": {"only_en": "brasil"},
	})
}

//nolint:paralleltest // swaps the global catalogs
func TestCatalogMsg(t *testing.T) {
	testCatalogs(t)
	tests := []struct {
		lang      Lang
		key, want string
		args      []any
	}{
		{"ja", "hello", "こんにちは Ann", []any{"Ann"}},
		{"ja", "pair", "2 の前に one", []any{"one", 2}},
		{"ja", "only_en", "english only", nil},
		{"pt-br", "hello", "olá Ann", []any{"Ann"}},
		{"pt-br", "only_en", "brasil", nil},
		{"fr", "hello", "hello Ann", []any{"Ann"}},
		{English, "missing_key", "missing_key", nil},
		{English, "percent", "100%", nil},
	}
	for _, tt := range tests {
		c := New(tt.lang)
		if got := c.Msg(tt.key, tt.args...); got != tt.want {
			t.Errorf("New(%q).Msg(%q) = %q, want %q", tt.lang, tt.key, got, tt.want)
		}
	}
	if got := New("en-us").chain; !slices.Equal(got, []Lang{"en-us", English}) {
		t.Errorf("the chain of en-us = %v, want English once", got)
	}
	if EnglishCatalog().Msg("percent") != "100%" {
		t.Error("EnglishCatalog is not English")
	}
}

var (
	verbRe = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)
	// starRe finds a width or precision taken from an argument (%*s, %.*f), which shifts the
	// arguments after it. Catalogs do not use it, so verbs need not count it.
	starRe = regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*(?:\*|\d*\.\*)`)
)

// verbs maps each argument a message formats, by its position, to the verbs it uses (sorted; an
// argument formatted twice, %[1]s and %[1]q, keeps both).
func verbs(tmpl string) map[int][]string {
	used := map[int][]string{}
	next := 1
	for _, m := range verbRe.FindAllStringSubmatch(tmpl, -1) {
		if m[2] == "%" {
			continue
		}
		if m[1] != "" {
			next, _ = strconv.Atoi(m[1]) //nolint:errcheck // the pattern holds digits only
		}
		if !slices.Contains(used[next], m[2]) {
			used[next] = append(used[next], m[2])
			slices.Sort(used[next])
		}
		next++
	}
	return used
}

func sameVerbs(a, b map[int][]string) bool {
	return maps.EqualFunc(a, b, slices.Equal[[]string])
}

func TestVerbs(t *testing.T) {
	t.Parallel()

	tests := map[string]map[int][]string{
		"plain":                  {},
		"%s and %d":              {1: {"s"}, 2: {"d"}},
		"%[2]d の前に %[1]s":        {1: {"s"}, 2: {"d"}},
		"100%% of %q":            {1: {"q"}},
		"%-8s|%5.2f":             {1: {"s"}, 2: {"f"}},
		"%[1]s, again %[1]s":     {1: {"s"}},
		"%[1]s, quoted %[1]q":    {1: {"q", "s"}},
		"%[2]s then the next %s": {2: {"s"}, 3: {"s"}},
	}
	for tmpl, want := range tests {
		if got := verbs(tmpl); !sameVerbs(got, want) {
			t.Errorf("verbs(%q) = %v, want %v", tmpl, got, want)
		}
	}
}

// mismatches lists what keeps the catalogs from matching English: a missing or extra key, a message
// that formats other arguments or verbs than its English one, or a * width or precision in any
// language, which verbs would not count.
func mismatches(tables map[Lang]map[string]string) []string {
	var found []string
	for lang, table := range tables {
		for key, msg := range table {
			if starRe.MatchString(msg) {
				found = append(found, string(lang)+" uses * in "+key)
			}
		}
	}
	en := tables[English]
	for lang, table := range tables {
		if lang == English {
			continue
		}
		for key := range en {
			_, has := table[key]
			if !has {
				found = append(found, string(lang)+" lacks "+key)
			}
		}
		for key, msg := range table {
			ref, ok := en[key]
			switch {
			case !ok:
				found = append(found, string(lang)+" has extra "+key)
			case !sameVerbs(verbs(msg), verbs(ref)):
				found = append(found, string(lang)+" formats "+key+" differently")
			}
		}
	}
	slices.Sort(found)
	return found
}

func TestMismatches(t *testing.T) {
	t.Parallel()

	got := mismatches(map[Lang]map[string]string{
		English: {"a": "%s", "b": "%s %d", "c": "plain", "d": "%[1]s %[1]q", "e": "%*d", "f": "%d%%"},
		"ja":    {"a": "%d", "b": "%[2]d %[1]s", "d": "%[1]s %[1]s", "e": "%*d", "f": "%.*f", "x": "extra"},
	})
	want := []string{
		"en uses * in e", "ja formats a differently", "ja formats d differently", "ja formats f differently",
		"ja has extra x", "ja lacks c", "ja uses * in e", "ja uses * in f",
	}
	if !slices.Equal(got, want) {
		t.Errorf("mismatches = %q, want %q", got, want)
	}
}

// TestCatalogsMatchEnglish guards the real catalogs: every language has exactly the English keys, and each message formats the same arguments with the same verbs, so no
// translation drops or misplaces a value.
//
//nolint:paralleltest // reads the global catalogs, which other tests swap
func TestCatalogsMatchEnglish(t *testing.T) {
	for _, m := range mismatches(catalogs) {
		t.Error(m)
	}
	if _, ok := catalogs["ja"]; !ok {
		t.Error("the Japanese catalog is not registered")
	}
}
