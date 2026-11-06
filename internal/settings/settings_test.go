package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func isolatedConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestDefaultsAndLegacySettings(t *testing.T) {
	isolatedConfig(t)
	s, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if !s.ShowZIPImport || !s.ShowFolderImport || s.OutputDirectory != DownloadsDirectory() || !filepath.IsAbs(s.OutputDirectory) {
		t.Fatalf("bad defaults: %+v", s)
	}
	p, e := Path()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, []byte(`{"autoUpdate":false,"outputDirectory":""}`), 0600); e != nil {
		t.Fatal(e)
	}
	s, e = Load()
	if e != nil {
		t.Fatal(e)
	}
	if s.AutoUpdate || !s.ShowZIPImport || !s.ShowFolderImport || s.OutputDirectory != DownloadsDirectory() {
		t.Fatalf("legacy settings migration failed: %+v", s)
	}
}

func TestPreferencesPersist(t *testing.T) {
	isolatedConfig(t)
	s := Default()
	s.ShowZIPImport = false
	s.AutoUpdate = false
	s.OutputDirectory = t.TempDir()
	if e := Save(s); e != nil {
		t.Fatal(e)
	}
	got, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if got.ShowZIPImport || got.AutoUpdate || got.OutputDirectory != s.OutputDirectory {
		t.Fatalf("preferences did not persist: %+v", got)
	}
}

func TestImportOptionsCannotBothBeHidden(t *testing.T) {
	isolatedConfig(t)
	s := Default()
	s.ShowZIPImport = false
	s.ShowFolderImport = false
	if e := Save(s); e == nil {
		t.Fatal("saved inaccessible import controls")
	}
	p, _ := Path()
	os.MkdirAll(filepath.Dir(p), 0700)
	if e := os.WriteFile(p, []byte(`{"showZipImport":false,"showFolderImport":false}`), 0600); e != nil {
		t.Fatal(e)
	}
	got, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if !got.ShowZIPImport && !got.ShowFolderImport {
		t.Fatal("manually edited settings hid all imports")
	}
	s.ShowZIPImport = true
	if e := Save(s); e != nil {
		t.Fatal(e)
	}
	got, e = Load()
	if e != nil {
		t.Fatal(e)
	}
	if got.ShowFolderImport || !got.ShowZIPImport {
		t.Fatal("ZIP-only preference did not persist")
	}
}
