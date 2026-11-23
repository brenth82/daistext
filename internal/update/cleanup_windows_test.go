package update

import (
	"context"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestCleanupRetriesLockedInstaller(t *testing.T) {
	dir := installerTestCache(t)
	completed := writeInstallerFixture(t, dir, "DaisText-Setup-100.exe")
	path, err := windows.UTF16PtrFromString(completed)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		CleanupInstallers(ctx, completed)
	}()
	select {
	case <-done:
		t.Fatal("cleanup returned before the installer lock was released")
	case <-time.After(100 * time.Millisecond):
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	handle = 0
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("cleanup did not retry after the installer lock was released")
	}
	if _, err := os.Stat(completed); !os.IsNotExist(err) {
		t.Fatalf("expected completed installer to be removed, stat error: %v", err)
	}
}
