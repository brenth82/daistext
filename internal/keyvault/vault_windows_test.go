//go:build windows

package keyvault

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedKeyRecovery(t *testing.T) {
	if os.Getenv("DAISTEXT_TEST_DPAPI") == "" {
		t.Skip("set DAISTEXT_TEST_DPAPI for a Windows account-backed key recovery test")
	}
	filename := filepath.Join(t.TempDir(), "publisher", "key.dpapi")
	key, e := Create(filename)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(key)
	disk, e := os.ReadFile(filename)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(disk, key) {
		t.Fatal("private key stored in plaintext")
	}
	again, e := Create(filename)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(again)
	if !bytes.Equal(key, again) {
		t.Fatal("reprovisioning rotated the key")
	}
	disk[len(disk)/2] ^= 1
	if _, e := protect(disk, true); e == nil {
		t.Fatal("accepted a damaged encrypted backup")
	}
}
