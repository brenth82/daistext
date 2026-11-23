$ErrorActionPreference = 'Stop'

function Get-CurrentRelease {
    $response = gh api "repos/$env:GITHUB_REPOSITORY/releases/latest"
    if ($LASTEXITCODE) { throw 'Could not read the latest published release' }
    $release = $response | ConvertFrom-Json
    if ($release.tag_name -cne $env:RELEASE_TAG -or $release.immutable -ne $false) {
        throw 'The target is no longer the latest published release with replaceable assets'
    }
    return $release
}

function Rename-ReleaseAsset([long]$Id, [string]$Name) {
    gh api --method PATCH "repos/$env:GITHUB_REPOSITORY/releases/assets/$Id" -f "name=$Name" --silent
    if ($LASTEXITCODE) { throw "Could not rename release asset $Id to $Name" }
}

if ($env:RELEASE_TAG -cnotmatch '^v[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}$' -or
    $env:PREVIOUS_HASH -cnotmatch '^[0-9A-F]{64}$') {
    throw 'Expected a numeric tag and a SHA-256 from the verification job'
}
$release = Get-CurrentRelease
$previous = $release.assets | Where-Object name -CEQ 'update.json'
if (!$previous) { throw 'The published update.json is missing' }

$base = "https://github.com/$env:GITHUB_REPOSITORY/releases/download/$env:RELEASE_TAG"
Invoke-WebRequest -Uri "$base/update.json" -OutFile dist/previous.json -TimeoutSec 60
if ((Get-FileHash -LiteralPath dist/previous.json).Hash -cne $env:PREVIOUS_HASH) {
    throw 'The published manifest changed during this run'
}
Invoke-WebRequest -Uri "$base/DaisText-Setup.exe" -OutFile dist/DaisText-Setup.exe -TimeoutSec 120
go run ./cmd/release-sign -verify "-version=$($env:RELEASE_TAG.Substring(1))" -refresh-from dist/previous.json
if ($LASTEXITCODE) { throw 'The renewed manifest failed independent verification' }

# Stage the verified file first so an upload failure leaves update.json in place.
$suffix = "$env:GITHUB_RUN_ID-$env:GITHUB_RUN_ATTEMPT"
$stagedName = "update-next-$suffix.json"
$backupName = "update-previous-$suffix.json"
Copy-Item -LiteralPath dist/update.json -Destination "dist/$stagedName"
gh release upload $env:RELEASE_TAG "dist/$stagedName" --repo $env:GITHUB_REPOSITORY
if ($LASTEXITCODE) { throw 'Could not stage the verified replacement manifest' }

$release = Get-CurrentRelease
$staged = $release.assets | Where-Object name -CEQ $stagedName
$current = $release.assets | Where-Object name -CEQ 'update.json'
$verifiedHash = (Get-FileHash -LiteralPath dist/update.json).Hash.ToLowerInvariant()
if (!$staged -or $staged.digest -cne "sha256:$verifiedHash" -or
    $current.id -ne $previous.id -or $current.updated_at -ne $previous.updated_at) {
    throw 'A staged or published release asset changed before replacement'
}

try {
    Rename-ReleaseAsset $previous.id $backupName
    Rename-ReleaseAsset $staged.id 'update.json'
} catch {
    $swapFailure = $_
    $actualName = gh api "repos/$env:GITHUB_REPOSITORY/releases/assets/$($staged.id)" --jq .name
    if ($LASTEXITCODE -or $actualName -cne 'update.json') {
        Rename-ReleaseAsset $previous.id 'update.json'
        throw $swapFailure
    }
}

gh api --method DELETE "repos/$env:GITHUB_REPOSITORY/releases/assets/$($previous.id)" --silent
if ($LASTEXITCODE) { Write-Warning "The refresh succeeded. The backup $backupName can be removed." }
