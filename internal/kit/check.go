package kit

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/cycle"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// check checks what k read itself, not its base: the rules over the values read made. A part holds
// schema.json, part.html and part.css, and a look part.css alone; their CSS and a theme's are what floe can
// put in a layer (css); a part's schema is what a part may hold (schema.CheckPart); page.html and part.html
// escape (escapeMistake); the looks they take are parts or looks, and none takes its own; a template holds
// page.html and schema.json. Every mistake is told together, Sorted.
func check(k *Kit) error {
	var found usererr.List
	add := func(err error) error { return found.Add(err) }
	for _, name := range slices.Sorted(maps.Keys(k.parts)) {
		if err := add(checkPart(k.parts[name])); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(k.looks)) {
		l := k.looks[name]
		if !l.hasCSS {
			found = append(found, usererr.NewAt(l.dir, 0, 0, "kit_part_files"))
		}
		found = append(found, unread(l.dir, l.unread, "kit_look_unread")...)
		if err := add(css.Check(path.Join(l.dir, "part.css"), l.css)); err != nil {
			return err
		}
	}
	found = append(found, k.checkLooks()...)
	for _, name := range slices.Sorted(maps.Keys(k.themes)) {
		if err := add(checkTheme(k.themes[name])); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(k.templates)) {
		if err := add(checkTemplate(k.templates[name])); err != nil {
			return err
		}
	}
	if k.baseCSS != nil {
		if err := add(css.Check("base.css", k.baseCSS)); err != nil {
			return err
		}
	}
	return found.Sorted().Err()
}

// checkPart checks the files, the CSS, the schema, the checks' for and the escaping of p.
func checkPart(p *Part) error {
	var found usererr.List
	if p.pages == nil || !p.hasCSS {
		found = append(found, usererr.NewAt(p.dir, 0, 0, "kit_part_files"))
	}
	found = append(found, unread(p.dir, p.unread, "kit_part_unread")...)
	found = append(found, forMistakes(p.checks, p.schema)...)
	errs := []error{css.Check(path.Join(p.dir, "part.css"), p.css), p.schema.CheckPart()}
	if p.pages != nil {
		errs = append(errs, escapeMistake(p.pages[""], p.pageSrc, path.Join(p.dir, "part.html")))
	}
	for _, err := range errs {
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// checkLooks refuses a look that k's own parts and looks take that is no part or look of k or its base, and,
// when they name none such, each cycle of looks taken that one of them is in: the CSS of a look goes before
// that of those that take it, an order a cycle has none of.
func (k *Kit) checkLooks() usererr.List {
	own := map[string][]taken{}
	for name, p := range k.parts {
		own[name] = p.looks
	}
	for name, l := range k.looks {
		own[name] = l.looks
	}
	var found usererr.List
	for _, name := range slices.Sorted(maps.Keys(own)) {
		for _, t := range own[name] {
			if !k.isPartOrLook(t.name) {
				found = append(found, yamlpos.New(t.file, t.at, "kit_look_unknown", t.name))
			}
		}
	}
	if found != nil {
		return found
	}
	return k.lookCycles(own)
}

// lookCycles refuses each cycle of the looks the parts and looks of k and its base take that one of own is
// in, at the first of own in it, at the look it takes next in it.
func (k *Kit) lookCycles(own map[string][]taken) usererr.List {
	parts, looks, _, _ := k.Names()
	all := slices.Sorted(slices.Values(slices.Concat(parts, looks)))
	takes := map[string][]taken{}
	uses := map[string][]string{}
	for _, name := range all {
		if p, ok := k.Part(name); ok {
			takes[name] = p.looks
		} else if l, ok := k.Look(name); ok {
			takes[name] = l.looks
		}
		uses[name] = names(takes[name])
	}
	var found usererr.List
	for _, c := range cycle.Of(all, uses) {
		i := slices.IndexFunc(c[:len(c)-1], func(name string) bool { _, mine := own[name]; return mine })
		if i < 0 {
			continue
		}
		at := slices.IndexFunc(takes[c[i]], func(t taken) bool { return t.name == c[i+1] })
		t := takes[c[i]][at]
		// The path from the name told, round to it.
		path := slices.Concat(c[i:len(c)-1], c[:i+1])
		key := "kit_look_cycle_part"
		if _, ok := k.Look(c[i]); ok {
			key = "kit_look_cycle_look"
		}
		found = append(found, yamlpos.New(t.file, t.at, key, c[i], strings.Join(path, " → ")))
	}
	return found
}

// unread refuses each file of the directory dir that it holds and floe does not read, with key.
func unread(dir string, files []string, key string) usererr.List {
	found := make(usererr.List, 0, len(files))
	for _, f := range files {
		found = append(found, usererr.NewAt(path.Join(dir, f), 0, 0, key))
	}
	return found
}

// checkTheme checks the CSS of t, palette.css as the others; and refuses a file it does not read, of no part or
// look.
func checkTheme(t *Theme) error {
	errs := []error{unread(t.dir, t.unread, "kit_theme_unread").Err()}
	if t.palette != nil {
		errs = append(errs, css.Check(path.Join(t.dir, "palette.css"), t.palette))
	}
	if t.base != nil {
		errs = append(errs, css.Check(path.Join(t.dir, "base.css"), t.base))
	}
	for _, name := range slices.Sorted(maps.Keys(t.partsCSS)) {
		errs = append(errs, css.Check(path.Join(t.dir, name+".css"), t.partsCSS[name]))
	}
	return usererr.Collect(errs...)
}

// checkTemplate checks that t holds page.html, schema.json and example.yaml, whose template: names t, and no
// file it does not read, its page.css, and its page's escaping.
func checkTemplate(t *Template) error {
	var found usererr.List
	files := map[string]bool{"page.html": t.pages != nil, "schema.json": t.hasSchema, "example.yaml": t.example != nil}
	for file, has := range files {
		if !has {
			found = append(found, usererr.NewAt(t.dir, 0, 0, "kit_template_file", file))
		}
	}
	errs := []error{
		found.Err(), exampleNames(t), forMistakes(t.checks, t.schema).Err(), css.Check(path.Join(t.dir, "page.css"), t.css),
	}
	if t.pages != nil {
		errs = append(errs, escapeMistake(t.pages[""], t.pageSrc, path.Join(t.dir, "page.html")))
	}
	errs = append(errs, unread(t.dir, t.unread, "kit_template_unread").Err())
	return usererr.Collect(errs...)
}

// exampleNames refuses an example.yaml of t whose template: names another: floe template check would check
// that one. An example that cannot be read is told when it is drawn.
func exampleNames(t *Template) error {
	if t.example == nil {
		return nil
	}
	d, _ := doc.Read(t.File("example.yaml"), t.example, "") //nolint:errcheck // told when it is drawn
	if d == nil || d.Template == "" || d.Template == t.name {
		return nil
	}
	return yamlpos.New(d.File, d.Where["template"], "kit_example_name", d.Template, t.name)
}

// forMistakes refuses each for of checks, of the schema c, through a key the schema forbids (S10): such a
// check would find no items, and pass unseen.
func forMistakes(checks *rules.Checks, c *schema.Compiled) usererr.List {
	if checks == nil || c == nil {
		return nil
	}
	var found usererr.List
	for _, f := range checks.Fors() {
		if key, ok := c.Forbidding(strings.Split(f.Path, ".")); ok {
			found = append(found, yamlpos.New(checks.File(), f.At, "kit_for_undeclared", f.Path, key))
		}
	}
	return found
}
