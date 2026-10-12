// Package i18n renders the messages floe shows to people in their language. Catalogs are Go maps,
// one file per language (messages_<lang>.go), registered at init; a language is added by adding a
// file. Errors carry a key and arguments and are rendered at the boundary (the CLI's exit, an HTTP
// response), so the same error reads in English in logs and in the reader's language on screen.
//
// A key missing from a language falls back to the language without its region (pt-BR to pt), then
// to English, then to the key itself.
package i18n

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Lang is a language tag in lower case (en, ja, pt-br).
type Lang string

// English is the default and the fallback of every catalog.
const English Lang = "en"

// catalogs maps a language to its messages. It is written only by register, at init.
var catalogs = map[Lang]map[string]string{}

func register(lang Lang, messages map[string]string) {
	table := catalogs[lang]
	if table == nil {
		table = map[string]string{}
		catalogs[lang] = table
	}
	for k, v := range messages {
		table[k] = v
	}
}

// Supported lists the registered languages, English first.
func Supported() []Lang {
	langs := make([]Lang, 0, len(catalogs))
	for lang := range catalogs {
		if lang != English {
			langs = append(langs, lang)
		}
	}
	slices.Sort(langs)
	return append([]Lang{English}, langs...)
}

// tagRe is a BCP 47 language tag: a language of 2 or 3 letters, then subtags such as a region.
var tagRe = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{1,8})*$`)

// IsTag reports whether s is written as a BCP 47 language tag (en, ja, pt-BR).
func IsTag(s string) bool {
	return tagRe.MatchString(s)
}

// Match returns the registered language for a tag such as ja, ja_JP.UTF-8 or pt-BR: the tag itself,
// else its language without the region. It reports false for a tag no catalog covers (C, POSIX, fr).
func Match(tag string) (Lang, bool) {
	tag = strings.ToLower(tag)
	if i := strings.IndexAny(tag, ".@"); i >= 0 {
		tag = tag[:i] // ja_JP.UTF-8, sr_RS@latin
	}
	tag = strings.ReplaceAll(strings.TrimSpace(tag), "_", "-")
	if _, ok := catalogs[Lang(tag)]; ok && tag != "" {
		return Lang(tag), true
	}
	if base, _, found := strings.Cut(tag, "-"); found {
		if _, ok := catalogs[Lang(base)]; ok {
			return Lang(base), true
		}
	}
	return "", false
}

// Detect picks the language of the CLI from the environment: FLOE_LANG, LC_ALL, LC_MESSAGES, then
// LANG. A variable naming a language without a catalog is skipped, so FLOE_LANG=fr with LANG=ja_JP
// gives Japanese; nothing matching gives English.
func Detect(getenv func(string) string) Lang {
	for _, name := range []string{"FLOE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang, ok := Match(getenv(name)); ok {
			return lang
		}
	}
	return English
}

// FromAcceptLanguage picks the language of an HTTP response from an Accept-Language header: the
// supported language with the highest weight (ties keep the header's order), else English.
func FromAcceptLanguage(header string) Lang {
	type choice struct {
		tag string
		q   float64
	}
	var choices []choice
	for _, part := range strings.Split(header, ",") {
		params := strings.Split(part, ";")
		tag := strings.TrimSpace(params[0])
		q := 1.0
		for _, p := range params[1:] {
			name, v, _ := strings.Cut(strings.TrimSpace(p), "=")
			// A weight is 0 to 1; anything else (2, NaN) is malformed and keeps the default.
			parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if strings.EqualFold(name, "q") && err == nil && parsed >= 0 && parsed <= 1 {
				q = parsed
			}
		}
		if tag != "" && tag != "*" && q > 0 {
			choices = append(choices, choice{tag: tag, q: q})
		}
	}
	slices.SortStableFunc(choices, func(a, b choice) int {
		switch {
		case a.q > b.q:
			return -1
		case a.q < b.q:
			return 1
		default:
			return 0
		}
	})
	for _, c := range choices {
		if lang, ok := Match(c.tag); ok {
			return lang
		}
	}
	return English
}

// Localizer is implemented by errors that can render themselves in a language. A boundary renders
// the first one in an error's chain and falls back to Error() for the rest.
type Localizer interface {
	Localize(c *Catalog) string
}

// Catalog renders messages in one language.
type Catalog struct {
	lang  Lang
	chain []Lang
}

// New returns the catalog of lang. A language without a catalog renders in English.
func New(lang Lang) *Catalog {
	return &Catalog{lang: lang, chain: Fallbacks(lang)}
}

// Fallbacks lists where to look for a text in lang, in order: lang, lang without its region, English.
func Fallbacks(lang Lang) []Lang {
	chain := []Lang{lang}
	if base, _, found := strings.Cut(string(lang), "-"); found {
		chain = append(chain, Lang(base))
	}
	if !slices.Contains(chain, English) {
		chain = append(chain, English)
	}
	return chain
}

var english = New(English)

// EnglishCatalog returns the English catalog, which Error() methods render with.
func EnglishCatalog() *Catalog {
	return english
}

// Lang reports the catalog's language.
func (c *Catalog) Lang() Lang {
	return c.lang
}

// Msg renders the message key with args (fmt verbs; %[n]s when a language reorders them). Every
// message goes through fmt, with or without args, so a literal % is %% in every message.
func (c *Catalog) Msg(key string, args ...any) string {
	return fmt.Sprintf(c.lookup(key), args...)
}

// Formats returns the unrendered messages whose keys start with prefix, with the same fallback as
// Msg, for a page that renders them itself.
func (c *Catalog) Formats(prefix string) map[string]string {
	formats := map[string]string{}
	for key := range catalogs[English] {
		if strings.HasPrefix(key, prefix) {
			formats[key] = c.lookup(key)
		}
	}
	return formats
}

func (c *Catalog) lookup(key string) string {
	for _, lang := range c.chain {
		if v, ok := catalogs[lang][key]; ok {
			return v
		}
	}
	return key
}
