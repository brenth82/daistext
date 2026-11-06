package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/brenth82/daistext/internal/daisy"
	"github.com/brenth82/daistext/internal/export"
	"github.com/brenth82/daistext/internal/settings"
	"os"
	"strconv"
	"strings"
)

func main() {
	input := flag.String("input", "", "DAISY folder or ZIP")
	out := flag.String("output", "", "destination file")
	format := flag.String("format", "txt", "txt, md, html, pdf, brf")
	list := flag.Bool("list", false, "list sections with zero-based indices")
	selection := flag.String("sections", "all", "comma-separated indices (output remains in book order)")
	flag.Parse()
	b, e := daisy.Open(*input)
	must(e)
	if *list {
		fmt.Println(b.Title)
		for i, s := range b.Sections {
			fmt.Printf("%d\t%s%s (%d blocks)\n", i, strings.Repeat("  ", s.Depth), s.Title, len(s.Blocks))
		}
		return
	}
	var ids []int
	if *selection == "all" {
		for i := range b.Sections {
			ids = append(ids, i)
		}
	} else {
		for _, s := range strings.Split(*selection, ",") {
			n, e := strconv.Atoi(strings.TrimSpace(s))
			must(e)
			ids = append(ids, n)
		}
	}
	blocks, e := b.Selected(ids)
	must(e)
	s, e := settings.Load()
	must(e)
	if *out == "" {
		must(fmt.Errorf("provide -output"))
	}
	must(export.Write(context.Background(), *out, *format, b.Title, b.Language, blocks, s.Tools))
	fmt.Println("Saved", *out)
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
