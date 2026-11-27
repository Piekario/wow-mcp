# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
<#
.SYNOPSIS
Tests install.ps1 against a throwaway project copy and a fake game folder (Windows only: the installer uses robocopy).

.DESCRIPTION
Runs the installer in a child process of the same PowerShell host, so both Windows PowerShell 5.1 and PowerShell 7
can be covered by calling this script from each. Needs Go in PATH: the fresh install case uses -Build.
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:RepoRoot = Split-Path -Parent $PSScriptRoot
$script:HostExe = (Get-Process -Id $PID).Path
$script:Failures = 0

function Assert-True([bool]$Condition, [string]$Message) {
    if ($Condition) {
        Write-Host ('  ok    ' + $Message)
    }
    else {
        Write-Host ('  FAIL  ' + $Message) -ForegroundColor Red
        $script:Failures++
    }
}

function New-Sandbox {
    $root = Join-Path ([System.IO.Path]::GetTempPath()) ('wow-mcp-install-test-' + [guid]::NewGuid().ToString('N'))
    $project = Join-Path $root 'project'
    $wow = Join-Path $root 'World of Warcraft'
    $null = New-Item -ItemType Directory -Force -Path $project
    $null = New-Item -ItemType Directory -Force -Path (Join-Path $wow '_forever_\Interface\AddOns')
    Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'install.ps1') -Destination $project
    Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'addon') -Destination $project -Recurse
    Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'server') -Destination $project -Recurse
    return [pscustomobject]@{
        Root        = $root
        Project     = $project
        WowDir      = $wow
        AddonSource = Join-Path $project 'addon\WowMcp'
        AddonTarget = Join-Path $wow '_forever_\Interface\AddOns\WowMcp'
        BackupDir   = Join-Path $project 'backup'
        Exe         = Join-Path $project 'bin\wow-mcp.exe'
    }
}

function Invoke-Installer($Sandbox, [string[]]$Arguments) {
    $all = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $Sandbox.Project 'install.ps1'),
        '-WowDir', $Sandbox.WowDir) + $Arguments
    $saved = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $script:HostExe @all 2>&1 | Out-String
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $saved
    }
    return [pscustomobject]@{ Code = $code; Output = $output }
}

function Get-RelativeFiles([string]$Dir) {
    if (-not (Test-Path -LiteralPath $Dir)) { return @() }
    $prefix = (Resolve-Path -LiteralPath $Dir).Path.TrimEnd('\') + '\'
    return @(Get-ChildItem -LiteralPath $Dir -Recurse -File | ForEach-Object { $_.FullName.Substring($prefix.Length) } | Sort-Object)
}

function Test-ClientCheck($Sandbox) {
    Write-Host 'A -Client value that leaves the game folder is refused'
    foreach ($client in @('..\..', '_forever_\..', 'C:', '.')) {
        $result = Invoke-Installer $Sandbox @('-Client', $client, '-SkipCheck')
        Assert-True ($result.Code -eq 1) ("-Client '" + $client + "' exits with code 1 (got " + $result.Code + ')')
        Assert-True ($result.Output -match '-Client must be a single folder name') ("-Client '" + $client + "' names the reason")
    }
    $result = Invoke-Installer $Sandbox @('-Client', '_retail_', '-SkipCheck')
    Assert-True ($result.Code -eq 1) 'a client folder without AddOns exits with code 1'
    Assert-True ($result.Output -match 'The AddOns folder does not exist') 'a client folder without AddOns names the reason'
    Assert-True (-not (Test-Path -LiteralPath $Sandbox.AddonTarget)) 'nothing was installed by the refused runs'
    Assert-True (-not (Test-Path -LiteralPath $Sandbox.BackupDir)) 'no backup folder was created by the refused runs'
}

function Test-FreshInstall($Sandbox) {
    Write-Host 'A fresh install copies the addon and builds the server'
    $result = Invoke-Installer $Sandbox @('-Build')
    if ($result.Code -ne 0) { Write-Host $result.Output }
    Assert-True ($result.Code -eq 0) ('exits with code 0 (got ' + $result.Code + ')')
    $source = @(Get-RelativeFiles $Sandbox.AddonSource)
    $target = @(Get-RelativeFiles $Sandbox.AddonTarget)
    Assert-True ($source.Count -gt 0 -and (($source -join '|') -eq ($target -join '|'))) 'the installed addon has exactly the files of the source'
    Assert-True (Test-Path -LiteralPath $Sandbox.Exe) 'bin\wow-mcp.exe exists'
    Assert-True ($result.Output -match '"command"') 'the summary prints the client configuration entry'
}

function Test-Reinstall($Sandbox) {
    Write-Host 'A second install backs up the addon and removes files that left the source'
    $stale = Join-Path $Sandbox.AddonTarget 'Removed.lua'
    Set-Content -LiteralPath $stale -Value '-- stale'
    $result = Invoke-Installer $Sandbox @('-SkipCheck')
    if ($result.Code -ne 0) { Write-Host $result.Output }
    Assert-True ($result.Code -eq 0) ('exits with code 0 (got ' + $result.Code + ')')
    Assert-True (-not (Test-Path -LiteralPath $stale)) 'the stale file is gone from the addon folder'
    $backups = @(Get-ChildItem -LiteralPath $Sandbox.BackupDir -Directory -Filter 'addon-*' -ErrorAction SilentlyContinue)
    Assert-True ($backups.Count -eq 1) ('one backup exists (got ' + $backups.Count + ')')
    Assert-True (($backups.Count -eq 1) -and (Test-Path -LiteralPath (Join-Path $backups[0].FullName 'Removed.lua'))) 'the backup holds the stale file'
}

function Test-Rollback($Sandbox) {
    Write-Host '-Rollback restores the addon from the last backup'
    $result = Invoke-Installer $Sandbox @('-Rollback', '-SkipCheck')
    if ($result.Code -ne 0) { Write-Host $result.Output }
    Assert-True ($result.Code -eq 0) ('exits with code 0 (got ' + $result.Code + ')')
    Assert-True (Test-Path -LiteralPath (Join-Path $Sandbox.AddonTarget 'Removed.lua')) 'the file from the backup is back'
    Assert-True (Test-Path -LiteralPath (Join-Path $Sandbox.AddonTarget 'WowMcp.toc')) 'the addon is still complete'
}

function Test-JunctionGuard($Sandbox) {
    Write-Host 'An addon folder that is a junction is refused and the linked folder is left alone'
    $outside = Join-Path $Sandbox.Root 'linked-repo'
    $null = New-Item -ItemType Directory -Force -Path $outside
    $keep = Join-Path $outside 'keep.txt'
    Set-Content -LiteralPath $keep -Value 'must survive'
    Remove-Item -LiteralPath $Sandbox.AddonTarget -Recurse -Force
    $null = & cmd /c mklink /J $Sandbox.AddonTarget $outside
    Assert-True ($LASTEXITCODE -eq 0) 'the junction was created'
    $before = @(Get-ChildItem -LiteralPath $Sandbox.BackupDir -Directory -Filter 'addon-*').Count
    $result = Invoke-Installer $Sandbox @('-SkipCheck')
    Assert-True ($result.Code -eq 1) ('install exits with code 1 (got ' + $result.Code + ')')
    Assert-True ($result.Output -match 'is a link \(junction or symlink\)') 'install names the reason'
    $result = Invoke-Installer $Sandbox @('-Rollback', '-SkipCheck')
    Assert-True ($result.Code -eq 1) ('-Rollback exits with code 1 (got ' + $result.Code + ')')
    Assert-True (Test-Path -LiteralPath $keep) 'the file in the linked folder survived'
    Assert-True (@(Get-RelativeFiles $outside).Count -eq 1) 'nothing was copied into the linked folder'
    $after = @(Get-ChildItem -LiteralPath $Sandbox.BackupDir -Directory -Filter 'addon-*').Count
    Assert-True ($after -eq $before) 'no backup was taken through the junction'
    & cmd /c rmdir $Sandbox.AddonTarget
}

$sandbox = New-Sandbox
try {
    Write-Host ('Host: ' + $script:HostExe + ' (PowerShell ' + $PSVersionTable.PSVersion + ')')
    Test-ClientCheck $sandbox
    Test-FreshInstall $sandbox
    Test-Reinstall $sandbox
    Test-Rollback $sandbox
    Test-JunctionGuard $sandbox
}
finally {
    Remove-Item -LiteralPath $sandbox.Root -Recurse -Force -ErrorAction SilentlyContinue
}

if ($script:Failures -gt 0) {
    Write-Host ($script:Failures.ToString() + ' check(s) failed.') -ForegroundColor Red
    exit 1
}
Write-Host 'All install.ps1 checks passed.' -ForegroundColor Green
