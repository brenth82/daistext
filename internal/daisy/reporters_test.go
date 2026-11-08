package daisy

import (
	"reflect"
	"strings"
	"testing"
)

func TestReporterCleanupOnNewspaperArticles(t *testing.T) {
	for _, newspaper := range []bool{false, true} {
		files := synthetic()
		files["book.ncx"] = strings.Replace(files["book.ncx"], "<navPoint><navLabel><text>First", `<navPoint class="article"><navLabel><text>First`, 1)
		files["book.xml"] = strings.Replace(files["book.xml"], "Alpha &amp; omega.</p>",
			`Alpha &amp; omega.</p><p id="correction">Correction: The original article misstated the date.</p><p id="bio">Jane Example is a staff reporter for The Daily Example. She can be reached at jane@example.com.</p><p id="label">Local .</p>`, 1)
		files["book.smil"] = strings.Replace(files["book.smil"], `<par id="two">`, `<par><text src="book.xml#correction"/></par><par><text src="book.xml#bio"/></par><par><text src="book.xml#label"/></par><par id="two">`, 1)
		if newspaper {
			files["book.xml"] = strings.Replace(files["book.xml"], "<dtbook>", `<dtbook><head><meta name="Publication.Type" content="Newspaper"/></head>`, 1)
		}
		book, err := Open(fixture(t, files))
		if err != nil {
			t.Fatal(err)
		}
		original, err := book.Selected([]int{1})
		if err != nil {
			t.Fatal(err)
		}
		cleaned, err := book.SelectedForExport([]int{1}, true)
		if err != nil {
			t.Fatal(err)
		}
		want := original
		if newspaper {
			want = []Block{{"First", 2}, {"Alpha & omega.", 0}, {"Correction: The original article misstated the date.", 0}, {"Local .", 0}}
		}
		if !reflect.DeepEqual(cleaned, want) || !reflect.DeepEqual(cleaned, book.SectionBlocks(1, true)) {
			t.Fatalf("newspaper=%v: got %+v, want %+v", newspaper, cleaned, want)
		}
		if !reflect.DeepEqual(original, book.SectionBlocks(1, false)) {
			t.Fatal("cleanup changed the original section")
		}
	}
}

func TestReporterCleanupOnlyRemovesTrailingDetails(t *testing.T) {
	blocks := []Block{
		{"Contact Jane Example at jane@example.com.", 0},
		{"The story continues here.", 0},
		{"Jane Example", 0},
		{"Guest columnist", 0},
		{"DAILY EXAMPLE NETWORK", 0},
		{"Local .", 0},
	}
	want := []Block{blocks[0], blocks[1], blocks[5]}
	if got := withoutReporterFooter(blocks); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
