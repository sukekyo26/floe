// Package i18n renders the messages floe shows to people in their language. Catalogs are Go maps,
// one file per language (messages_<lang>.go), registered at init; a language is added by adding a file.
// Errors carry a key and arguments and are rendered at the boundary: the CLI's exit, in English.
//
// A key missing from a language falls back to the language without its last subtag, one at a time
// (zh-Hant-TW to zh-Hant to zh, pt-BR to pt), then to English, then to the key itself.
package i18n

import (
	"fmt"
	"slices"
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

// parent is tag without its last subtag, "" for one of none: zh-hant for zh-hant-tw.
func parent(tag string) string {
	i := strings.LastIndex(tag, "-")
	if i < 0 {
		return ""
	}
	return tag[:i]
}

// Catalog renders messages in one language.
type Catalog struct {
	chain []Lang
}

// New returns the catalog of lang. A language without a catalog renders in English.
func New(lang Lang) *Catalog {
	return &Catalog{chain: fallbacks(lang)}
}

// fallbacks lists where to look for a text in lang, in order: lang, lang without its last subtag, one at a
// time, then English.
func fallbacks(lang Lang) []Lang {
	var chain []Lang
	for tag := string(lang); tag != ""; tag = parent(tag) {
		chain = append(chain, Lang(tag))
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

// Msg renders the message key with args (fmt verbs; %[n]s when a language reorders them). Every
// message goes through fmt, with or without args, so a literal % is %% in every message.
func (c *Catalog) Msg(key string, args ...any) string {
	return fmt.Sprintf(c.lookup(key), args...)
}

func (c *Catalog) lookup(key string) string {
	for _, lang := range c.chain {
		if v, ok := catalogs[lang][key]; ok {
			return v
		}
	}
	return key
}
