//go:build !windows

package keyvault

import (
	"crypto/ed25519"
	"fmt"
)

func DefaultPath() (string, error) { return "", fmt.Errorf("DPAPI publisher backups require Windows") }
func Load(string) (ed25519.PrivateKey, error) {
	return nil, fmt.Errorf("DPAPI publisher backups require Windows")
}
func Create(string) (ed25519.PrivateKey, error) {
	return nil, fmt.Errorf("DPAPI publisher backups require Windows")
}
