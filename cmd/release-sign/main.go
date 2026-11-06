// release-sign runs offline or in a protected release environment; it never logs a key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/brenth82/daistext/internal/keyvault"
	"github.com/brenth82/daistext/internal/update"
	"os"
	"strings"
	"time"
)

func main() {
	keyOut := flag.String("generate-key", "", "write a NEW private key file; keep it secret")
	installer := flag.String("installer", "dist/DaisText-Setup.exe", "installer")
	version := flag.String("version", "", "release version")
	output := flag.String("output", "dist/update.json", "signed manifest")
	backup := flag.String("key-backup", "", "read a CurrentUser DPAPI publisher backup instead of an environment key")
	verify := flag.Bool("verify", false, "verify an existing manifest and installer without a private key")
	flag.Parse()
	if *keyOut != "" {
		pub, priv, e := ed25519.GenerateKey(rand.Reader)
		must(e)
		f, e := os.OpenFile(*keyOut, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		must(e)
		_, e = f.WriteString(base64.StdEncoding.EncodeToString(priv))
		must(e)
		must(f.Close())
		fmt.Println("Public key:", base64.StdEncoding.EncodeToString(pub))
		return
	}
	if !update.Newer(*version, "0.0.0") {
		must(fmt.Errorf("version must be a numeric x.y.z release"))
	}
	if *verify {
		data, e := os.ReadFile(*output)
		must(e)
		pub := strings.TrimSpace(os.Getenv("DAISTEXT_UPDATE_PUBLIC_KEY"))
		if pub == "" {
			b, e := os.ReadFile("release/update-public-key.txt")
			must(e)
			pub = strings.TrimSpace(string(b))
		}
		m, e := update.Verify(data, pub, time.Now())
		must(e)
		if m.Version != *version {
			must(fmt.Errorf("manifest version differs from release tag"))
		}
		b, e := os.ReadFile(*installer)
		must(e)
		h := sha256.Sum256(b)
		if int64(len(b)) != m.Size || hex.EncodeToString(h[:]) != m.SHA256 {
			must(fmt.Errorf("installer does not match authenticated manifest"))
		}
		fmt.Println("Verified installer and signed update manifest for", m.Version)
		return
	}
	key, e := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("DAISTEXT_UPDATE_PRIVATE_KEY")))
	must(e)
	if *backup != "" {
		key, e = keyvault.Load(*backup)
		must(e)
	}
	defer clear(key)
	if len(key) != ed25519.PrivateKeySize {
		must(fmt.Errorf("DAISTEXT_UPDATE_PRIVATE_KEY must contain a base64 Ed25519 private key"))
	}
	public := base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Public().(ed25519.PublicKey))
	expected := strings.TrimSpace(os.Getenv("DAISTEXT_UPDATE_PUBLIC_KEY"))
	if b, e := os.ReadFile("release/update-public-key.txt"); e == nil {
		pinned := strings.TrimSpace(string(b))
		if expected != "" && expected != pinned {
			must(fmt.Errorf("configured public key differs from the pinned trust root"))
		}
		expected = pinned
	} else if !os.IsNotExist(e) {
		must(e)
	}
	if expected != "" && public != expected {
		must(fmt.Errorf("private signing key does not match the build's public trust key"))
	}
	data, e := os.ReadFile(*installer)
	must(e)
	h := sha256.Sum256(data)
	m := update.Manifest{Version: *version, URL: "https://github.com/brenth82/daistext/releases/download/v" + *version + "/DaisText-Setup.exe", SHA256: hex.EncodeToString(h[:]), Size: int64(len(data)), Expires: time.Now().UTC().Add(30 * 24 * time.Hour)}
	payload, e := json.Marshal(m)
	must(e)
	env := update.Envelope{Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))}
	out, e := json.MarshalIndent(env, "", "  ")
	must(e)
	_, e = update.Verify(out, public, time.Now())
	must(e)
	must(os.WriteFile(*output, out, 0600))
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
