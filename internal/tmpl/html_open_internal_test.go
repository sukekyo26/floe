package tmpl

import "testing"

// An item of HTML leaves open nothing that would take in the page after it, as a browser reads it: foreign
// content, a comment, raw text, a table, a formatting element or a block, or a tag or a quote. A <p> or an
// <li> left open, which the page's markup after it closes, and void elements are fine.
func TestOpenIn(t *testing.T) {
	t.Parallel()

	for item, want := range map[string]string{
		"":                          "",
		"<b>bold</b> and <i>it</i>": "",
		"<p>one<p>two":              "",
		"<ul><li>a<li>b</ul>":       "",
		"<img src=x><br>":           "",
		"<svg><rect/></svg><math><mi>x</mi></math>": "",
		"<table><tr><td>a</table>":                  "",
		"<section>s</section>":                      "",
		"<svg><rect/>":                              "HTML leaves <svg> open",
		"<SVG>":                                     "HTML leaves <svg> open",
		"<math><mi>x":                               "HTML leaves <math> open",
		"<svg><foreignObject><div>x</div>":          "HTML leaves <svg> open",
		"a <!-- note":                               "HTML leaves <!-- open",
		"<textarea>x":                               "HTML leaves <textarea> open",
		"<title>x":                                  "HTML leaves <title> open",
		"<template><p>x":                            "HTML leaves <template> open",
		"<table><tr><td>a":                          "HTML leaves <table> open",
		"<select><option>a":                         "HTML leaves <select> open",
		"<b>bold":                                   "HTML leaves <b> open",
		"<a href=x>link":                            "HTML leaves <a> open",
		"<div>block":                                "HTML leaves <div> open",
		"<plaintext>x":                              "HTML leaves <plaintext> open",
		`<a title="x`:                               "HTML ends in a tag or a quote",
		"x <b":                                      "HTML ends in a tag or a quote",
		"x</div><p>y":                               "HTML closes an element it does not open",
		// A tag the item ends in takes in the mark.
		"x<section ":          "HTML ends in a tag or a quote",
		"<b>x</b><br title='": "HTML ends in a tag or a quote",
		// What holds the mark is told: the <svg>, not the <p> it is in, and the element the item leaves open
		// in a <p>, which the parser opens again after it.
		"<p>a<svg>":         "HTML leaves <svg> open",
		"<p><a href=x>link": "HTML leaves <a> open",
		"<h2>t</h2><p><b>x": "HTML leaves <b> open",
		"<b><div>x":         "HTML leaves <div> open",
		// An end tag of an element it does not open closes one of the page's, whichever it is.
		"x</section>":         "HTML closes an element it does not open",
		"<p>x</p></main>":     "HTML closes an element it does not open",
		"<ul><li>a</li></li>": "HTML closes an element it does not open",
		// A <li> or a <dd> left open is no <p>: a <section> does not close it.
		"<li>a": "HTML leaves <li> open",
	} {
		got := ""
		if err := openIn(item); err != nil {
			got = err.Error()
		}
		if want == "" && got != "" || len(got) < len(want) || got[:len(want)] != want {
			t.Errorf("openIn(%q) = %q, want %q", item, got, want)
		}
	}
}
