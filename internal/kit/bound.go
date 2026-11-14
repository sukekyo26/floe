package kit

import (
	"fmt"
	"html/template"
	"maps"
	"path"
	"slices"
	"text/template/parse"

	"floe/internal/textpos"
	"floe/internal/usererr"
)

// boundFuncs are the functions of a part.html bound to the part draw draws (BoundFuncs), each here a name alone,
// which part.html is read with: they take no argument and read the part's value, so that the places of their
// mistakes, at a JSON pointer from the root of the value, are where they say.
var boundFuncs = template.FuncMap{
	"bars": unbound[[]bar]("bars"), "chart": unbound[*chart]("chart"), "code": unbound[codeText]("code"),
	"diagram": unbound[template.HTML]("diagram"), "script": unbound[template.HTML]("script"),
	"stats": unbound[[]stat]("stats"), "written": unbound[any]("written"),
}

// unbound is a function of boundFuncs, named name, as it is before draw binds it.
func unbound[T any](name string) func() (T, error) {
	return func() (T, error) {
		var none T
		return none, fmt.Errorf("%w: %s: not bound to a part", ErrFunc, name)
	}
}

// BoundFuncs returns the functions of boundFuncs bound to a part drawn: value, its value as drawn, which they read;
// written, its value as written, which written returns; and diagram, which draws written as a diagram. A value of a
// shape they do not read is the mistake kit_part_value, at the value or its key: the part's schema, which may let it
// through, says what it takes, and no argument has text/template check its type.
func BoundFuncs(value, written any, diagram func() (template.HTML, error)) template.FuncMap {
	return template.FuncMap{
		"bars": func() ([]bar, error) {
			items, err := listOf("bars", value)
			if err != nil {
				return nil, err
			}
			return barsOf(items)
		},
		"chart": func() (*chart, error) { return chartOf(value) },
		"code": func() (codeText, error) {
			m, text, err := keyedText("code", value, "text")
			if err != nil {
				return codeText{Lines: nil, Before: 0, Digits: 0, Numbered: false}, err
			}
			return codeOf(text, m["start"], m["mark"])
		},
		"diagram": diagram,
		"script": func() (template.HTML, error) {
			m, js, err := keyedText("script", value, "js")
			if err != nil {
				return "", err
			}
			return scriptOf(js, m["data"])
		},
		"stats": func() ([]stat, error) {
			items, err := listOf("stats", value)
			if err != nil {
				return nil, err
			}
			return statsOf(items)
		},
		"written": func() (any, error) { return written, nil },
	}
}

// listOf is value, a part's, as the list the function fn reads.
func listOf(fn string, value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, usererr.New("kit_part_value", fn, ".", "array").PlacedIn("#")
	}
	return items, nil
}

// keyedText is value, a part's, as the keys and values the function fn reads, with the text of its key.
// LIMIT: a key the schema makes Markdown (drawn as HTML) or leaves out is told to be given "type": "string", which
// it may have: say its format and that it is required too if part authors come to miss it.
func keyedText(fn string, value any, key string) (map[string]any, string, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, "", usererr.New("kit_part_value", fn, ".", "object").PlacedIn("#")
	}
	text, ok := m[key].(string)
	if !ok {
		return nil, "", usererr.New("kit_part_value", fn, "."+key, "string").PlacedIn("#/" + key)
	}
	return m, text, nil
}

// boundArgs refuses, in page, a part.html of src read from the directory dir, each call of a function of
// boundFuncs given an argument, as an argument after it or a value piped into it, at its line and column: it would
// be refused at each page it draws (wrong number of args), as text/template counts the arguments only then.
func boundArgs(page *template.Template, src []byte, dir string) usererr.List {
	file := path.Join(dir, "part.html")
	var found usererr.List
	for _, t := range page.Templates() {
		if t.Tree == nil {
			continue
		}
		walkCommands(t.Tree.Root, func(cmd *parse.CommandNode, piped bool) {
			id, ok := cmd.Args[0].(*parse.IdentifierNode)
			if !ok || boundFuncs[id.Ident] == nil || len(cmd.Args) == 1 && !piped {
				return
			}
			line, column := textpos.Of(src, int(id.Position()))
			found = append(found, usererr.NewAt(file, line, column, "kit_bound_args", id.Ident))
		})
	}
	return found.Sorted()
}

// walkCommands calls fn with each command under n, and whether a value is piped into it (a command after the first
// of its pipeline).
func walkCommands(n parse.Node, fn func(cmd *parse.CommandNode, piped bool)) {
	switch n := n.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, c := range n.Nodes {
				walkCommands(c, fn)
			}
		}
	case *parse.ActionNode:
		walkCommands(n.Pipe, fn)
	case *parse.IfNode:
		walkBranch(&n.BranchNode, fn)
	case *parse.RangeNode:
		walkBranch(&n.BranchNode, fn)
	case *parse.WithNode:
		walkBranch(&n.BranchNode, fn)
	case *parse.TemplateNode:
		walkCommands(n.Pipe, fn)
	case *parse.PipeNode:
		if n == nil {
			return
		}
		for i, cmd := range n.Cmds {
			fn(cmd, i > 0)
			for _, arg := range cmd.Args {
				walkCommands(arg, fn)
			}
		}
	case *parse.ChainNode:
		walkCommands(n.Node, fn)
	default:
	}
}

// walkBranch walks the pipeline and the lists of an if, a range or a with.
func walkBranch(b *parse.BranchNode, fn func(cmd *parse.CommandNode, piped bool)) {
	walkCommands(b.Pipe, fn)
	walkCommands(b.List, fn)
	walkCommands(b.ElseList, fn)
}

// boundNames are the names of boundFuncs, in order.
func boundNames() []string {
	return slices.Sorted(maps.Keys(boundFuncs))
}
