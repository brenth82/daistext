package settings

import (
	"encoding/json"
	"fmt"
	"github.com/brenth82/daistext/internal/export"
	"os"
	"path/filepath"
)

type Settings struct {
	AutoUpdate       bool           `json:"autoUpdate"`
	ShowZIPImport    bool           `json:"showZipImport"`
	ShowFolderImport bool           `json:"showFolderImport"`
	Format           string         `json:"format"`
	OutputDirectory  string         `json:"outputDirectory"`
	Tools            export.Options `json:"tools"`
}

func Default() Settings {
	return Settings{AutoUpdate: true, ShowZIPImport: true, ShowFolderImport: true, Format: "txt", OutputDirectory: DownloadsDirectory(), Tools: export.Options{CellsPerLine: 40, LinesPerPage: 25}}
}
func Path() (string, error) {
	dir, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "DaisText", "settings.json"), nil
}
func Load() (Settings, error) {
	s := Default()
	p, e := Path()
	if e != nil {
		return s, e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("invalid settings %s: %w", p, e)
	}
	if s.OutputDirectory == "" {
		s.OutputDirectory = DownloadsDirectory()
	}
	if !s.ShowZIPImport && !s.ShowFolderImport {
		s.ShowZIPImport = true
	}
	return s, nil
}
func Save(s Settings) error {
	if !s.ShowZIPImport && !s.ShowFolderImport {
		return fmt.Errorf("keep at least one Open option visible")
	}
	p, e := Path()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), "settings-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
