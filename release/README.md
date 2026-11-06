# Publisher setup and approval

The `brenth82/daistext` release environment is configured with:

* `brenth82` as required reviewer. Self-approval is allowed for this personal
  repository; approval is still a deliberate step after the build finishes.
* Administrator bypass disabled for environment approval.
* Only version tags matching `v*.*.*` allowed to use the environment.
* `DAISTEXT_UPDATE_PRIVATE_KEY` stored as an environment secret, never a
  repository-wide secret or checked-in file.
* `DAISTEXT_UPDATE_PUBLIC_KEY` stored as a public environment/repository variable
  and pinned in `update-public-key.txt`.

The repository requires full commit SHA references for actions and allows only
the checkout, setup-go, upload-artifact and download-artifact actions. A tag
ruleset prevents version tags from being changed or deleted, without bypass actors.
Do not change the code behind a released tag; create a new version instead.

## Trigger and approve

First review, commit and push the application code and workflows. Then tag the
reviewed commit:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

Open [GitHub Actions](https://github.com/brenth82/daistext/actions), open the release
run and wait for the build to finish. Choose **Review deployments**, select
**release**, then **Approve and deploy**. Inspect the tag, commit and build result
before approving: this approval authorizes the reviewed source to use the signing
key. Signing and independent verification run before automatic publication.

Build, signing and publishing use separate hosted runners. The private key is
injected only into the signing step, not into the build tools or publishing job.
The signing job checks its protected public key against the reviewed trust file.
The publish job verifies the signature and installer hash again without a private
key. No publication happens merely from configuring the environment.

## Local signing and recovery

The publisher key was generated with Go's cryptographic random source. Provisioning
uploads it to GitHub through standard input, never a command-line argument or log.
An encrypted recovery copy is held at:

`%LOCALAPPDATA%\DaisTextPublisher\update-signing-key.dpapi`

This directory grants access only to the current Windows user and SYSTEM. The
backup uses **CurrentUser** DPAPI, not machine-wide encryption. Recovery normally
requires the original Windows account/profile and computer. Copying this file
alone to another machine is not a portable backup. Preserve the original profile,
or separately arrange secure offline key escrow before relying on cross-machine
recovery. GitHub does not let you read back the stored secret.

The backup belongs to publisher tooling and is deliberately outside the application
and repository. Uninstalling the consumer app does not delete this signing key.

```powershell
./scripts/package.ps1 -Version 0.1.0
go run ./cmd/release-sign -version 0.1.0 -key-backup "$env:LOCALAPPDATA\DaisTextPublisher\update-signing-key.dpapi"
go run ./cmd/release-sign -verify -version 0.1.0
```

To re-provision the same key, use `go run ./cmd/release-key`. This loads the encrypted
backup, checks that existing public keys match and refuses unplanned key rotation.
It uploads the secret without printing its contents. It does not create or weaken
the environment's protection rules.

Never put a private key, decrypted backup, supplied newspaper, or converted private
document into Git or release assets. Only the public key is intended for Git.

## Trust boundaries

GitHub administrators who can change environment protections, or a compromised
publisher Windows account, remain able to compromise the signing process. Approval
must include reviewing changes to signing tools and workflows. A second independent
maintainer or an external signing service would provide stronger separation if the
project grows. Account-bound encrypted storage protects the backup at rest; it
cannot protect a key while malicious code runs as its owner.

Signed metadata expires in 30 days. Refresh it for long-lived releases before
expiry. Key loss or emergency rotation requires a separately trusted installer;
clients do not download replacement trust roots from the release server.
