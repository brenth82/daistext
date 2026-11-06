package export

import (
	"context"
	"github.com/brenth82/daistext/internal/daisy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscaping(t *testing.T) {
	blocks := []daisy.Block{{Text: "<script>alert(1)</script> & **bold**", Level: 2}}
	h := HTML(`a"b`, "en", blocks)
	if strings.Contains(h, "<script>") || !strings.Contains(h, "<h2>&lt;script&gt;") {
		t.Fatal(h)
	}
	m := Markdown(blocks)
	if !strings.Contains(m, `\*\*bold\*\*`) {
		t.Fatal(m)
	}
}
func TestFailedConversionPreservesOutput(t *testing.T) {
	p := filepath.Join(t.TempDir(), "book.txt")
	os.WriteFile(p, []byte("original"), 0600)
	if e := Write(context.Background(), p, "unknown", "", "en", []daisy.Block{{Text: "hello"}}, Options{}); e == nil {
		t.Fatal("accepted unknown format")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "original" {
		t.Fatal("destination overwritten on failure")
	}
	if e := Write(context.Background(), p, "txt", "", "en", []daisy.Block{{Text: "new text"}}, Options{}); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(p)
	if string(b) != "new text\r\n\r\n" {
		t.Fatal(string(b))
	}
}
func TestBRFPagination(t *testing.T) {
	s := paginate("ABCDEFGHIJK\nsecond\nthird\nfourth\nfifth\nsixth", 10, 5)
	if !strings.Contains(s, "ABCDEFGHIJ\r\nK\r\n") || !strings.Contains(s, "\f") {
		t.Fatal(s)
	}
}

func TestPDFUAIntegration(t *testing.T) {
	typst := os.Getenv("DAISTEXT_TEST_TYPST")
	if typst == "" {
		t.Skip("set DAISTEXT_TEST_TYPST to test the real PDF engine")
	}
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	dest := filepath.Join(t.TempDir(), "accessible.pdf")
	err := Write(context.Background(), dest, "pdf", "Test document", "en", []daisy.Block{{Text: "Selected article", Level: 3}, {Text: `Literal # and [brackets] and a quote " remain text.`}}, Options{TypstPath: typst})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || !strings.HasPrefix(string(data), "%PDF-") {
		t.Fatalf("no PDF output: %v", err)
	}
	if !strings.Contains(string(data), "/StructTreeRoot") {
		t.Fatal("PDF has no structure tree")
	}
}

func TestHeadingNormalization(t *testing.T) {
	in := []daisy.Block{{Level: 1}, {Level: 3}, {Level: 3}, {Level: 2}, {Level: 4}}
	out := normalizeHeadings(in)
	for i, want := range []int{1, 2, 2, 2, 3} {
		if out[i].Level != want {
			t.Fatalf("heading %d: %d", i, out[i].Level)
		}
	}
	if in[1].Level != 3 {
		t.Fatal("normalization mutated source")
	}
}

func TestLiblouisIntegration(t *testing.T) {
	louis := os.Getenv("DAISTEXT_TEST_LOUIS")
	if louis == "" {
		t.Skip("set DAISTEXT_TEST_LOUIS to test the real translator")
	}
	dest := filepath.Join(t.TempDir(), "braille.brf")
	e := Write(context.Background(), dest, "brf", "Test", "en", []daisy.Block{{Text: "The quick brown fox jumps over the lazy dog."}}, Options{LouisPath: louis, CellsPerLine: 40, LinesPerPage: 25})
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(dest)
	if e != nil {
		t.Fatal(e)
	}
	if len(b) == 0 || strings.Contains(string(b), "The quick brown fox") {
		t.Fatal("translation was empty or not braille")
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if len(line) > 40 {
			t.Fatal("line exceeds configured cell width")
		}
	}
	for _, c := range b {
		if c != 10 && c != 13 && c != 12 && (c < 32 || c > 126) {
			t.Fatal("BRF contains non-ASCII content")
		}
	}
}

func TestCanceledPDFLeavesNoTemporarySource(t *testing.T) {
	typst := os.Getenv("DAISTEXT_TEST_TYPST")
	if typst == "" {
		t.Skip("set DAISTEXT_TEST_TYPST to test cancellation")
	}
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	t.Setenv("LOCALAPPDATA", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)
	out := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := Write(ctx, filepath.Join(out, "book.pdf"), "pdf", "Test", "en", []daisy.Block{{Text: "Private synthetic text."}}, Options{TypstPath: typst})
	if e == nil {
		t.Fatal("canceled conversion succeeded")
	}
	for _, dir := range []string{tmp, out} {
		files, e := os.ReadDir(dir)
		if e != nil {
			t.Fatal(e)
		}
		if len(files) != 0 {
			t.Fatalf("temporary files remained after cancellation in %s", dir)
		}
	}
}
