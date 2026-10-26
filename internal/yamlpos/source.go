package yamlpos

import (
	"cmp"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	"github.com/sukekyo26/floe/internal/bom"
)

// Source returns the value n as written: the text of its tokens, each as the parser kept it (Origin), from its
// first to its last, its line breaks \n, trimmed, and its lines after the first without the indentation they
// share with the line it starts on, counted in characters. Lines and columns are not used: the parser's are off
// after a tab, inside a folded block with a blank line and with lone CR line breaks. Comments before its first
// token and after its last (on its last line too) are no tokens of it, and the blank lines that end a |+ or >+
// are trimmed; a quoted value over a line indented less than its keys sets the indentation taken off.
// LIMIT: the comments around it and a |+ value's last blank lines are left out; add them if a spec needs them.
// LIMIT: a quoted value over a shallower line sets the indentation taken off; measure it apart if one shows so.
func Source(n ast.Node) string {
	in := tokensOf{}
	ast.Walk(in, n)
	// Once along the file's tokens, from its first.
	var head, first, last *token.Token
	for tk := range in {
		head = tk
		break
	}
	if head == nil {
		return ""
	}
	for head.Prev != nil {
		head = head.Prev
	}
	for tk := head; tk != nil; tk = tk.Next {
		if in[tk] {
			first = cmp.Or(first, tk)
			last = tk
		}
	}
	var b strings.Builder
	for tk := first; ; tk = tk.Next {
		b.WriteString(tk.Origin)
		if tk == last {
			break
		}
	}
	text := newlines(b.String())
	body := strings.TrimLeft(text, " \t\n")
	lines := strings.Split(strings.TrimRight(body, " \t\n"), "\n")
	shared := indentBefore(first, text[:len(text)-len(body)])
	for _, line := range lines[1:] {
		rest := strings.TrimLeft(line, " ")
		if rest != "" && !strings.HasPrefix(rest, "#") {
			shared = min(shared, utf8.RuneCountInString(line)-utf8.RuneCountInString(rest))
		}
	}
	for i := 1; i < len(lines); i++ {
		cut := 0
		for cut < shared && cut < len(lines[i]) && lines[i][cut] == ' ' {
			cut++
		}
		lines[i] = lines[i][cut:]
	}
	return strings.Join(lines, "\n")
}

// File returns src, a YAML file, as Source writes a value: without its BOM, its line breaks \n, trimmed.
func File(src []byte) string {
	return strings.TrimSpace(newlines(string(bom.Trim(src))))
}

// indentBefore returns how many characters are before the first token on its line: lead, what its Origin holds
// before it, after its last line break, else that and the text of the tokens before it back to one.
func indentBefore(first *token.Token, lead string) int {
	if i := strings.LastIndex(lead, "\n"); i >= 0 {
		return utf8.RuneCountInString(lead[i+1:])
	}
	n := utf8.RuneCountInString(lead)
	for tk := first.Prev; tk != nil; tk = tk.Prev {
		o := newlines(tk.Origin)
		if i := strings.LastIndex(o, "\n"); i >= 0 {
			return n + utf8.RuneCountInString(o[i+1:])
		}
		n += utf8.RuneCountInString(o)
	}
	return n
}

func newlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// tokensOf is an ast.Visitor that keeps in each token of the nodes it visits, with a flow collection's brackets.
type tokensOf map[*token.Token]bool

func (in tokensOf) Visit(n ast.Node) ast.Visitor {
	add := func(tks ...*token.Token) {
		for _, tk := range tks {
			if tk != nil {
				in[tk] = true
			}
		}
	}
	add(n.GetToken())
	switch n := n.(type) {
	case *ast.MappingNode:
		add(n.Start, n.End)
	case *ast.SequenceNode:
		add(n.Start, n.End)
	}
	return in
}
