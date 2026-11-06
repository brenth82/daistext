// Package update authenticates a short-lived release manifest before downloading an installer.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var PublicKey string // Base64 Ed25519 trust root, embedded at build time.
var ManifestURL = "https://github.com/brenth82/daistext/releases/latest/download/update.json"

const maxInstaller = 256 << 20

type Manifest struct {
	Version string    `json:"version"`
	URL     string    `json:"url"`
	SHA256  string    `json:"sha256"`
	Size    int64     `json:"size"`
	Expires time.Time `json:"expires"`
}
type Envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func version(s string) ([3]int, error) {
	var v [3]int
	p := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(p) != 3 {
		return v, fmt.Errorf("invalid release version")
	}
	for i, x := range p {
		n, e := strconv.Atoi(x)
		if e != nil || n < 0 || n > 65535 {
			return v, fmt.Errorf("invalid release version")
		}
		v[i] = n
	}
	return v, nil
}
func Newer(candidate, current string) bool {
	a, e := version(candidate)
	if e != nil {
		return false
	}
	b, e := version(current)
	if e != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func Verify(data []byte, key string, now time.Time) (Manifest, error) {
	var m Manifest
	pub, e := base64.StdEncoding.DecodeString(key)
	if e != nil || len(pub) != ed25519.PublicKeySize {
		return m, fmt.Errorf("this build has no valid update trust key; install an official configured release")
	}
	var env Envelope
	if e = json.Unmarshal(data, &env); e != nil {
		return m, e
	}
	payload, e := base64.StdEncoding.DecodeString(env.Payload)
	if e != nil {
		return m, e
	}
	sig, e := base64.StdEncoding.DecodeString(env.Signature)
	if e != nil || !ed25519.Verify(pub, payload, sig) {
		return m, fmt.Errorf("update signature is invalid")
	}
	if e = json.Unmarshal(payload, &m); e != nil {
		return m, e
	}
	if _, e = version(m.Version); e != nil {
		return m, e
	}
	if !m.Expires.After(now) || m.Expires.After(now.Add(31*24*time.Hour)) {
		return m, fmt.Errorf("update manifest expired or exceeds its validity window")
	}
	if m.Size <= 0 || m.Size > maxInstaller {
		return m, fmt.Errorf("invalid installer size")
	}
	hash, e := hex.DecodeString(m.SHA256)
	if e != nil || len(hash) != 32 {
		return m, fmt.Errorf("invalid installer hash")
	}
	u, e := url.Parse(m.URL)
	expected := "/brenth82/daistext/releases/download/v" + strings.TrimPrefix(m.Version, "v") + "/DaisText-Setup.exe"
	if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.Path != expected || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return m, fmt.Errorf("installer URL is outside the trusted release location")
	}
	return m, nil
}
func client() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return fmt.Errorf("too many redirects")
		}
		host := req.URL.Hostname()
		if req.URL.Scheme != "https" || (host != "github.com" && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com") {
			return fmt.Errorf("untrusted update redirect")
		}
		return nil
	}}
}
func get(ctx context.Context, u string) (*http.Response, error) {
	parsed, e := url.Parse(u)
	if e != nil || parsed.Scheme != "https" || parsed.Host != "github.com" {
		return nil, fmt.Errorf("untrusted update endpoint")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, e
	}
	r, e := client().Do(req)
	if e != nil {
		return nil, e
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		return nil, fmt.Errorf("release service returned %s", r.Status)
	}
	return r, nil
}
func Check(ctx context.Context, current string) (*Manifest, error) {
	if PublicKey == "" {
		return nil, fmt.Errorf("updates are not configured in this development build")
	}
	r, e := get(ctx, ManifestURL)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	data, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("manifest exceeds size limit")
	}
	m, e := Verify(data, PublicKey, time.Now())
	if e != nil {
		return nil, e
	}
	if !Newer(m.Version, current) {
		return nil, nil
	}
	return &m, nil
}
func Download(ctx context.Context, m Manifest) (string, error) {
	dir, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	dir = filepath.Join(dir, "DaisText", "updates")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(dir, "DaisText-Setup-*.exe")
	if e != nil {
		return "", e
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(f.Name())
		}
	}()
	r, e := get(ctx, m.URL)
	if e != nil {
		return "", e
	}
	defer r.Body.Close()
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(r.Body, m.Size+1))
	if e != nil {
		return "", e
	}
	if n != m.Size || hex.EncodeToString(h.Sum(nil)) != strings.ToLower(m.SHA256) {
		return "", fmt.Errorf("installer integrity verification failed")
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	ok = true
	return f.Name(), nil
}
