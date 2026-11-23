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
	"io"
	"os"
	"strings"
	"time"

	"github.com/brenth82/daistext/internal/keyvault"
	"github.com/brenth82/daistext/internal/update"
)

func main() {
	keyOut := flag.String("generate-key", "", "write a NEW private key file; keep it secret")
	installer := flag.String("installer", "dist/DaisText-Setup.exe", "installer")
	version := flag.String("version", "", "release version")
	output := flag.String("output", "dist/update.json", "signed manifest")
	backup := flag.String("key-backup", "", "read a CurrentUser DPAPI publisher backup instead of an environment key")
	verify := flag.Bool("verify", false, "verify an existing manifest and installer without a private key")
	inspectPublished := flag.Bool("inspect-published", false, "verify a published manifest and print its expiry, even if expired; pass -installer '' to check metadata only")
	refreshFrom := flag.String("refresh-from", "", "require a previously signed manifest for the same installer and version")
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
	expected, e := expectedPublicKey()
	must(e)
	now := time.Now().UTC()
	if *inspectPublished {
		m, err := readPublished(*output, expected, *version, now)
		must(err)
		if *installer != "" {
			checkInstaller(m, *installer)
		}
		fmt.Println(m.Expires.Format(time.RFC3339Nano))
		return
	}
	var previous update.Manifest
	if *refreshFrom != "" {
		previous, e = readPublished(*refreshFrom, expected, *version, now)
		must(e)
		checkInstaller(previous, *installer)
	}
	if *verify {
		data, e := os.ReadFile(*output)
		must(e)
		m, e := update.Verify(data, expected, now)
		must(e)
		if m.Version != *version {
			must(fmt.Errorf("manifest version differs from release tag"))
		}
		checkInstaller(m, *installer)
		checkRefreshExpiry(previous, m)
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
	if expected != "" && public != expected {
		must(fmt.Errorf("private signing key does not match the build's public trust key"))
	}
	data, e := os.ReadFile(*installer)
	must(e)
	h := sha256.Sum256(data)
	m := update.Manifest{Version: *version, URL: "https://github.com/brenth82/daistext/releases/download/v" + *version + "/DaisText-Setup.exe", SHA256: hex.EncodeToString(h[:]), Size: int64(len(data)), Expires: now.Add(30 * 24 * time.Hour)}
	if *refreshFrom != "" && (m.SHA256 != previous.SHA256 || m.Size != previous.Size) {
		must(fmt.Errorf("installer changed after verification"))
	}
	checkRefreshExpiry(previous, m)
	payload, e := json.Marshal(m)
	must(e)
	env := update.Envelope{Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))}
	out, e := json.MarshalIndent(env, "", "  ")
	must(e)
	_, e = update.Verify(out, public, time.Now())
	must(e)
	must(os.WriteFile(*output, out, 0600))
}

func expectedPublicKey() (string, error) {
	expected := strings.TrimSpace(os.Getenv("DAISTEXT_UPDATE_PUBLIC_KEY"))
	b, err := os.ReadFile("release/update-public-key.txt")
	if os.IsNotExist(err) {
		return expected, nil
	}
	if err != nil {
		return "", err
	}
	pinned := strings.TrimSpace(string(b))
	if expected != "" && expected != pinned {
		return "", fmt.Errorf("configured public key differs from the pinned trust root")
	}
	return pinned, nil
}

func readPublished(path, key, version string, now time.Time) (update.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return update.Manifest{}, err
	}
	m, err := verifyPublished(data, key, now)
	if err == nil && m.Version != version {
		err = fmt.Errorf("published manifest version differs from the release tag")
	}
	return m, err
}

// Only publisher tooling may verify an expired manifest as historical proof of
// an installer's identity. Verify still checks the signature and all fields.
func verifyPublished(data []byte, key string, now time.Time) (update.Manifest, error) {
	m, err := update.Verify(data, key, now)
	if err != nil && !m.Expires.IsZero() && !m.Expires.After(now) {
		return update.Verify(data, key, m.Expires.Add(-time.Nanosecond))
	}
	return m, err
}

func checkInstaller(m update.Manifest, path string) {
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, m.Size+1))
	must(err)
	if n != m.Size || hex.EncodeToString(h.Sum(nil)) != m.SHA256 {
		must(fmt.Errorf("installer does not match authenticated manifest"))
	}
}

func checkRefreshExpiry(previous, refreshed update.Manifest) {
	if !previous.Expires.IsZero() && !refreshed.Expires.After(previous.Expires) {
		must(fmt.Errorf("refresh must extend the validity of the published manifest"))
	}
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
