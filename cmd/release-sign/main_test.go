package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brenth82/daistext/internal/update"
)

func TestVerifyPublishedManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(pub)
	now := time.Date(2026, time.November, 23, 15, 0, 0, 0, time.UTC)
	previous := update.Manifest{
		Version: "0.1.5",
		URL:     "https://github.com/brenth82/daistext/releases/download/v0.1.5/DaisText-Setup.exe",
		SHA256:  strings.Repeat("ab", 32),
		Size:    100,
		Expires: now.Add(-time.Hour),
	}
	seal := func(m update.Manifest, tamper bool) []byte {
		t.Helper()
		payload, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		sig := ed25519.Sign(priv, payload)
		if tamper {
			sig[0] ^= 1
		}
		data, err := json.Marshal(update.Envelope{
			Payload:   base64.StdEncoding.EncodeToString(payload),
			Signature: base64.StdEncoding.EncodeToString(sig),
		})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	expired := seal(previous, false)
	if _, err := update.Verify(expired, key, now); err == nil {
		t.Fatal("updater accepted expired metadata")
	}
	if got, err := verifyPublished(expired, key, now); err != nil || !got.Expires.Equal(previous.Expires) {
		t.Fatalf("publisher read = %+v, %v", got, err)
	}
	if _, err := verifyPublished(seal(previous, true), key, now); err == nil {
		t.Fatal("publisher accepted a tampered signature")
	}
	unsafe := previous
	unsafe.URL = "https://example.org/installer.exe"
	if _, err := verifyPublished(seal(unsafe, false), key, now); err == nil {
		t.Fatal("publisher accepted an unsafe URL")
	}
	future := previous
	future.Expires = now.Add(32 * 24 * time.Hour)
	if _, err := verifyPublished(seal(future, false), key, now); err == nil {
		t.Fatal("publisher accepted metadata too far in the future")
	}
}
