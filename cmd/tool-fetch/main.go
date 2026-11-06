// tool-fetch stages pinned third-party build tools inside .tools, never globally.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	must(os.MkdirAll(".tools", 0700))
	data, e := os.ReadFile("scripts/tools.lock.json")
	must(e)
	var lock map[string]struct {
		URL string `json:"url"`
		SHA string `json:"sha256"`
	}
	must(json.Unmarshal(data, &lock))
	client := &http.Client{Timeout: 5 * time.Minute}
	for _, name := range []string{"typst", "inno", "liblouis", "liblouis-source"} {
		v := lock[name]
		ext := ".exe"
		if name != "inno" {
			ext = ".zip"
		}
		dest := filepath.Join(".tools", name+ext)
		b, _ := os.ReadFile(dest)
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != v.SHA {
			fmt.Println("Downloading pinned", name)
			r, e := client.Get(v.URL)
			must(e)
			if r.StatusCode != 200 {
				r.Body.Close()
				must(fmt.Errorf("%s: %s", name, r.Status))
			}
			b, e = io.ReadAll(io.LimitReader(r.Body, 256<<20))
			r.Body.Close()
			must(e)
			h = sha256.Sum256(b)
			if hex.EncodeToString(h[:]) != v.SHA {
				must(fmt.Errorf("%s checksum mismatch", name))
			}
			must(os.WriteFile(dest, b, 0600))
		}
		if name == "typst" {
			z, e := zip.OpenReader(dest)
			must(e)
			must(os.MkdirAll(".tools/typst", 0700))
			for _, f := range z.File {
				base := filepath.Base(f.Name)
				if base != "typst.exe" && !strings.HasPrefix(strings.ToUpper(base), "LICENSE") && !strings.HasPrefix(strings.ToUpper(base), "NOTICE") {
					continue
				}
				r, e := f.Open()
				must(e)
				out, e := os.Create(filepath.Join(".tools/typst", base))
				must(e)
				_, e = io.Copy(out, io.LimitReader(r, 128<<20))
				must(e)
				must(out.Close())
				r.Close()
			}
			z.Close()
		}
		if name == "liblouis" {
			must(extract(dest, ".tools/liblouis"))
		}
		if name == "liblouis-source" {
			z, e := zip.OpenReader(dest)
			must(e)
			must(os.MkdirAll(".tools/liblouis/licenses", 0700))
			for _, f := range z.File {
				base := filepath.Base(f.Name)
				if base != "COPYING" && base != "COPYING.LESSER" && base != "README.windows" && base != "HACKING" {
					continue
				}
				r, e := f.Open()
				must(e)
				data, e := io.ReadAll(r)
				r.Close()
				must(e)
				must(os.WriteFile(filepath.Join(".tools/liblouis/licenses", base), data, 0600))
			}
			z.Close()
		}
		fmt.Println("Verified", name)
	}
}

func extract(filename, root string) error {
	z, e := zip.OpenReader(filename)
	if e != nil {
		return e
	}
	defer z.Close()
	for _, f := range z.File {
		rel := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(rel) || strings.Contains(f.Name, "\\") || f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe tool archive path: %s", f.Name)
		}
		dest := filepath.Join(root, rel)
		if f.FileInfo().IsDir() {
			if e := os.MkdirAll(dest, 0700); e != nil {
				return e
			}
			continue
		}
		if f.UncompressedSize64 > 128<<20 {
			return fmt.Errorf("tool resource exceeds size limit")
		}
		if e := os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		r, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.Create(dest)
		if e != nil {
			r.Close()
			return e
		}
		_, copyErr := io.Copy(out, io.LimitReader(r, (128<<20)+1))
		closeErr := out.Close()
		r.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
