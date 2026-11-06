// Package daisy reads text-bearing DAISY 2.02 and 3 folders and ZIPs without extraction.
package daisy

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

const maxFile = 32 << 20
const maxTotal = 128 << 20

type Block struct {
	Text  string
	Level int
}

// Each section owns only its own blocks. Parent selections never duplicate children.
type Section struct {
	Title  string
	Depth  int
	Blocks []Block
}
type Book struct {
	Title, Language string
	Sections        []Section
	Warnings        []string
}
type node struct {
	tag      string
	attrs    map[string]string
	children []*node
	text     string
}

func (n *node) all(tag string) (out []*node) {
	if n.tag == tag {
		out = append(out, n)
	}
	for _, c := range n.children {
		out = append(out, c.all(tag)...)
	}
	return
}
func (n *node) content() string {
	var b strings.Builder
	var visit func(*node)
	visit = func(x *node) {
		if x.tag == "script" || x.tag == "style" || x.tag == "head" {
			return
		}
		if x.tag == "img" {
			b.WriteString(" " + x.attrs["alt"] + " ")
			return
		}
		switch x.tag {
		case "p", "li", "div", "td", "th", "tr", "br":
			b.WriteByte(' ')
		}
		b.WriteString(x.text)
		if x.tag == "br" {
			b.WriteByte(' ')
		}
		for _, c := range x.children {
			visit(c)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func decode(data []byte, isHTML bool) (*node, error) {
	// Honor an actual legacy declaration. For mislabeled UTF-8, repair only
	// invalid bytes so valid Unicode elsewhere in the book is preserved.
	decl := encodingDeclaration.FindSubmatch(data)
	legacy := len(decl) > 1 && (strings.EqualFold(string(decl[1]), "windows-1252") || strings.EqualFold(string(decl[1]), "iso-8859-1"))
	if isHTML && legacy {
		data, _ = charmap.Windows1252.NewDecoder().Bytes(data)
	} else if !legacy && !utf8.Valid(data) {
		var repaired bytes.Buffer
		for len(data) > 0 {
			r, n := utf8.DecodeRune(data)
			if r == utf8.RuneError && n == 1 {
				repaired.WriteRune(charmap.Windows1252.DecodeByte(data[0]))
			} else {
				repaired.Write(data[:n])
			}
			data = data[n:]
		}
		data = repaired.Bytes()
	}
	root := &node{attrs: map[string]string{}}
	stack := []*node{root}
	if isHTML {
		h, e := html.Parse(bytes.NewReader(data))
		if e != nil {
			return nil, e
		}
		var convert func(*html.Node) *node
		convert = func(h *html.Node) *node {
			n := &node{tag: strings.ToLower(h.Data), attrs: map[string]string{}}
			if h.Type == html.TextNode {
				n.tag = ""
				n.text = h.Data
			}
			for _, a := range h.Attr {
				n.attrs[strings.ToLower(a.Key)] = a.Val
			}
			for c := h.FirstChild; c != nil; c = c.NextSibling {
				n.children = append(n.children, convert(c))
			}
			return n
		}
		return convert(h), nil
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Entity = map[string]string{"nbsp": " ", "copy": "©", "reg": "®", "mdash": "—", "ndash": "–"}
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) {
		if strings.EqualFold(label, "windows-1252") || strings.EqualFold(label, "iso-8859-1") {
			return charmap.Windows1252.NewDecoder().Reader(r), nil
		}
		return nil, fmt.Errorf("unsupported XML encoding %q", label)
	}
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		switch v := t.(type) {
		case xml.StartElement:
			if len(stack) > 256 {
				return nil, fmt.Errorf("XML nesting exceeds limit")
			}
			n := &node{tag: strings.ToLower(v.Name.Local), attrs: map[string]string{}}
			for _, a := range v.Attr {
				n.attrs[strings.ToLower(a.Name.Local)] = a.Value
			}
			p := stack[len(stack)-1]
			p.children = append(p.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			p := stack[len(stack)-1]
			p.children = append(p.children, &node{text: string(v)})
		}
	}
	return root, nil
}

var encodingDeclaration = regexp.MustCompile(`(?i)^\s*<\?xml\s[^?]*encoding\s*=\s*["']([^"']+)["']`)

type archive struct {
	files  map[string]resource
	parsed map[string]*node
	total  int64
}

func (a *archive) read(name string) (*node, error) {
	if n := a.parsed[name]; n != nil {
		return n, nil
	}
	f, exists := a.files[name]
	if !exists {
		return nil, fmt.Errorf("missing book resource: %s", name)
	}
	if f.size > maxFile {
		return nil, fmt.Errorf("resource too large: %s", name)
	}
	r, e := f.open()
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := io.ReadAll(io.LimitReader(r, maxFile+1))
	if e != nil {
		return nil, e
	}
	a.total += int64(len(b))
	if len(b) > maxFile || a.total > maxTotal {
		return nil, fmt.Errorf("book exceeds safe text size limit")
	}
	n, e := decode(b, strings.HasSuffix(strings.ToLower(name), ".html") || strings.HasSuffix(strings.ToLower(name), ".htm"))
	if e != nil {
		return nil, fmt.Errorf("%s: %w", name, e)
	}
	a.parsed[name] = n
	return n, nil
}
func resolve(base, ref string) (string, string, error) {
	u, e := url.Parse(ref)
	if e != nil {
		return "", "", e
	}
	if u.IsAbs() || u.Host != "" || strings.Contains(u.Path, "\\") {
		return "", "", fmt.Errorf("external or invalid book reference: %s", ref)
	}
	p := base
	if u.Path != "" {
		p = path.Join(path.Dir(base), u.Path)
	}
	if strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return "", "", fmt.Errorf("reference escapes book")
	}
	return p, u.Fragment, nil
}
func find(n *node, id string) *node {
	if id == "" {
		return n
	}
	if n.attrs["id"] == id {
		return n
	}
	for _, c := range n.children {
		if v := find(c, id); v != nil {
			return v
		}
	}
	return nil
}
func blocks(n *node) []Block {
	var out []Block
	var walk func(*node)
	walk = func(n *node) {
		if n.tag == "head" || n.tag == "script" || n.tag == "style" {
			return
		}
		level := 0
		if len(n.tag) == 2 && n.tag[0] == 'h' && n.tag[1] >= '1' && n.tag[1] <= '6' {
			level = int(n.tag[1] - '0')
		}
		switch n.tag {
		case "p", "li", "dt", "dd", "caption", "pagenum", "doctitle", "blockquote", "pre", "tr", "note", "prodnote", "line", "hd":
			if s := n.content(); s != "" {
				out = append(out, Block{s, 0})
			}
			return
		case "img":
			if s := n.attrs["alt"]; s != "" {
				out = append(out, Block{s, 0})
			}
			return
		}
		if level > 0 {
			out = append(out, Block{n.content(), level})
			return
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(n)
	if len(out) == 0 && n.tag != "head" && n.tag != "script" && n.tag != "style" {
		if text := n.content(); text != "" {
			out = append(out, Block{Text: text})
		}
	}
	return out
}

type nav struct {
	title, ref string
	depth      int
}

func Open(filename string) (*Book, error) {
	a, closeSource, e := openSource(filename)
	if e != nil {
		return nil, e
	}
	defer closeSource()
	var opfs, nccs []string
	for name := range a.files {
		switch strings.ToLower(path.Ext(name)) {
		case ".opf":
			opfs = append(opfs, name)
		}
		if strings.EqualFold(path.Base(name), "ncc.html") {
			nccs = append(nccs, name)
		}
	}
	sort.Strings(opfs)
	sort.Strings(nccs)
	b := &Book{Language: "en"}
	var navigation []nav
	var smils []string
	if len(opfs) > 1 {
		return nil, fmt.Errorf("multiple packages found; select a single-book folder or ZIP")
	}
	if len(opfs) == 1 {
		p := opfs[0]
		root, e := a.read(p)
		if e != nil {
			return nil, e
		}
		if ns := root.all("title"); len(ns) > 0 {
			b.Title = ns[0].content()
		}
		if ns := root.all("language"); len(ns) > 0 {
			b.Language = ns[0].content()
		}
		items := map[string]string{}
		var ncx string
		for _, n := range root.all("item") {
			f, _, e := resolve(p, n.attrs["href"])
			if e != nil {
				return nil, e
			}
			items[n.attrs["id"]] = f
			if strings.EqualFold(path.Ext(f), ".ncx") {
				ncx = f
			}
		}
		for _, n := range root.all("itemref") {
			if f := items[n.attrs["idref"]]; strings.EqualFold(path.Ext(f), ".smil") {
				smils = append(smils, f)
			}
		}
		if ncx != "" {
			n, e := a.read(ncx)
			if e != nil {
				return nil, e
			}
			var visit func(*node, int) error
			visit = func(n *node, depth int) error {
				if n.tag == "navpoint" {
					label := n.all("navlabel")
					for _, c := range n.children {
						if c.tag == "content" && len(label) > 0 {
							f, id, err := resolve(ncx, c.attrs["src"])
							if err != nil {
								return err
							}
							navigation = append(navigation, nav{label[0].content(), f + "#" + id, depth})
						}
					}
					depth++
				}
				for _, c := range n.children {
					if err := visit(c, depth); err != nil {
						return err
					}
				}
				return nil
			}
			for _, m := range n.all("navmap") {
				if err := visit(m, 0); err != nil {
					return nil, err
				}
			}
		}
	} else if len(nccs) == 1 {
		p := nccs[0]
		n, e := a.read(p)
		if e != nil {
			return nil, e
		}
		if t := n.all("title"); len(t) > 0 {
			b.Title = t[0].content()
		}
		var visit func(*node) error
		visit = func(n *node) error {
			if len(n.tag) == 2 && n.tag[0] == 'h' && n.tag[1] >= '1' && n.tag[1] <= '6' {
				for _, link := range n.all("a") {
					f, id, e := resolve(p, link.attrs["href"])
					if e != nil {
						return e
					}
					navigation = append(navigation, nav{n.content(), f + "#" + id, int(n.tag[1] - '1')})
					if len(smils) == 0 || smils[len(smils)-1] != f {
						smils = append(smils, f)
					}
				}
				return nil
			}
			for _, c := range n.children {
				if e := visit(c); e != nil {
					return e
				}
			}
			return nil
		}
		if e := visit(n); e != nil {
			return nil, e
		}
	} else {
		return nil, fmt.Errorf("no DAISY OPF or ncc.html found")
	}
	if len(smils) == 0 {
		return nil, fmt.Errorf("no text-bearing SMIL spine; audio-only DAISY is unsupported")
	}
	var timeline []Block
	positions := map[string]int{}
	seen := map[string]bool{}
	for _, s := range smils {
		root, e := a.read(s)
		if e != nil {
			return nil, e
		}
		texts := root.all("text")
		var visit func(*node) error
		visit = func(n *node) error {
			if id := n.attrs["id"]; id != "" {
				positions[s+"#"+id] = len(timeline)
			}
			if n.tag == "text" {
				f, id, e := resolve(s, n.attrs["src"])
				if e != nil {
					return e
				}
				key := f + "#" + id
				if seen[key] {
					return nil
				}
				seen[key] = true
				positions[key] = len(timeline)
				doc, e := a.read(f)
				if e != nil {
					return e
				}
				target := find(doc, id)
				if target == nil {
					return fmt.Errorf("missing text anchor %s", key)
				}
				// Clip a container reference at the next SMIL text target, avoiding ancestor duplication.
				var next *node
				for i, t := range texts {
					if t == n && i+1 < len(texts) {
						nf, ni, _ := resolve(s, texts[i+1].attrs["src"])
						if nf == f {
							next = find(doc, ni)
						}
						break
					}
				}
				if next != nil {
					var clone func(*node) *node
					stopped := false
					clone = func(x *node) *node {
						if x == next {
							stopped = true
						}
						if stopped {
							return nil
						}
						c := *x
						c.children = nil
						for _, ch := range x.children {
							if v := clone(ch); v != nil {
								c.children = append(c.children, v)
							}
						}
						return &c
					}
					if c := clone(target); c != nil {
						timeline = append(timeline, blocks(c)...)
					}
				} else {
					timeline = append(timeline, blocks(target)...)
				}
				return nil
			}
			for _, c := range n.children {
				if e := visit(c); e != nil {
					return e
				}
			}
			return nil
		}
		positions[s+"#"] = len(timeline)
		if e := visit(root); e != nil {
			return nil, e
		}
	}
	if len(timeline) == 0 {
		return nil, fmt.Errorf("book has no extractable text (audio-only books require transcription)")
	}
	type boundary struct {
		nav
		start int
	}
	var bounds []boundary
	for _, n := range navigation {
		i, ok := positions[n.ref]
		if !ok {
			return nil, fmt.Errorf("unresolved navigation target %s", n.ref)
		}
		bounds = append(bounds, boundary{n, i})
	}
	sort.SliceStable(bounds, func(i, j int) bool { return bounds[i].start < bounds[j].start })
	if len(bounds) == 0 || bounds[0].start > 0 {
		b.Sections = append(b.Sections, Section{"Front matter", 0, append([]Block(nil), timeline[:func() int {
			if len(bounds) > 0 {
				return bounds[0].start
			}
			return len(timeline)
		}()]...)})
	}
	for i, n := range bounds {
		end := len(timeline)
		if i+1 < len(bounds) {
			end = bounds[i+1].start
		}
		b.Sections = append(b.Sections, Section{n.title, n.depth, append([]Block(nil), timeline[n.start:end]...)})
	}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(path.Base(filename), path.Ext(filename))
	}
	return b, nil
}

func (b *Book) Selected(indices []int) ([]Block, error) {
	chosen := map[int]bool{}
	for _, i := range indices {
		if i < 0 || i >= len(b.Sections) {
			return nil, fmt.Errorf("section index out of range: %d", i)
		}
		chosen[i] = true
	}
	var out []Block
	for i, s := range b.Sections {
		if chosen[i] {
			out = append(out, s.Blocks...)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("select at least one section containing text")
	}
	return out, nil
}
