package lint

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// textSteps are base.css's steps of parts' small text, smallest first.
var textSteps = []string{"var(--floe-text-small)", "var(--floe-text-aside)"}

// textScale keeps the size of parts' and looks' text, in font-size and in font, to base.css's steps of small text, inherit,
// or a rem or an em of 1 or more. Only floe's base.css sets a step. An exception's Key is a part's file, the rule and the
// size it writes: "parts/code/part.css :scope pre: 0.875rem".
// LIMIT: the templates' page.css and the themes are not read for sizes.
var textScale = Rule{Name: "text-scale", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		if s.Kind != kindBase {
			for name, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--floe-text-") }) {
				for _, d := range ds {
					file, line := s.at(d.at)
					found = append(found, Finding{
						file, line, "text-scale",
						"sets " + name + "; only floe's base.css sets the steps of small text, and a theme writes a size of its own as it is",
					})
				}
			}
		}
		if s.Kind != kindPart && s.Kind != kindLook {
			continue
		}
		sizes := declarations(s.Src, func(p string) bool { return p == "font-size" || p == "font" })
		for _, prop := range slices.Sorted(maps.Keys(sizes)) {
			for _, d := range sizes[prop] {
				in.Saw()
				v, ok := sizeOf(prop, d.value)
				var why string
				switch {
				case !ok:
					why = "writes no size"
				case slices.Contains(textSteps, v) || v == "inherit" || large(v) || in.Allowed(s.File+" "+d.rule+": "+v):
					continue
				default:
					why = fmt.Sprintf("has the size %q", v)
				}
				file, line := s.at(d.at)
				found = append(found, Finding{file, line, "text-scale", fmt.Sprintf(
					"%s: %s in %s %s; write a part's small text by one of base.css's steps %s",
					prop, text(s.Src, d.value), d.rule, why, strings.Join(textSteps, ", "))})
			}
		}
	}
	return found
}, Allow: []Allow{
	{"parts/code/part.css :scope pre: 0.875rem", "code's lines, which follow code's size rather than a step"},
	{"parts/tree/part.css :scope .name: 0.86rem", "a tree's names, code, which follow code's size"},
	{"parts/tags/part.css :scope code: 0.86rem", "the code in a row of tags, which follows code's size"},
	{"parts/diagram/part.css .dg .main: 13px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .sub: 12px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .code: 12px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .type: 12px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .frame text: 12px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .frame.grouped text: 13px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .label: 12px", "a diagram's text, in the SVG's own units"},
	{"parts/diagram/part.css .dg .label.strong: 13px", "a diagram's text, in the SVG's own units"},
}}
