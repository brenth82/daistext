// release-key provisions an environment secret using stdin, never a command argument.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/brenth82/daistext/internal/keyvault"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	repo := flag.String("repo", "brenth82/daistext", "GitHub repository")
	environment := flag.String("environment", "release", "protected environment")
	backup := flag.String("backup", "", "CurrentUser DPAPI backup path")
	output := flag.String("public-out", "release/update-public-key.txt", "public trust key file (safe to commit)")
	flag.Parse()
	if *backup == "" {
		p, e := keyvault.DefaultPath()
		must(e)
		*backup = p
	}
	key, e := keyvault.Create(*backup)
	must(e)
	defer clear(key)
	public := base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	if b, e := os.ReadFile(*output); e == nil {
		if strings.TrimSpace(string(b)) != public {
			must(fmt.Errorf("existing public trust file differs; refusing key rotation"))
		}
	} else if !os.IsNotExist(e) {
		must(e)
	}
	// Refuse to overwrite an established environment with a different trust root.
	remote, e := run(nil, "variable", "list", "--repo", *repo, "--env", *environment, "--json", "name,value")
	must(e)
	var variables []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	must(json.Unmarshal(remote, &variables))
	matched := false
	for _, v := range variables {
		if v.Name == "DAISTEXT_UPDATE_PUBLIC_KEY" {
			if v.Value != public {
				must(fmt.Errorf("environment has a different trust root; refusing key rotation"))
			}
			matched = true
		}
	}
	if !matched {
		secrets, e := run(nil, "secret", "list", "--repo", *repo, "--env", *environment, "--json", "name")
		must(e)
		var names []struct {
			Name string `json:"name"`
		}
		must(json.Unmarshal(secrets, &names))
		for _, n := range names {
			if n.Name == "DAISTEXT_UPDATE_PRIVATE_KEY" {
				must(fmt.Errorf("an existing signing secret has no matching public key; resolve before provisioning"))
			}
		}
	}
	must(os.MkdirAll(filepath.Dir(*output), 0755))
	must(os.WriteFile(*output, []byte(public+"\n"), 0644))
	// Install the public value first, allowing an interrupted run to safely resume.
	_, e = run(nil, "variable", "set", "DAISTEXT_UPDATE_PUBLIC_KEY", "--repo", *repo, "--env", *environment, "--body", public)
	must(e)
	_, e = run(nil, "variable", "set", "DAISTEXT_UPDATE_PUBLIC_KEY", "--repo", *repo, "--body", public)
	must(e)
	encoded := []byte(base64.StdEncoding.EncodeToString(key))
	defer clear(encoded)
	_, e = run(encoded, "secret", "set", "DAISTEXT_UPDATE_PRIVATE_KEY", "--repo", *repo, "--env", *environment)
	must(e)
	fmt.Println("Protected signing secret configured; public trust key written to", *output)
	fmt.Println("Encrypted recovery backup:", *backup)
}
func run(input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("GitHub provisioning failed: %w: %s", e, stderr.String())
	}
	return out, nil
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
