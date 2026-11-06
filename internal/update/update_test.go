package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignedManifest(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := base64.StdEncoding.EncodeToString(pub)
	now := time.Now()
	m := Manifest{Version: "1.2.3", URL: "https://github.com/brenth82/daistext/releases/download/v1.2.3/DaisText-Setup.exe", SHA256: strings.Repeat("ab", 32), Size: 200, Expires: now.Add(time.Hour)}
	seal := func(m Manifest) []byte {
		b, _ := json.Marshal(m)
		out, _ := json.Marshal(Envelope{base64.StdEncoding.EncodeToString(b), base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b))})
		return out
	}
	if _, e := Verify(seal(m), key, now); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Manifest){func(m *Manifest) { m.URL = "https://evil.example/setup.exe" }, func(m *Manifest) { m.Expires = now.Add(-time.Second) }, func(m *Manifest) { m.Expires = now.Add(32 * 24 * time.Hour) }, func(m *Manifest) { m.Size = maxInstaller + 1 }, func(m *Manifest) { m.SHA256 = "abc" }} {
		bad := m
		change(&bad)
		if _, e := Verify(seal(bad), key, now); e == nil {
			t.Fatal("accepted unsafe manifest")
		}
	}
	b := seal(m)
	var env Envelope
	json.Unmarshal(b, &env)
	env.Payload = base64.StdEncoding.EncodeToString([]byte(`{"version":"9.9.9"}`))
	b, _ = json.Marshal(env)
	if _, e := Verify(b, key, now); e == nil {
		t.Fatal("accepted tampering")
	}
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, e := Verify(seal(m), base64.StdEncoding.EncodeToString(wrong), now); e == nil {
		t.Fatal("accepted wrong signing key")
	}
}
func TestVersionOrdering(t *testing.T) {
	for _, s := range []string{"1.10.0", "2.0.0", "v1.2.4"} {
		if !Newer(s, "1.2.3") {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"1.2.3", "1.2.2", "garbage", "1.2.3-beta", "-1.3.4"} {
		if Newer(s, "1.2.3") {
			t.Fatal(s)
		}
	}
}
