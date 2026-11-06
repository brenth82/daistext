//go:build windows

// Package keyvault stores publisher keys encrypted with Windows CurrentUser DPAPI.
// It is release tooling only and is never included in the installed application.
package keyvault

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func DefaultPath() (string, error) {
	dir, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "DaisTextPublisher", "update-signing-key.dpapi"), nil
}

func protect(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty key data")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var e error
	if decrypt {
		e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		e = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if e != nil {
		return nil, e
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	raw := unsafe.Slice(out.Data, int(out.Size))
	result := append([]byte(nil), raw...)
	clear(raw)
	return result, nil
}

func privateDirectory(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	sd, e := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if e != nil {
		return e
	}
	acl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

func Load(filename string) (ed25519.PrivateKey, error) {
	b, e := os.ReadFile(filename)
	if e != nil {
		return nil, e
	}
	key, e := protect(b, true)
	if e != nil {
		return nil, fmt.Errorf("decrypt publisher key using its original Windows account: %w", e)
	}
	if len(key) != ed25519.PrivateKeySize || !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])) {
		clear(key)
		return nil, fmt.Errorf("invalid recovered publisher key")
	}
	return ed25519.PrivateKey(key), nil
}

// Create never replaces an existing key. Re-running provisioning reuses the backup.
func Create(filename string) (ed25519.PrivateKey, error) {
	if _, e := os.Stat(filename); e == nil {
		return Load(filename)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if e := privateDirectory(filepath.Dir(filename)); e != nil {
		return nil, e
	}
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return nil, e
	}
	encrypted, e := protect(key, false)
	if e != nil {
		clear(key)
		return nil, e
	}
	f, e := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		clear(key)
		return nil, e
	}
	_, e = f.Write(encrypted)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		clear(key)
		return nil, e
	}
	recovered, e := Load(filename)
	if e != nil {
		clear(key)
		return nil, e
	}
	if !bytes.Equal(key, recovered) {
		clear(key)
		clear(recovered)
		return nil, fmt.Errorf("key recovery verification failed")
	}
	clear(recovered)
	return key, nil
}
