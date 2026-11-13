// Package lint holds the rules of floe's CSS that are read in its text (.claude/rules/css.md, and ui.md for the
// management page), each a Rule of Rules, named as its line of the rules names it. It reads CSS by internal/css's Tokens
// alone (read.go) and is imported by no product code: TestLintRule runs each rule on floe's own files, on a good example
// and on a bad one (testdata/<name>).
package lint

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"floe/internal/kit"
)

// Rule is a rule of floe's CSS, read in its text.
type Rule struct {
	// Name names the rule, as its line in .claude/rules names it ([name]).
	Name string
	// Check returns what in reads amiss of the rule.
	Check func(in *Input) []Finding
	// Allow are the rule's exceptions on floe's own files; what a Key names is the rule's to say.
	Allow []Allow
}

// Allow is an exception of a rule: a key the rule looks up by Input.Allowed, and why it is one.
type Allow struct{ Key, Reason string }

// Finding is what a rule finds amiss: the file and line (0 for none) it is at, the rule, and what is amiss and how to set
// it right.
type Finding struct {
	File string
	Line int
	Rule string
	Text string
}

// String writes f as a linter does: file:line: rule: text.
func (f Finding) String() string {
	if f.Line == 0 {
		return fmt.Sprintf("%s: %s: %s", f.File, f.Rule, f.Text)
	}
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Text)
}

// Kinds of sheet.
const (
	kindBase     = "base"     // floe's base.css
	kindPart     = "part"     // a part's part.css
	kindLook     = "look"     // a look's part.css
	kindTheme    = "theme"    // a theme's file
	kindTemplate = "template" // a template's page.css
)

// Sheet is a CSS file a rule reads: its kind, the part, look, theme or template it is of (Owner), for a theme's file the
// part or look it is for or palette, base or markdown (For), its path and its text.
type Sheet struct {
	Kind, Owner, For, File string
	Src                    []byte
}

// at returns the place of the byte at in s, for a finding.
func (s Sheet) at(at int) (string, int) { return s.File, lineOf(s.Src, at) }

// Input is all a rule reads, by its methods alone, so that what it reads has one source: floe's own files, or a
// testdata example's.
type Input struct {
	kit   *kit.Kit
	base  []byte            // floe's base.css
	docs  fs.FS             // floe's docs
	pages fs.FS             // pages floe drew, the HTML (every-part)
	ui    []byte            // the management page, ui.html
	allow map[string]string // the exceptions in force: a key and its reason
	used  map[string]bool   // the keys of allow looked up and found
	seen  int               // what the rule looked at (Saw)
}

// Saw notes that the rule looked at one more thing it checks (a declaration, a file): TestLintRule fails a rule that
// looks at nothing on floe's files or its good example, as when what it checks is renamed or gone.
func (in *Input) Saw() { in.seen++ }

// Sheets returns every CSS file of in: base.css, then each part's and look's, each theme's and each template's, in the
// order of their names.
func (in *Input) Sheets() []Sheet {
	var out []Sheet
	if in.base != nil {
		out = append(out, Sheet{Kind: kindBase, File: "base.css", Src: in.base})
	}
	if in.kit == nil {
		return out
	}
	parts, looks, themes, templates := in.kit.Names()
	for _, name := range parts {
		p, _ := in.kit.Part(name)
		if src := p.CSS(); src != nil {
			out = append(out, Sheet{Kind: kindPart, Owner: name, File: "parts/" + name + "/part.css", Src: src})
		}
	}
	for _, name := range looks {
		l, _ := in.kit.Look(name)
		if src := l.CSS(); src != nil {
			out = append(out, Sheet{Kind: kindLook, Owner: name, File: "parts/" + name + "/part.css", Src: src})
		}
	}
	named := slices.Sorted(slices.Values(slices.Concat(parts, looks)))
	for _, name := range themes {
		t, _ := in.kit.Theme(name)
		files := []struct {
			name string
			src  []byte
		}{{"base", t.Base()}, {"markdown", t.Markdown()}, {"palette", t.Palette()}}
		for _, of := range named {
			files = append(files, struct {
				name string
				src  []byte
			}{of, t.CSS(of)})
		}
		for _, f := range files {
			if f.src != nil {
				out = append(out, Sheet{Kind: kindTheme, Owner: name, For: f.name, File: "themes/" + name + "/" + f.name + ".css", Src: f.src})
			}
		}
	}
	for _, name := range templates {
		tm, _, err := in.kit.Template(name)
		if err != nil {
			panic(err)
		}
		if src := tm.CSS(); src != nil {
			out = append(out, Sheet{Kind: kindTemplate, Owner: name, File: "templates/" + name + "/page.css", Src: src})
		}
	}
	return out
}

// Names returns the names of the parts and the looks of in.
func (in *Input) Names() (parts, looks []string) {
	if in.kit == nil {
		return nil, nil
	}
	parts, looks, _, _ = in.kit.Names()
	return parts, looks
}

// Taken returns the looks the part or look name takes, their looks too.
func (in *Input) Taken(name string) []string {
	if in.kit == nil {
		return nil
	}
	return in.kit.Taken(name)
}

// Docs returns floe's docs.
func (in *Input) Docs() fs.FS { return in.docs }

// Pages returns the pages floe drew.
func (in *Input) Pages() fs.FS { return in.pages }

// UI returns the management page.
func (in *Input) UI() []byte { return in.ui }

// Exceptions returns the keys of the exceptions in force, in order.
func (in *Input) Exceptions() []string { return slices.Sorted(maps.Keys(in.allow)) }

// Allowed reports whether key is an exception in force, and notes it used.
func (in *Input) Allowed(key string) bool {
	if in.allow[key] == "" {
		return false
	}
	if in.used == nil {
		in.used = map[string]bool{}
	}
	in.used[key] = true
	return true
}

// Rules are floe's CSS rules read in its text, in the order of their names.
var Rules = []Rule{
	codeLook,
	manageColours,
	manageCornersRule,
	opacityRule,
	paletteInDocs,
	partVariables,
	partsInDocs,
	radiusRule,
	rawInDocs,
	speechCSS,
	textCase,
	textScale,
	themeMarks,
}
