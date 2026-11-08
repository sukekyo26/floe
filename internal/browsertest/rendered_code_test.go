package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

const codePage = `template: report
lang: ja
title: Code
sections:
  - heading: A
    parts:
      - code:
          title: 誤りの応答を返す所（直す前）
          file: internal/auth/login.go
          lang: go
          start: 12
          mark: [15]
          text: "func Login() {\n\tuser := find()\n\tif user == nil {\n\t\thttp.Error()\n\t}\n\n}\n"
      - code: { lang: go, mark: [2, 3], text: "if err != nil {\n\treturn err\n}\n" }
      - code:
          title: Long
          file: run.sh
          start: 98
          mark: [99, 100]
          text: "short\nfloe render --out-dir internal/kit/floe/parts/findings/testdata/golden/with/a/very/long/path/that/never/ends/at/all/and/goes/on/and/on --check\nmid\nlast\n"
      - code: { start: 100, text: "a\n\tb\n\t\tc\n" }
      - code:
          title: 誤りの応答を返す所で、利用者に見せてはいけない内部の情報が混ざらないことを確かめる所（直す前の形）
          file: internal/kit/floe/parts/findings/testdata/golden/part_internal_test.go
          lang: typescript
          mark: [1, 99]
          text: let a = 1
      - code: { file: main.go, text: "plain\n" }
`

// codeWritten are the texts of codePage's code, each as the button copies it.
var codeWritten = []string{
	"func Login() {\n\tuser := find()\n\tif user == nil {\n\t\thttp.Error()\n\t}\n\n}\n",
	"if err != nil {\n\treturn err\n}\n",
	"short\nfloe render --out-dir internal/kit/floe/parts/findings/testdata/golden/with/a/very/long/path/that/never/ends/at/all/and/goes/on/and/on --check\nmid\nlast\n",
	"a\n\tb\n\t\tc\n",
	"let a = 1\n",
	"plain\n",
}

// codeHelpers read what code draws: its lines, where the text of a line starts, one character's width, and its rows.
const codeHelpers = `
  const parts = [...document.querySelectorAll('[data-floe-part=code]')];
  const lines = (p) => [...p.querySelectorAll('.lines > [data-floe-copy]')];
  const textAt = (e, i) => { const r = document.createRange(); r.setStart(e.firstChild, i); r.setEnd(e.firstChild, i + 1); return r.getBoundingClientRect().left; };
  const ch = (p) => { const c = document.createElement('span'); c.textContent = '0'; lines(p)[0].append(c); const w = c.getBoundingClientRect().width; c.remove(); return w; };
`

// measureCode tells what code's lines must hold on screen: the copy of each as written, the selection without its
// numbers and hidden words, every line reaching the end of the longest, the text of every line starting at one place
// and the room for its number, a tab's width kept, what is marked, and the title no heading.
const measureCode = `(() => {` + codeHelpers + `
  const out = new Set();
  const doc = document.documentElement;
  if (doc.scrollWidth > doc.clientWidth) out.add('the page scrolls sideways');
  const written = %s;
  parts.forEach((p, i) => {
    const copy = lines(p).map((e) => e.textContent.endsWith('\n') ? e.textContent : e.textContent + '\n').join('');
    if (copy !== written[i]) out.add(i + ' copies ' + JSON.stringify(copy));
    const pre = p.querySelector('pre'), sel = getSelection();
    sel.selectAllChildren(pre);
    const selected = sel.toString();
    sel.removeAllRanges();
    if (selected !== written[i].replace(/\n$/, '')) out.add(i + ' selects ' + JSON.stringify(selected));
    for (const e of lines(p)) if (Math.round(e.getBoundingClientRect().width) !== pre.scrollWidth) out.add(i + ' has a line ' + e.getBoundingClientRect().width + ' wide in ' + pre.scrollWidth);
    const room = parseFloat(getComputedStyle(p).getPropertyValue('--floe-code-pad')) * parseFloat(getComputedStyle(doc).fontSize);
    const numbered = getComputedStyle(lines(p)[0], '::before').content !== 'none';
    const digits = numbered ? Number(p.querySelector('code').style.getPropertyValue('--digits')) : 0;
    const want = pre.getBoundingClientRect().left + room + (numbered ? (digits + 2) * ch(p) : 0);
    for (const e of lines(p)) {
      if (e.textContent === '\n' || e.textContent[0] === '\t') continue;
      const got = textAt(e, 0);
      if (Math.abs(got - want) > 0.5) out.add(i + ' has text at ' + got + ', want ' + want);
    }
  });
  const tabs = lines(parts[3]), a = textAt(tabs[0], 0), w = ch(parts[3]);
  // Within a tenth of a character: a font's widths round the tab stops.
  const steps = [textAt(tabs[1], 1), textAt(tabs[2], 2)].map((x) => (x - a) / w);
  if (Math.abs(steps[0] - 4) > 0.1 || Math.abs(steps[1] - 8) > 0.1) out.add('tabs of ' + steps.map((n) => n.toFixed(2)).join(' ') + ' characters');
  const markedOf = (p) => [...p.querySelectorAll('.mark')].map((e) => e.textContent);
  const marks = JSON.stringify(parts.map(markedOf));
  if (marks !== JSON.stringify([['\t\thttp.Error()\n'], ['\treturn err\n', '}\n'], [lines(parts[2])[1].textContent, 'mid\n'], [], ['let a = 1'], []])) out.add('marks ' + marks);
  for (const e of document.querySelectorAll('[data-floe-part=code] .mark')) {
    const s = getComputedStyle(e);
    if (s.borderLeftStyle !== 'solid' || parseFloat(s.borderLeftWidth) !== 3) out.add('a marked line with no line at its side');
    if (e.previousElementSibling?.className !== 'floe-hidden' || !e.previousElementSibling.textContent) out.add('a marked line tells no hidden word');
  }
  for (const i of [1, 4, 5]) if (getComputedStyle(lines(parts[i])[0], '::before').content !== 'none') out.add(i + ' numbers its lines with no start');
  const counts = parts.map((p) => getComputedStyle(p.querySelector('code')).counterReset).join();
  if (counts !== 'line 11,none,line 97,line 99,none,none') out.add('counts from ' + counts);
  for (const t of document.querySelectorAll('[data-floe-part=code] .title')) if (t.tagName !== 'SPAN' || !t.closest('figcaption')) out.add('a title as ' + t.tagName + ' under ' + t.parentElement.tagName);
  for (const c of document.querySelectorAll('[data-floe-part=code] figcaption')) {
    const box = c.getBoundingClientRect();
    for (const e of c.querySelectorAll('span')) { const r = e.getBoundingClientRect(); if (r.left < box.left - 0.5 || r.right > box.right + 0.5) out.add(e.className + ' out of the heading'); }
  }
  return [...out].join('\n');
})()`

// measureCodePaper tells, drawn for print, that a long line folds right of its number, its ground on all of it.
const measureCodePaper = `(() => {` + codeHelpers + `
  const out = new Set();
  const pre = parts[2].querySelector('pre'), long = lines(parts[2])[1];
  if (pre.scrollWidth > pre.clientWidth) out.add('it scrolls on paper');
  const r = document.createRange();
  r.selectNodeContents(long);
  // A row may hold a box for each word: what starts a row is its leftmost.
  const starts = new Map();
  for (const c of r.getClientRects()) starts.set(Math.round(c.top), Math.min(starts.get(Math.round(c.top)) ?? Infinity, Math.round(c.left)));
  const rows = [...new Set(starts.values())];
  if (rows.length !== 1) out.add('its rows start at ' + rows);
  if (long.getBoundingClientRect().height < 2 * lines(parts[2])[0].getBoundingClientRect().height) out.add('it does not fold');
  const number = textAt(lines(parts[2])[0], 0);
  if (Math.abs(rows[0] - number) > 1) out.add('its rows at ' + rows[0] + ', the text of a line at ' + number);
  return [...out].join('\n');
})()`

// measureCodeColours tells what falls short in a theme: text at 4.5 to its ground, the marked line's side at 3, each
// colour mixed over what lies under it.
const measureCodeColours = `(() => {` + colours + codeHelpers + `
  const out = [];
  const p = parts[0], faint = lines(p)[0], mark = p.querySelector('.mark');
  const code = ground(p.querySelector('pre')).colour, marked = ground(mark).colour, head = ground(p.querySelector('figcaption')).colour;
  const on = (colour, under) => ratio(over(rgba(colour), under), under);
  for (const [what, q, least] of [
    ['a marked line', on(getComputedStyle(mark).color, marked), 4.5],
    ['a marked number', on(getComputedStyle(mark, '::before').color, marked), 4.5],
    ['a number', on(getComputedStyle(faint, '::before').color, code), 4.5],
    ['the title', on(getComputedStyle(p.querySelector('.title')).color, head), 4.5],
    ['the file', on(getComputedStyle(p.querySelector('.file')).color, head), 4.5],
    ['the side', on(getComputedStyle(mark).borderLeftColor, code), 3],
  ]) if (q < least) out.push(what + ' at ' + q.toFixed(2));
  return out.join(', ');
})()`

// TestRenderedCode draws code's title, numbers and marks in every theme at the widths that turn how they fold, and on paper.
func TestRenderedCode(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	written, err := json.Marshal(codeWritten)
	if err != nil {
		t.Fatal(err)
	}
	screen := fmt.Sprintf(measureCode, written)
	file := s.write(t, "code.yaml", codePage)
	amiss := []string{}
	for _, th := range themes {
		s.floe(t, "publish", file, "--slug", "code-"+th.Name(), "--theme", th.Name())
		b.navigate("/a/code-" + th.Name() + "/?raw")
		if got := b.eval(measureCodeColours); got != "" {
			amiss = append(amiss, th.Name()+": "+got)
		}
		for _, w := range []int64{1440, 400, 320} {
			b.size(w, 900, false)
			if got := b.eval(screen); got != "" {
				amiss = append(amiss, fmt.Sprintf("%s at %d:\n%s", th.Name(), w, got))
			}
		}
		b.unsize()
		b.media("print")
		if got := b.eval(measureCodePaper); got != "" {
			amiss = append(amiss, th.Name()+" on paper:\n"+got)
		}
		b.media("")
	}
	if len(amiss) > 0 {
		t.Error(strings.Join(amiss, "\n"))
	}
	b.navigate("/a/code-default/?raw")
	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	var spoken []string
	for _, n := range tree.Nodes {
		if n.Ignored || n.Role == nil || n.Name == nil {
			continue
		}
		var role, name string
		if json.Unmarshal(n.Role.Value, &role) == nil && role == "StaticText" && json.Unmarshal(n.Name.Value, &name) == nil {
			spoken = append(spoken, strings.TrimSpace(name))
		}
	}
	want := []string{"14", "if user == nil {", "見てほしい行：", "15", "http.Error()", "16"}
	if i := slices.Index(spoken, "14"); i < 0 || i+len(want) > len(spoken) || !slices.Equal(spoken[i:i+len(want)], want) {
		t.Errorf("read as %q, want %q in it", spoken, want)
	}
}

var codeOwnKit = map[string]string{
	"parts/lined/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
	"parts/lined/part.html": `{{$c := code .text .start .mark}}<ol data-floe-part="lined" data-before="{{$c.Before}}" data-digits="{{$c.Digits}}" data-numbered="{{$c.Numbered}}">` +
		`{{range $c.Lines}}<li{{if .Mark}} class="mark"{{end}}>{{.Text}}</li>{{end}}</ol>`,
	"parts/lined/part.css": "",
	"templates/lines/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.lined}}\n</body></html>\n",
	"templates/lines/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"lined": {"$ref": "floe:///parts/lined.json"}}}`,
	"templates/lines/example.yaml": "lang: en\ntemplate: lines\nlined: { text: a }\n",
}

// TestRenderedCodeOwnPart draws a part of one's own by the function code: its lines, numbers and marks are what the docs say.
func TestRenderedCodeOwnPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, codeOwnKit)
	file := s.write(t, "own.yaml", "lang: en\ntemplate: lines\ntitle: Own\nlined: { start: 3, mark: [4], text: \"a\\nb\\n\" }\n")
	s.floe(t, "publish", file, "--slug", "own")
	s.b.navigate("/a/own/?raw")
	got := s.b.eval(`(() => { const l = document.querySelector('[data-floe-part=lined]');
		return JSON.stringify([l.dataset.before, l.dataset.digits, l.dataset.numbered, [...l.children].map((e) => [e.textContent, e.className])]); })()`)
	check(t, "own", []string{got}, `["2","1","true",[["a\n",""],["b\n","mark"]]]`)
}
