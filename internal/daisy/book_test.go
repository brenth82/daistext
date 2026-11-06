package daisy

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "book.zip")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for name, s := range files {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(s)); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	f.Close()
	return p
}
func synthetic() map[string]string {
	return map[string]string{
		"book.opf":  `<package><metadata><Title>Test Book</Title><Language>en</Language></metadata><manifest><item id="nav" href="book.ncx"/><item id="s" href="book.smil"/></manifest><spine><itemref idref="s"/></spine></package>`,
		"book.ncx":  `<ncx><navMap><navPoint><navLabel><text>Edition</text></navLabel><content src="book.smil#edition"/><navPoint><navLabel><text>First</text></navLabel><content src="book.smil#one"/></navPoint><navPoint><navLabel><text>Second</text></navLabel><content src="book.smil#two"/></navPoint></navPoint></navMap></ncx>`,
		"book.smil": `<smil><body><seq><par id="edition"><text src="book.xml#e"/></par><par><text src="book.xml#intro"/></par><par id="one"><text src="book.xml#a"/></par><par><text src="book.xml#p1"/></par><par id="two"><text src="book.xml#b"/></par><par><text src="book.xml#p2"/></par></seq></body></smil>`,
		"book.xml":  `<dtbook><book><level1 id="e"><h1>Edition</h1><p id="intro">Intro</p><level2 id="a"><h2>First</h2><p id="p1">Alpha &amp; omega.</p></level2><level2 id="b"><h2>Second</h2><p id="p2">Beta.</p></level2></level1></book></dtbook>`}
}

func TestFolderAndZIPImportMatch(t *testing.T) {
	files := synthetic()
	dir := t.TempDir()
	for name, s := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before, _ := os.ReadDir(dir)
	folder, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	zipped, e := Open(fixture(t, files))
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(folder, zipped) {
		t.Fatal("folder and ZIP produced different books")
	}
	after, _ := os.ReadDir(dir)
	if len(before) != len(after) {
		t.Fatal("import wrote files into the source folder")
	}
}

func TestFolderRejectsExternalLinks(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.xml")
	if e := os.WriteFile(outside, []byte("outside"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(dir, "linked.xml")); e != nil {
		t.Skipf("symlinks unavailable: %v", e)
	}
	if _, e := Open(dir); e == nil {
		t.Fatal("accepted a linked external file")
	}
}
func TestHierarchySelectionAndNoDuplicates(t *testing.T) {
	b, e := Open(fixture(t, synthetic()))
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Sections) != 3 {
		t.Fatalf("%+v", b.Sections)
	}
	for _, s := range b.Sections {
		if len(s.Blocks) != 2 {
			t.Fatalf("duplicate/missing blocks: %+v", s)
		}
	}
	out, e := b.Selected([]int{2, 0, 2})
	if e != nil {
		t.Fatal(e)
	}
	var text string
	for _, p := range out {
		text += p.Text + "|"
	}
	if text != "Edition|Intro|Second|Beta.|" {
		t.Fatal(text)
	}
	if _, e = b.Selected(nil); e == nil {
		t.Fatal("empty selection accepted")
	}
	if _, e = b.Selected([]int{99}); e == nil {
		t.Fatal("invalid index accepted")
	}
}
func TestRejectTraversalAndExternalResources(t *testing.T) {
	for _, ref := range []string{"../../escape.xml#x", "https://evil.example/a.xml#x"} {
		files := synthetic()
		files["book.smil"] = `<smil><text src="` + ref + `"/></smil>`
		if _, e := Open(fixture(t, files)); e == nil {
			t.Fatalf("accepted %s", ref)
		}
	}
	if _, e := Open(fixture(t, map[string]string{"../escape": "bad"})); e == nil {
		t.Fatal("accepted ZIP traversal")
	}
}
func TestDaisy202(t *testing.T) {
	p := fixture(t, map[string]string{"ncc.html": `<html><head><title>Older book</title></head><body><h1><a href="a.smil#p">First</a></h1><h1><a href="b.smil#p">Second</a></h1></body></html>`, "a.smil": `<smil><par id="p"><text src="text.html#a"/></par></smil>`, "b.smil": `<smil><par id="p"><text src="text.html#b"/></par></smil>`, "text.html": `<html><body><p id="a">one</p><p id="b">two</p></body></html>`})
	b, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	if b.Title != "Older book" || len(b.Sections) != 2 || b.Sections[1].Blocks[0].Text != "two" {
		t.Fatalf("%+v", b)
	}
}

func TestInlineTextReferenceAndTableText(t *testing.T) {
	n, e := decode([]byte(`<html><head><title>Hidden title</title></head><body><p><span id="sentence">A sentence with <em>emphasis</em>.</span></p><table><tr><td>First</td><td>Second</td></tr></table></body></html>`), true)
	if e != nil {
		t.Fatal(e)
	}
	inline := blocks(find(n, "sentence"))
	if len(inline) != 1 || inline[0].Text != "A sentence with emphasis." {
		t.Fatalf("%+v", inline)
	}
	all := blocks(n)
	if len(all) != 2 || all[1].Text != "First Second" {
		t.Fatalf("%+v", all)
	}
}

func TestLegacyAndMislabeledEncoding(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?><p>caf\xe9</p>"),
		[]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?><p>caf\xe9</p>"),
	} {
		n, e := decode(data, false)
		if e != nil {
			t.Fatal(e)
		}
		if n.content() != "café" {
			t.Fatal(n.content())
		}
	}
	n, e := decode([]byte("<p>Unicode – and legacy caf\xe9</p>"), false)
	if e != nil || n.content() != "Unicode – and legacy café" {
		t.Fatalf("%s: %v", n.content(), e)
	}
}
func TestLocalBook(t *testing.T) {
	p := os.Getenv("DAISTEXT_TEST_BOOK")
	if p == "" {
		t.Skip("set DAISTEXT_TEST_BOOK for a private local integration test")
	}
	b, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	var ids []int
	for i, s := range b.Sections {
		if len(s.Blocks) > 0 {
			ids = append(ids, i)
		}
	}
	if len(ids) < 2 {
		t.Fatal("local integration book needs at least two sections with text")
	}
	first, last := ids[0], ids[len(ids)-1]
	out, e := b.Selected([]int{last, first})
	if e != nil {
		t.Fatal(e)
	}
	expected := append(append([]Block(nil), b.Sections[first].Blocks...), b.Sections[last].Blocks...)
	if !reflect.DeepEqual(out, expected) {
		t.Fatal("selection changed reading order or included omitted content")
	}
	if folder := os.Getenv("DAISTEXT_TEST_FOLDER"); folder != "" {
		other, e := Open(folder)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(b, other) {
			t.Fatal("ZIP and folder import differ")
		}
	}
}
