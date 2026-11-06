[CmdletBinding()]
param([string]$Version = '0.1.0', [string]$UpdatePublicKey = $env:DAISTEXT_UPDATE_PUBLIC_KEY, [switch]$WithTools)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw 'Version must be x.y.z' }
$trustFile = Join-Path $PWD 'release/update-public-key.txt'
if (Test-Path -LiteralPath $trustFile) {
    $pinnedKey = (Get-Content -LiteralPath $trustFile -Raw).Trim()
    if ($UpdatePublicKey -and $UpdatePublicKey.Trim() -ne $pinnedKey) { throw 'Configured update key differs from the pinned publisher key' }
    $UpdatePublicKey = $pinnedKey
}
if ($UpdatePublicKey -and $UpdatePublicKey -notmatch '^[A-Za-z0-9+/]{43}=$') { throw 'Expected base64 Ed25519 public key' }
$env:GOPATH = Join-Path $PWD '.cache/gopath'
$env:GOCACHE = Join-Path $PWD '.cache/go-build'
$env:GOMODCACHE = Join-Path $PWD '.cache/go-mod'
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force dist,dist/tools | Out-Null
go mod download
if ($LASTEXITCODE) { throw 'Dependency download failed' }
if ($WithTools) {
    go run ./cmd/tool-fetch
    if ($LASTEXITCODE) { throw 'Pinned tool staging failed' }
    $env:DAISTEXT_TEST_TYPST = (Resolve-Path '.tools/typst/typst.exe').Path
    $env:DAISTEXT_TEST_LOUIS = (Resolve-Path '.tools/liblouis/bin/lou_translate.exe').Path
}
go test ./...
if ($LASTEXITCODE) { throw 'Tests failed' }
go run github.com/akavel/rsrc@v0.10.2 -manifest cmd/daistext/app.manifest -arch amd64 -o cmd/daistext/rsrc_windows_amd64.syso
if ($LASTEXITCODE) { throw 'Manifest embedding failed' }
$linkFlags = "-s -w -H=windowsgui -X main.version=$Version"
if ($UpdatePublicKey) { $linkFlags += " -X github.com/brenth82/daistext/internal/update.PublicKey=$UpdatePublicKey" }
go build -trimpath -ldflags $linkFlags -o dist/DaisText.exe ./cmd/daistext
if ($LASTEXITCODE) { throw 'GUI build failed' }
go build -trimpath -ldflags '-s -w' -o dist/daistext-cli.exe ./cmd/daistext-cli
if ($LASTEXITCODE) { throw 'CLI build failed' }
if ($WithTools) {
    Copy-Item .tools/typst/* dist/tools -Force
    New-Item -ItemType Directory -Force dist/liblouis,dist/liblouis/source | Out-Null
    Copy-Item .tools/liblouis/* dist/liblouis -Recurse -Force
    Copy-Item .tools/liblouis-source.zip dist/liblouis/source/liblouis-3.39.0.zip -Force
}
Copy-Item README.md,THIRD-PARTY-NOTICES.md dist -Force
$licenseDir = Join-Path $PWD 'dist/tools/licenses'
New-Item -ItemType Directory -Force $licenseDir | Out-Null
$moduleDirs = go list -m -f '{{.Dir}}' all
if ($LASTEXITCODE) { throw 'Module license discovery failed' }
foreach ($moduleDir in $moduleDirs) {
    if (-not $moduleDir -or $moduleDir -eq $PWD.Path) { continue }
    $name = Split-Path $moduleDir -Leaf
    $targetDir = Join-Path $licenseDir $name
    New-Item -ItemType Directory -Force $targetDir | Out-Null
    Get-ChildItem -LiteralPath $moduleDir -File | Where-Object Name -Match '^(LICENSE|COPYING|NOTICE|PATENTS)' | Copy-Item -Destination $targetDir -Force
}
Copy-Item (Join-Path (go env GOROOT) 'LICENSE') (Join-Path $licenseDir 'Go-LICENSE') -Force
Write-Host "Built DaisText $Version in $PWD/dist"
