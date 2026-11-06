[CmdletBinding()]
param([string]$Version = '0.1.0', [string]$UpdatePublicKey = $env:DAISTEXT_UPDATE_PUBLIC_KEY, [string]$InnoCompiler)
$ErrorActionPreference = 'Stop'
& "$PSScriptRoot/build.ps1" -Version $Version -UpdatePublicKey $UpdatePublicKey -WithTools
$compiler = $InnoCompiler
if (-not $compiler) {
    $candidates = @(
        "$env:ProgramFiles\Inno Setup 7\ISCC.exe",
        "${env:ProgramFiles(x86)}\Inno Setup 7\ISCC.exe",
        "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        (Join-Path $PWD '.tools/inno/ISCC.exe')
    )
    $compiler = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $compiler) { $compiler = Join-Path $PWD '.tools/inno/ISCC.exe' }
if (-not (Test-Path $compiler)) {
    $setup = (Resolve-Path '.tools/inno.exe').Path
    $target = Join-Path $PWD '.tools/inno'
    $log = Join-Path $PWD '.tools/inno-install.log'
    $process = Start-Process -FilePath $setup -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/CURRENTUSER',"/DIR=`"$target`"",'/NOICONS',"/LOG=`"$log`"") -WindowStyle Hidden -PassThru -Wait
    if ($process.ExitCode -ne 0) { throw "Inno Setup staging failed: $($process.ExitCode)" }
}
& $compiler "/DAppVersion=$Version" installer/DaisText.iss
if ($LASTEXITCODE) { throw 'Installer compilation failed' }
Get-FileHash dist/DaisText-Setup.exe -Algorithm SHA256
