[CmdletBinding()]
param([string]$Installer = (Join-Path $PSScriptRoot '../dist/DaisText-Setup.exe'))
$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Installer)) { throw 'Build the installer with scripts/package.ps1 first.' }
# Unattended per-user installation; run the EXE directly for the standard wizard.
$process = Start-Process -FilePath (Resolve-Path -LiteralPath $Installer).Path -ArgumentList @('/SP-','/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/CURRENTUSER') -WindowStyle Hidden -Wait -PassThru
if ($process.ExitCode -ne 0) { throw "Installation failed with exit code $($process.ExitCode)" }
Write-Host 'DaisText installed. Open it from the Start menu.'
