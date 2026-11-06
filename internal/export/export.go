// Package export renders selected blocks and delegates PDF/BRF to explicit local tools.
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brenth82/daistext/internal/daisy"
)

type Options struct {
	TypstPath, LouisPath, BrailleTable string
	CellsPerLine, LinesPerPage         int
}

func Text(blocks []daisy.Block) string {
	var b strings.Builder
	for _, v := range blocks {
		b.WriteString(v.Text)
		b.WriteString("\r\n\r\n")
	}
	return b.String()
}

var mdEscape = strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#", "!", "\\!", "|", "\\|", "&", "&amp;", "~", "\\~")

func Markdown(blocks []daisy.Block) string {
	var b strings.Builder
	for _, v := range blocks {
		if v.Level > 0 {
			b.WriteString(strings.Repeat("#", v.Level) + " ")
		}
		s := mdEscape.Replace(v.Text)
		if v.Level == 0 {
			s = strings.ReplaceAll(s, ".", "\\.")
			s = strings.ReplaceAll(s, "-", "\\-")
		}
		b.WriteString(s + "\n\n")
	}
	return b.String()
}
func HTML(title, lang string, blocks []daisy.Block) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=\"%s\"><head><meta charset=\"utf-8\"><title>%s</title><meta name=\"viewport\" content=\"width=device-width\"><style>body{max-width:75ch;margin:2em auto;padding:0 1em;font:1.1em/1.6 system-ui}pre{white-space:pre-wrap}</style></head><body><main>\n", html.EscapeString(lang), html.EscapeString(title))
	for _, v := range blocks {
		tag := "p"
		if v.Level > 0 {
			tag = fmt.Sprintf("h%d", v.Level)
		}
		fmt.Fprintf(&b, "<%s>%s</%s>\n", tag, html.EscapeString(v.Text), tag)
	}
	b.WriteString("</main></body></html>\n")
	return b.String()
}
func tool(configured, name string) (string, error) {
	if configured != "" {
		p, e := filepath.Abs(configured)
		if e != nil {
			return "", e
		}
		if _, e = os.Stat(p); e != nil {
			return "", e
		}
		return p, nil
	}
	exe, _ := os.Executable()
	if name == "lou_translate" {
		for _, root := range []string{filepath.Join(filepath.Dir(exe), "tools", "liblouis"), filepath.Join(filepath.Dir(exe), "liblouis")} {
			bundled := filepath.Join(root, "bin", "lou_translate.exe")
			if _, e := os.Stat(bundled); e == nil {
				return bundled, nil
			}
		}
	}
	bundled := filepath.Join(filepath.Dir(exe), "tools", name+".exe")
	if _, e := os.Stat(bundled); e == nil {
		return bundled, nil
	}
	p, e := exec.LookPath(name)
	if e != nil {
		return "", fmt.Errorf("%s is not installed: configure its executable in Settings", name)
	}
	return p, nil
}

// Write stages output beside the destination so failed conversions never truncate existing files.
func Write(ctx context.Context, dest, format, title, lang string, blocks []daisy.Block, o Options) error {
	if len(blocks) == 0 {
		return fmt.Errorf("no text selected")
	}
	format = strings.ToLower(format)
	blocks = normalizeHeadings(blocks)
	tmp, e := os.CreateTemp(filepath.Dir(dest), ".daistext-*")
	if e != nil {
		return e
	}
	stage := tmp.Name()
	tmp.Close()
	defer os.Remove(stage)
	var data []byte
	switch format {
	case "txt":
		data = []byte(Text(blocks))
	case "md":
		data = []byte(Markdown(blocks))
	case "html":
		data = []byte(HTML(title, lang, blocks))
	case "pdf":
		p, e := tool(o.TypstPath, "typst")
		if e != nil {
			return e
		}
		cache, e := os.UserCacheDir()
		if e != nil {
			return e
		}
		root := filepath.Join(cache, "DaisText", "temp")
		if e = os.MkdirAll(root, 0700); e != nil {
			return e
		}
		// Abandoned work after a crash stays in an app-owned location so
		// uninstall can clean it without touching source or output folders.
		defer func() { os.Remove(root); os.Remove(filepath.Dir(root)) }()
		dir, e := os.MkdirTemp(root, "pdf-*")
		if e != nil {
			return e
		}
		defer os.RemoveAll(dir)
		if lang == "eng" {
			lang = "en"
		}
		payload, _ := json.Marshal(map[string]any{"title": title, "lang": lang, "blocks": blocks})
		if e = os.WriteFile(filepath.Join(dir, "book.json"), payload, 0600); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, "book.typ"), []byte(typstTemplate), 0600); e != nil {
			return e
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(cctx, p, "compile", "--format", "pdf", "--root", dir, "--pdf-standard", "ua-1", filepath.Join(dir, "book.typ"), stage)
		if out, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("PDF/UA export failed: %w: %s", e, out)
		}
		return replace(stage, dest)
	case "brf":
		p, e := tool(o.LouisPath, "lou_translate")
		if e != nil {
			return e
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		tables := filepath.Clean(filepath.Join(filepath.Dir(p), "..", "share", "liblouis", "tables"))
		args := []string{"--forward"}
		if o.BrailleTable != "" {
			args = append(args, o.BrailleTable)
		} else if _, e := os.Stat(filepath.Join(tables, "en-ueb-g2.ctb")); e == nil {
			args = append(args, "--display-table", filepath.Join(tables, "en-us-brf.dis"), filepath.Join(tables, "en-ueb-g2.ctb"))
		} else {
			args = append(args, "--display-table", "en-us-brf.dis", "en-ueb-g2.ctb")
		}
		cmd := exec.CommandContext(cctx, p, args...)
		if _, e := os.Stat(tables); e == nil {
			cmd.Env = append(os.Environ(), "LOUIS_TABLEPATH="+tables)
		}
		cmd.Stdin = strings.NewReader(Text(blocks))
		var stderr strings.Builder
		cmd.Stderr = &stderr
		raw, e := cmd.Output()
		if e != nil {
			return fmt.Errorf("Liblouis: %w: %s", e, stderr.String())
		}
		for _, c := range raw {
			if c != 10 && c != 13 && c != 12 && (c < 32 || c > 126) {
				return fmt.Errorf("translator did not produce ASCII BRF; include a BRF display table")
			}
		}
		data = []byte(paginate(string(raw), o.CellsPerLine, o.LinesPerPage))
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	if e = os.WriteFile(stage, data, 0600); e != nil {
		return e
	}
	return replace(stage, dest)
}

// Removed ancestors must not leave a document starting at heading level three,
// or a level jump in its accessible outline.
func normalizeHeadings(blocks []daisy.Block) []daisy.Block {
	out := append([]daisy.Block(nil), blocks...)
	var stack []int
	for i, b := range out {
		if b.Level == 0 {
			continue
		}
		for len(stack) > 0 && stack[len(stack)-1] >= b.Level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, b.Level)
		out[i].Level = len(stack)
	}
	return out
}
func replace(stage, dest string) error { return os.Rename(stage, dest) }
func paginate(s string, width, height int) string {
	if width < 10 {
		width = 40
	}
	if height < 5 {
		height = 25
	}
	s = strings.ReplaceAll(s, "\r", "")
	var b strings.Builder
	line := 0
	emit := func(s string) {
		if line == height {
			b.WriteByte('\f')
			line = 0
		}
		b.WriteString(s + "\r\n")
		line++
	}
	for _, p := range strings.Split(s, "\n") {
		p = strings.ReplaceAll(p, "\f", "")
		for len(p) > width {
			cut := strings.LastIndexByte(p[:width+1], ' ')
			if cut < 1 {
				cut = width
			}
			emit(p[:cut])
			p = strings.TrimLeft(p[cut:], " ")
		}
		emit(p)
	}
	return b.String()
}

const typstTemplate = `#let book = json("book.json")
#set document(title: book.title)
#set text(lang: book.lang, size: 12pt)
#set page(paper: "us-letter", margin: 1in, numbering: "1")
#set heading(numbering: none)
#for block in book.blocks {
  if block.Level > 0 { heading(level: block.Level, block.Text) }
  else { par(block.Text) }
}
`
