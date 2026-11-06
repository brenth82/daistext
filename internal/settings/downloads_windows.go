//go:build windows

package settings

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

// Use the user's Known Folder, including OneDrive/network redirection.
func DownloadsDirectory() string {
	if p, e := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); e == nil && p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}
