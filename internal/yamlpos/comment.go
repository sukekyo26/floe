package yamlpos

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// comments refuses each comment of doc that may have cut a value short, once for each value, at the value's start:
// one on the line of a plain value YAML 1.2 reads as text (a key's is not judged: it ends no value), which may have
// ended it (title: Plan #1 draft reads Plan); and one right after a | or > text, which may have ended it, its line
// shallower than the text's indentation. go.yaml.in/yaml/v4 gives the one as the value's LineComment, and the other
// as the HeadComment of the node after the text, the FootComment of a key, an item or a value whose last scalar is
// the text, or the document's FootComment.
// LIMIT: a comment after a | or > text at its right place, as deep as its key or shallower (t: |, "  x", "# c"),
// and one after the - of the next item (- # c), are refused too: v4's tree gives them as it gives one that cut the
// text, and floe refuses the one form alike (the user decided).
// LIMIT: a comment right after a | or > text past --- or ... ("---", "# c"), or after an empty | text, is refused
// too, and so is one after an empty [] or {} item that follows the text (- |, x, - [], "# c"): the walk follows
// scalars alone.
// LIMIT: a comment right after a | or > text that v4 gives as the foot of the next item's first key passes, as the
// walk looks for the text under that key alone: one with a blank line and an item at its depth after it (- text: |,
// "    one", "  # c", "", "  - text: more"), or one followed by an empty item (- |, "  x", " # c", -).
// LIMIT: a comment on a line of its own, deeper than the key of a plain value before it (a: b, then "   # c"), passes:
// YAML 1.2 reads it as a comment, and a plain value goes on over lines only where the line does not start with #,
// so a line of such a value over lines that starts with # ends it with no mistake (a: Plan, "  draft", "  #1").
func (w *walk) comments(doc *yaml.Node) {
	c := &cuts{w: w, told: map[*yaml.Node]bool{}, prev: nil}
	c.walk(doc, nil)
}

// cuts is a walk of a document for the comments that may have cut a value short.
type cuts struct {
	w    *walk
	told map[*yaml.Node]bool // the values told, each once
	prev *yaml.Node          // the scalar before, in the order written
}

// walk walks n, the key of value when value is not nil. An anchor, alias or tag, refused as such, is not walked.
func (c *cuts) walk(n, value *yaml.Node) {
	if n.HeadComment != "" {
		c.after(c.prev)
	}
	if wrapped(n) {
		return
	}
	if n.Kind == yaml.ScalarNode {
		if value == nil && n.LineComment != "" && styleOf(n) == Plain && read12(n.Value).kind == kindText {
			c.tell(n, "yaml_comment_cut")
		}
		c.prev = n
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Tag == "!!merge" {
				continue
			}
			c.walk(n.Content[i], n.Content[i+1])
			c.walk(n.Content[i+1], nil)
		}
	} else {
		for _, item := range n.Content {
			c.walk(item, nil)
		}
	}
	if n.FootComment != "" {
		under := n
		if value != nil { // a key: what its value holds
			under = value
		}
		c.after(lastScalar(under))
	}
}

// after tells a comment right after n when n is a | or > text.
func (c *cuts) after(n *yaml.Node) {
	if n != nil && (styleOf(n) == Literal || styleOf(n) == Folded) {
		c.tell(n, "yaml_comment_after_block")
	}
}

// tell refuses value with key, once; an empty | or > text is shown as its mark.
func (c *cuts) tell(value *yaml.Node, key string) {
	if c.told[value] {
		return
	}
	c.told[value] = true
	end := endOf(value.Value)
	if end == "" {
		end = map[Style]string{Literal: "|", Folded: ">"}[styleOf(value)]
	}
	c.w.found = append(c.w.found, New(c.w.file, at(value), key, end))
}

// lastScalar is the last scalar under n in the order written, n itself for a scalar; nil for none.
func lastScalar(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.ScalarNode {
		return n
	}
	for _, v := range slices.Backward(n.Content) {
		if s := lastScalar(v); s != nil {
			return s
		}
	}
	return nil
}

// endOf returns the end of value, a value cut short, to show in its mistake on one line: a line break, which a
// blank line in a value over lines makes, as a space, and only the last shownEnd characters after "…" of a longer
// one.
func endOf(value string) string {
	r := []rune(strings.ReplaceAll(strings.TrimRight(value, "\n"), "\n", " "))
	if len(r) <= shownEnd {
		return string(r)
	}
	return "…" + string(r[len(r)-shownEnd:])
}
