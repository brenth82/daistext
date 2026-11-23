package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func installerTestCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	dir := filepath.Join(cache, installerCache)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeInstallerFixture(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test installer"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanupInstallers(t *testing.T) {
	dir := installerTestCache(t)
	old := writeInstallerFixture(t, dir, "DaisText-Setup-100.exe")
	stale := time.Now().Add(-2 * installerStaleAge)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	completed := writeInstallerFixture(t, dir, "DaisText-Setup-200.exe")
	inProgress := writeInstallerFixture(t, dir, "DaisText-Setup-300.exe")
	unrelated := writeInstallerFixture(t, dir, "DaisText-Setup-manual.exe")
	if err := os.Chtimes(unrelated, stale, stale); err != nil {
		t.Fatal(err)
	}
	outside := writeInstallerFixture(t, t.TempDir(), filepath.Base(inProgress))
	subdir := filepath.Join(dir, "DaisText-Setup-400.exe")
	if err := os.Mkdir(subdir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(subdir, stale, stale); err != nil {
		t.Fatal(err)
	}

	CleanupInstallers(context.Background(), outside)
	CleanupInstallers(context.Background(), completed)
	for _, name := range []string{old, completed} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat error: %v", name, err)
		}
	}
	for _, name := range []string{inProgress, unrelated, outside, subdir} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s to be kept: %v", name, err)
		}
	}
}
