package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// treeNotesPage holds rows of every depth, a name long and short, a mark (+ ± −) and none, a note and none, a folder
// of no note over rows with one, and a long note.
const treeNotesPage = `template: report
lang: en
title: Notes
sections:
  - heading: Notes
    parts:
      - tree:
          - { path: internal/auth/login.go, note: entry, change: changed }
          - { path: internal/auth/check.go, note: input check, change: added }
          - { path: internal/auth/old_form.go, note: old screen, change: removed }
          - { path: internal/auth/deep/er/x.go }
          - { path: internal/auth/deep/er/very_long_file_name_here.go, note: a note long enough to fold onto more than one line beside the names of the rows over it }
          - { path: internal/plain/inner.go, note: under a folder of no note }
          - { path: docs/login.md, note: guide }
          - { path: README.md }
`

// measureTreeNotes returns a line for what stands amiss: the notes of a tree start at one edge, whatever the depth, the
// mark and the text of a name end before the note of its row, and a row with no note is no taller than its name.
// LIMIT: on paper the names fold and the marks are not measured (a mark is a generated box, found by letting the name
// reach its widest, which unfolds it); the text of every line of a name is.
const measureTreeNotes = `(() => {
  ` + textRightJS + `
  const out = [], paper = matchMedia('print').matches;
  const trees = document.querySelectorAll('[data-floe-part=tree]');
  if (trees.length !== 1) return 'trees: ' + trees.length;
  let notes = 0, left;
  for (const li of trees[0].querySelectorAll('li')) {
    const name = li.querySelector(':scope > .name'), note = li.querySelector(':scope > .note');
    const label = name.firstChild.data;
    if (!note) {
      const pad = li.getBoundingClientRect().height - name.getBoundingClientRect().height;
      if (!li.querySelector('li') && pad > 8) out.push(label + ': a row of no note is ' + pad + ' px taller than its name');
      continue;
    }
    notes++;
    const x = note.getBoundingClientRect().left;
    if (left === undefined) left = x;
    else if (Math.abs(x - left) > 1) out.push(label + ': its note starts at ' + x + ', not ' + left);
    let right = Math.max(...[...name.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE).map(textRight));
    if (!paper && li.matches('.added, .removed, .changed')) {
      name.style.width = 'max-content';
      right = Math.max(right, name.getBoundingClientRect().right);
      name.style.width = '';
    }
    if (right > x + 0.5) out.push(label + ': its name or mark ends at ' + right + ', past its note at ' + x);
  }
  if (notes !== 6) out.push('notes: ' + notes);
  return out.join('\n');
})()`

// TestRenderedTreeNotes measures that a tree's notes start at one edge in every theme, on a screen as wide as a desktop
// and a phone, and on paper.
func TestRenderedTreeNotes(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "notes-"+theme+".yaml", treeNotesPage+"theme: "+theme+"\n"), "--slug", "notes-"+theme)
	}
	defer s.b.unsize()
	measure := func(what string) {
		for _, theme := range themes {
			s.b.navigate("/a/notes-" + theme + "/?raw")
			if got := s.b.eval(measureTreeNotes); got != "" {
				t.Errorf("%s, %s:\n%s", theme, what, got)
			}
		}
	}
	s.b.size(1440, 900, false)
	measure("1440 px")
	onPhone(t, s, 390)
	measure("390 px")
	s.b.size(paperWidth, 1000, false)
	s.b.media("print")
	measure("paper")
}
