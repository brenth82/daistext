# DaisText

A native Windows DAISY-to-text converter written in Go. Windows 10/11 x64;
ARM64 Windows can use x64 emulation. Document conversion happens locally.

DaisText source and documentation are available under the [MIT License](LICENSE).
Bundled tools and dependencies retain the licenses listed in
[Third-party notices](THIRD-PARTY-NOTICES.md). **About** shows the running version.

## Use

Install `DaisText-Setup.exe`, then open DaisText from the Start menu. The installer
includes the PDF engine and needs no administrator access or Go installation.
Open a DAISY folder, `.zip` or `.daisy` archive, uncheck entries to skip, choose a format,
and choose **Convert selected**. Space toggles a section checkbox. Preview follows
the highlighted row. **Toggle selected branch** selects or clears a parent and
all its descendants. A parent's own checkbox affects its introductory text only.
Selection always exports in the source reading order, with no duplicated parent
content. Skipped ancestors are omitted; surviving heading levels are normalized.

On launch, selection, preview and export controls stay hidden until a book loads.
Settings lets you hide either the folder or ZIP import button, with at least one
kept visible. For NFB NEWSLINE emails with four separate attachments, save the
OPF, NCX, SMIL and XML files together in a folder and use **Open DAISY folder**.
No manual ZIP creation is needed. Both import modes read resources in place;
archives are never extracted. Closing DaisText cancels active conversions and
waits for temporary conversion files to be cleaned up.

Formats:

* `.txt`: Unicode UTF-8, Windows line endings.
* `.md`: escaped Markdown headings and paragraphs.
* `.html`: standalone HTML with language, headings, and escaped text.
* `.pdf`: tagged PDF compiled with Typst's enforced PDF/UA-1 mode. Typst is bundled
  in packaged installers. Individual executable builds can use Typst on PATH or
  an executable path in Settings. Human reading-order and screen-reader review
  are still needed for a public accessibility claim.
* `.brf`: optional Liblouis `lou_translate`, ASCII output, cell wrapping and page
  breaks. Select the **Liblouis braille translator and tables** component in the
  installer for offline export, or configure a separate installation. With blank
  tool and table settings, the bundled translator uses contracted English UEB and
  the US BRF display table. Configure another table chain for other languages or
  conventions (for example `en-us-brf.dis,en-ueb-g2.ctb`). Use absolute
  table paths if they are not in Liblouis's data search path. Review translation
  and embossing conventions with the intended reader; this is simple text layout,
  not a full braille publishing/typesetting engine.

Supports text-bearing DAISY 3 OPF/NCX/SMIL/DTBook and DAISY 2.02 NCC/SMIL/HTML
folders or archives. Audio-only books are reported as unsupported. Archives stay compressed;
external references, archive traversal and excessive text sizes are rejected.
This initial text-focused importer preserves headings and prose, not original
visual layout, images, linked resources or rich table semantics. Multiple OPFs in
one input are rejected to avoid silently selecting the wrong book. Folder reads
are rooted at the selected directory, and symbolic links are rejected.

Settings are stored per user at `%APPDATA%\DaisText\settings.json`. Settings
includes import button visibility, default output folder, PDF/BRF tool paths,
braille tables and page size, and automatic update checks. The output default is
the current user's Windows Downloads Known Folder, including redirected folders.
Change it using Settings, Browse or Use Downloads. Saving a particular conversion
elsewhere does not change that configured default.

Uninstall removes DaisText settings, staged update installers, conversion cache, bundled tools,
shortcuts and its installer registration. Source books, exported documents, and
separately installed translators are preserved. Windows may retain its own
installation/security history; DaisText does not erase operating-system records.
To reset settings without uninstalling,
close DaisText and remove that file. Update installers use
`%LOCALAPPDATA%\DaisText\updates`; these files can be removed after installation.
PDF conversion workspaces use `%LOCALAPPDATA%\DaisText\temp` and are deleted after
conversion or cancellation. Uninstall also removes workspaces abandoned by a crash.

## Build and package

Install Go 1.25 or newer, Git and PowerShell. No C compiler or Windows SDK is
needed: the application uses Go and native Win32 controls through Walk.

```powershell
./scripts/build.ps1
./scripts/package.ps1 -Version 0.1.0
# Optional unattended per-user installation of that build:
./scripts/install.ps1
```

Outputs: `dist/DaisText.exe`, `dist/daistext-cli.exe`, and (package command)
`dist/DaisText-Setup.exe`. `build.ps1 -WithTools` includes the PDF engine without
compiling an installer. Packaging stages checksum-pinned Typst, Liblouis and Inno Setup
under ignored `.tools`, uses installed Inno Setup when available (otherwise stages
the compiler into `.tools/inno`), and includes
Typst and runtime licenses in the offline installer. Optional Liblouis ships
with its full corresponding source archive, licensing files and build instructions.
Network access is needed
only on the build machine for Go modules and these tools. Tool versions and
SHA-256 checksums are in `scripts/tools.lock.json`; review both when updating.

```powershell
go test ./...
go vet ./...
# Optional private integration test: never add the input book to Git.
$env:DAISTEXT_TEST_BOOK = 'C:\private\book.zip'
# Optional comparison against an already extracted private folder:
$env:DAISTEXT_TEST_FOLDER = 'C:\private\book'
go test -v ./internal/daisy
./dist/daistext-cli.exe -input C:\books\book.zip -list
./dist/daistext-cli.exe -input C:\books\book.zip -sections 5,2 -format html -output C:\books\selected.html
```

Build scripts use ignored `.cache` directories for Go caches to work in restricted
workspaces. For direct Go commands in such a workspace, set `GOPATH`, `GOMODCACHE`
and `GOCACHE` to the same directories used by `scripts/build.ps1`.

## Releases and secure updates

CI builds/tests on pushes and PRs. Pushing a numeric `vX.Y.Z` tag triggers the
installer release workflow. The publisher public key is pinned in
`release/update-public-key.txt` and is safe to commit. Builds refuse a conflicting
key supplied through their environment. The private key lives only in the protected
GitHub `release` environment and a Windows-account-encrypted publisher backup.
See [release setup and approval](release/README.md) for the configured protections,
local signing, recovery limits and the steps to approve a release.

The release workflow builds and tests without signing credentials. After approval,
an isolated runner signs the manifest. A third runner independently verifies the
signature and installer hash before publishing; it has no private signing key.
Actions use immutable commit references, and signing does not reuse build caches.

Optionally add your organization's Windows Authenticode signing step before
hashing the installer. Without a publisher certificate Windows can display an
unknown-publisher/SmartScreen warning. Ed25519 update signing authenticates updates
to DaisText; it does not replace Windows publisher trust for the initial download.

Updates use a build-embedded public key, Ed25519 signatures over exact manifest
bytes, HTTPS-only trusted release URLs, strict size limits, version comparison,
a 30-day signed expiry and SHA-256 installer verification before execution.
An automatic startup check asks before downloading/installing. Turning it off
disables the startup network request; **Check for updates** remains available.
After confirmation, installation shows progress and reopens DaisText when done.
Development builds with no public key never
silently trust a downloaded manifest and explain that updates are unconfigured.

Publish a new signed manifest at least every 30 days, even if re-signing the
current release, or clients will correctly reject stale metadata. Keep the public
key stable across releases. Key rotation requires a separately trusted installer;
there is no remote trust-key replacement endpoint. A compromised release host can
block updates but cannot produce an authenticated installer without the private
key. Compromise of the signing environment/key remains a critical risk.

## Validation and remaining release work

Automated tests cover DAISY hierarchy boundaries, source-order selection,
DAISY 2.02, omitted articles, unsafe resource references, output escaping,
non-destructive export failure, braille pagination and signed-manifest rejection.
Private local integration tests compare folder and ZIP imports without storing
publication text, expected headlines or converted outputs in the test suite.
No supplied publication text or archive is included in the repository or CI.

Before broad distribution, test the installer/update/uninstall cycle in a clean
Windows VM, test keyboard navigation with NVDA and JAWS, validate sample PDFs
with PAC/veraPDF and a screen reader, and test BRF with the intended embosser.
Tests exercise the real bundled Liblouis and Typst engines during packaged builds.
GitHub publication, a production signing key and a publisher
certificate are deployment setup, not checked-in credentials.
