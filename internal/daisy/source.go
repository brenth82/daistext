package daisy

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

type resource struct {
	size uint64
	open func() (io.ReadCloser, error)
}

// Both import modes read in place. ZIP contents are never extracted to disk.
// os.Root keeps folder references inside the selected book, including symlinks.
func openSource(filename string) (*archive, func() error, error) {
	a := &archive{files: map[string]resource{}, parsed: map[string]*node{}}
	info, e := os.Stat(filename)
	if e != nil {
		return nil, nil, e
	}
	if info.IsDir() {
		root, e := os.OpenRoot(filename)
		if e != nil {
			return nil, nil, e
		}
		e = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic links are not supported in book folders: %s", name)
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsupported book resource: %s", name)
			}
			if len(a.files) >= 20000 {
				return fmt.Errorf("too many book resources")
			}
			a.files[name] = resource{uint64(info.Size()), func() (io.ReadCloser, error) { return root.Open(name) }}
			return nil
		})
		if e != nil {
			root.Close()
			return nil, nil, e
		}
		return a, root.Close, nil
	}
	z, e := zip.OpenReader(filename)
	if e != nil {
		return nil, nil, e
	}
	fail := func(err error) (*archive, func() error, error) { z.Close(); return nil, nil, err }
	if len(z.File) > 20000 {
		return fail(fmt.Errorf("too many archive entries"))
	}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(f.Name)
		if name != f.Name || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || f.Mode()&os.ModeSymlink != 0 {
			return fail(fmt.Errorf("unsafe archive path"))
		}
		if _, exists := a.files[name]; exists {
			return fail(fmt.Errorf("duplicate archive path: %s", name))
		}
		a.files[name] = resource{f.UncompressedSize64, f.Open}
	}
	return a, z.Close, nil
}
