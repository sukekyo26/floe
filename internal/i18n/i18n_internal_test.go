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
func TestMatch(t *testing.T) {
	testCatalogs(t)
	tests := map[string]Lang{
		"ja": "ja", "JA": "ja", "ja_JP.UTF-8": "ja", "ja-JP": "ja", "en_US.UTF-8": English,
		"pt-BR": "pt-br", "pt_PT": "pt", "sr@latin": "", "C": "", "POSIX": "", "fr": "", "": "",
	}
	for tag, want := range tests {
		got, ok := Match(tag)
		if got != want || ok != (want != "") {
			t.Errorf("Match(%q) = %q, %v; want %q", tag, got, ok, want)
		}
	}
	if got := Supported(); !slices.Equal(got, []Lang{English, "ja", "pt", "pt-br"}) {
		t.Errorf("Supported() = %v", got)
	}
}

//nolint:paralleltest // swaps the global catalogs
func TestDetect(t *testing.T) {
	testCatalogs(t)
	tests := []struct {
		env  map[string]string
		want Lang
	}{
		{map[string]string{}, English},
		{map[string]string{"LANG": "ja_JP.UTF-8"}, "ja"},
		{map[string]string{"LANG": "ja_JP.UTF-8", "LC_MESSAGES": "en_US.UTF-8"}, English},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_ALL": "ja_JP.UTF-8"}, "ja"},
		{map[string]string{"LC_ALL": "ja_JP.UTF-8", "FLOE_LANG": "en"}, English},
		{map[string]string{"FLOE_LANG": "fr", "LANG": "ja_JP.UTF-8"}, "ja"},
		{map[string]string{"LC_ALL": "C", "LANG": "ja_JP.UTF-8"}, "ja"},
	}
	for _, tt := range tests {
		if got := Detect(func(k string) string { return tt.env[k] }); got != tt.want {
			t.Errorf("Detect(%v) = %q, want %q", tt.env, got, tt.want)
		}
	}
}

//nolint:paralleltest // swaps the global catalogs
func TestFromAcceptLanguage(t *testing.T) {
	testCatalogs(t)
	tests := map[string]Lang{
		"":                                 English,
		"ja":                               "ja",
		"ja-JP,ja;q=0.9,en-US;q=0.8":       "ja",
		"fr-FR,fr;q=0.9,ja;q=0.8,en;q=0.7": "ja",
		"en;q=0.5, ja;q=0.9":               "ja",
		"ja;q=0, en":                       English,
		"*, ja;q=0.1":                      "ja",
		"pt-BR, ja":                        "pt-br",
		"ja;q=bad":                         "ja",
		"de, fr":                           English,
	}
	for header, want := range tests {
		if got := FromAcceptLanguage(header); got != want {
			t.Errorf("FromAcceptLanguage(%q) = %q, want %q", header, got, want)
		}
	}
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
		if got := c.Msg(tt.key, tt.args...); got != tt.want || c.Lang() != tt.lang {
			t.Errorf("New(%q).Msg(%q) = %q, want %q", tt.lang, tt.key, got, tt.want)
		}
	}
	if EnglishCatalog().Lang() != English {
		t.Error("EnglishCatalog is not English")
	}
}

var verbRe = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)

// verbs maps each argument a message formats, by its position, to the verb it uses.
func verbs(tmpl string) map[int]string {
	used := map[int]string{}
	next := 1
	for _, m := range verbRe.FindAllStringSubmatch(tmpl, -1) {
		if m[2] == "%" {
			continue
		}
		if m[1] != "" {
			next, _ = strconv.Atoi(m[1]) //nolint:errcheck // the pattern holds digits only
		}
		used[next] = m[2]
		next++
	}
	return used
}

func TestVerbs(t *testing.T) {
	t.Parallel()

	tests := map[string]map[int]string{
		"plain":                  {},
		"%s and %d":              {1: "s", 2: "d"},
		"%[2]d の前に %[1]s":        {1: "s", 2: "d"},
		"100%% of %q":            {1: "q"},
		"%-8s|%5.2f":             {1: "s", 2: "f"},
		"%[1]s, again %[1]s":     {1: "s"},
		"%[2]s then the next %s": {2: "s", 3: "s"},
	}
	for tmpl, want := range tests {
		if got := verbs(tmpl); !maps.Equal(got, want) {
			t.Errorf("verbs(%q) = %v, want %v", tmpl, got, want)
		}
	}
}

// mismatches lists what keeps the catalogs from matching English: a missing or extra key, or a message
// that formats other arguments or verbs than its English one.
func mismatches(tables map[Lang]map[string]string) []string {
	var found []string
	en := tables[English]
	for lang, table := range tables {
		if lang == English {
			continue
		}
		for key := range en {
			if _, ok := table[key]; !ok {
				found = append(found, string(lang)+" lacks "+key)
			}
		}
		for key, msg := range table {
			ref, ok := en[key]
			switch {
			case !ok:
				found = append(found, string(lang)+" has extra "+key)
			case !maps.Equal(verbs(msg), verbs(ref)):
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
		English: {"a": "%s", "b": "%s %d", "c": "plain"},
		"ja":    {"a": "%d", "b": "%[2]d %[1]s", "x": "extra"},
	})
	want := []string{"ja formats a differently", "ja has extra x", "ja lacks c"}
	if !slices.Equal(got, want) {
		t.Errorf("mismatches = %q, want %q", got, want)
	}
}

// TestCatalogsMatchEnglish guards the real catalogs: every language has exactly the English keys,
// and each message formats the same arguments with the same verbs, so no translation drops or
// misplaces a value.
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
