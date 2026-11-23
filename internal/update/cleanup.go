package update

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	installerCache  = "DaisText/updates"
	installerPrefix = "DaisText-Setup-"
	installerSuffix = ".exe"
	// Allow time for another DaisText window to download and start an update.
	installerStaleAge = 10 * time.Minute
)

var stagedInstaller = regexp.MustCompile(`^` + regexp.QuoteMeta(installerPrefix) + `[0-9]+` + regexp.QuoteMeta(installerSuffix) + `$`)

// CleanupInstallers removes old staged installers and the one that just
// relaunched DaisText. Windows may keep that executable locked until Setup exits.
func CleanupInstallers(ctx context.Context, completedInstaller string) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		return
	}
	defer root.Close()
	dir, err := root.OpenRoot(installerCache)
	if err != nil {
		return
	}
	defer dir.Close()
	f, err := dir.Open(".")
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}

	completed, _ := filepath.Rel(filepath.Join(cache, installerCache), completedInstaller)
	staleBefore := time.Now().Add(-installerStaleAge)
	var pending []string
	for _, entry := range entries {
		name := entry.Name()
		if !stagedInstaller.MatchString(name) {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.Mode().IsRegular() && (name == completed || info.ModTime().Before(staleBefore)) {
			pending = append(pending, name)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for len(pending) > 0 && ctx.Err() == nil {
		retry := pending[:0]
		for _, name := range pending {
			if err := dir.Remove(name); err != nil && !os.IsNotExist(err) {
				retry = append(retry, name)
			}
		}
		pending = retry
		if len(pending) > 0 {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
}
