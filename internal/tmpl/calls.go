package tmpl

import (
	"html/template"
	"slices"
	"strings"
	tparse "text/template/parse"
)

// calls are the calls of a function of a template: the names passed to it as written, once each in the
// order they come, and whether it is called otherwise (with a value of the data, or as no name is).
type calls struct {
	names []string
	other bool
}

// callsOf returns the calls of the function fn in page and the templates it defines, in the order of
// their names: fn "x" and "x" | fn name x, any other call is other.
// LIMIT: a name in parentheses (fn ("x")) is other; follow the pipe in them if a template comes to write
// one so.
func callsOf(page *template.Template, fn string) calls {
	var c calls
	templates := page.Templates()
	slices.SortFunc(templates, func(x, y *template.Template) int { return strings.Compare(x.Name(), y.Name()) })
	for _, t := range templates {
		if t.Tree != nil {
			c.node(fn, t.Tree.Root)
		}
	}
	return c
}

func (c *calls) node(fn string, n tparse.Node) {
	if p, ok := n.(*tparse.PipeNode); ok && p != nil {
		for i, cmd := range p.Cmds {
			if !calls1(cmd, fn) {
				continue
			}
			switch {
			case len(cmd.Args) == 2:
				c.add(cmd.Args[1])
			case len(cmd.Args) == 1 && i > 0 && len(p.Cmds[i-1].Args) == 1:
				c.add(p.Cmds[i-1].Args[0])
			default:
				c.other = true
			}
		}
	}
	for _, child := range children(n) {
		c.node(fn, child)
	}
}

// add notes arg, passed to the function: its text when written, else a call of another kind.
func (c *calls) add(arg tparse.Node) {
	s, ok := arg.(*tparse.StringNode)
	switch {
	case !ok:
		c.other = true
	case !slices.Contains(c.names, s.Text):
		c.names = append(c.names, s.Text)
	}
}

// calls1 reports whether cmd calls the function fn.
func calls1(cmd *tparse.CommandNode, fn string) bool {
	id, ok := cmd.Args[0].(*tparse.IdentifierNode)
	return ok && id.Ident == fn
}

// children returns the nodes right under n that may hold a call.
func children(n tparse.Node) []tparse.Node {
	switch n := n.(type) {
	case *tparse.ListNode:
		if n != nil {
			return n.Nodes
		}
	case *tparse.PipeNode:
		if n != nil {
			nodes := make([]tparse.Node, 0, len(n.Cmds))
			for _, c := range n.Cmds {
				nodes = append(nodes, c)
			}
			return nodes
		}
	case *tparse.ActionNode:
		return []tparse.Node{n.Pipe}
	case *tparse.TemplateNode:
		return []tparse.Node{n.Pipe}
	case *tparse.CommandNode:
		return n.Args
	case *tparse.ChainNode: // (fn "x").y
		return []tparse.Node{n.Node}
	case *tparse.IfNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	case *tparse.RangeNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	case *tparse.WithNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	}
	return nil
}
