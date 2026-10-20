package cli_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIcons(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	root := filepath.Join(e.vars["HOME"], ".local", "share", "floe-icons")
	dir := filepath.Join(root, "aws")
	want(t, "search before a fetch", e.run(t, "", "icons", "search", "lambda"), 1, "",
		"floe: no icons are in "+root+`; fetch a set with "floe icons fetch <set>" (aws) and try again`)

	zipFile := filepath.Join(e.tmp, "Icon-package.zip")
	f, err := os.Create(zipFile)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{
		"Architecture-Service-Icons_01022030/Arch_Compute/48/Arch_AWS-Lambda_48.svg",
		"Architecture-Group-Icons_01022030/Region_32.svg",
	} {
		w, cerr := zw.Create(name)
		if cerr == nil {
			_, cerr = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
		}
		if cerr != nil {
			t.Fatal(cerr)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	want(t, "fetch", e.run(t, "", "icons", "fetch", "aws", "Icon-package.zip"), 0, "fetched 2 AWS icons (01022030) into "+dir+"\n", "")
	want(t, "fetch again", e.run(t, "", "icons", "fetch", "aws", "Icon-package.zip"), 0,
		"AWS icons 01022030 are already in "+dir+" (add --force to fetch them again)\n", "")
	want(t, "search", e.run(t, "", "icons", "search", "aws", "Lambda"), 0, "aws/aws-lambda\tCompute\n", "")
	want(t, "fetch an unknown set", e.run(t, "", "icons", "fetch", "gcp"), 2, "",
		`floe: unknown icon set "gcp"; use aws`)
	want(t, "fetch without a set", e.run(t, "", "icons", "fetch"), 2, "", "floe: accepts between 1 and 2 arg(s), received 0")
	want(t, "search nothing", e.run(t, "", "icons", "search"), 2, "", "floe: requires at least 1 arg(s)")

	// render draws icons from there, and names close ones for a name not there.
	e.write(t, "icon.yaml", "id: i\nwidth: 100\nheight: 100\ntitle: t\nparts:\n  - icon: { x: 10, y: 10, name: aws/lambda }\n")
	want(t, "render an unknown icon", e.run(t, "", "diagram", "render", "icon.yaml"), 1, "",
		`icon.yaml:6:5: icon "aws/lambda": no icon "aws/lambda"; did you mean aws/aws-lambda? (search with: floe icons search <word>)`)
	e.write(t, "icon.yaml", "id: i\nwidth: 100\nheight: 100\ntitle: t\nparts:\n  - icon: { x: 10, y: 10, name: aws/aws-lambda }\n")
	if r := e.run(t, "", "diagram", "render", "--no-assets", "icon.yaml"); r.code != 0 || !strings.Contains(r.stdout, `href="data:image/svg+xml;base64,`) || !strings.Contains(r.stdout, `<use href="#i_icon-`) {
		t.Errorf("render an icon = %+v", r)
	}

	// FLOE_ICONS_DIR moves them: the aws example draws with the stand-ins of the diagram tests.
	stand, err := filepath.Abs("../diagram/testdata/icons")
	if err != nil {
		t.Fatal(err)
	}
	e.vars["FLOE_ICONS_DIR"] = stand
	r := e.run(t, "", "diagram", "example", "aws")
	e.write(t, "aws.yaml", r.stdout)
	if r := e.run(t, "", "diagram", "render", "aws.yaml"); r.code != 0 || strings.Count(r.stdout, "<image ") != 6 {
		t.Errorf("render of the aws example = %+v, want 6 images", r)
	}
}
