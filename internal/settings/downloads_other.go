//go:build !windows

package settings

import (
	"os"
	"path/filepath"
)

func DownloadsDirectory() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}
