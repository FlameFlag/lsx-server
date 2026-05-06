<#
.SYNOPSIS
Builds an optimized lsx-server binary.

.DESCRIPTION
Generates embedded browser assets, optionally runs tests, and builds a trimmed
release binary for Windows, Linux, or macOS.

.EXAMPLE
.\build.ps1 -Target windows/amd64

.EXAMPLE
.\build.ps1 -Os macos -Arch arm64 -SkipTests
#>
#Requires -Version 5.1
[CmdletBinding()]
param(
    [ValidatePattern('^[^/]+/[^/]+$')]
    [string] $Target,

    [Alias('Os')]
    [ValidateSet('linux', 'darwin', 'macos', 'windows')]
    [string] $TargetOS = $env:GOOS,

    [Alias('Architecture')]
    [ValidateSet('386', 'amd64', 'x64', 'arm64', 'aarch64')]
    [string] $Arch = $env:GOARCH,

    [string] $Out = $env:OUT,
    [string] $Name = $env:APP_NAME,
    [string] $Version = $env:VERSION,
    [switch] $SkipTests,
    [switch] $SkipChecks,
    [switch] $ListTargets
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$InformationPreference = 'Continue'

function Invoke-Native([string] $FilePath, [string[]] $ArgumentList) {
    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath exited with code $LASTEXITCODE"
    }
}

function Get-GoEnv([string] $Name) {
    $value = & go env $Name
    if ($LASTEXITCODE -ne 0) {
        throw "go env $Name exited with code $LASTEXITCODE"
    }
    $value.Trim()
}

function Get-ProjectTarget {
    (& go tool dist list) | Where-Object { $_ -match '^(linux|darwin|windows)/(386|amd64|arm64)$' }
    if ($LASTEXITCODE -ne 0) {
        throw "go tool dist list exited with code $LASTEXITCODE"
    }
}

function Get-Version {
    $gitVersion = & git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($gitVersion)) {
        return $gitVersion.Trim()
    }
    'dev'
}

function Get-DefaultOutput([string] $AppName, [string] $OS, [string] $Architecture) {
    $suffix = if ($OS -eq 'windows') { '.exe' } else { '' }
    Join-Path 'dist' "$($AppName)_$($OS)_$($Architecture)$suffix"
}

function Test-Command([string] $Name) {
    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

Set-Location (Split-Path -Parent $PSCommandPath)

if ($ListTargets) {
    Get-ProjectTarget
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Name)) {
    $Name = 'lsx-server'
}

if ([string]::IsNullOrWhiteSpace($TargetOS)) {
    $TargetOS = Get-GoEnv 'GOOS'
}

if ([string]::IsNullOrWhiteSpace($Arch)) {
    $Arch = Get-GoEnv 'GOARCH'
}

if (-not [string]::IsNullOrWhiteSpace($Target)) {
    $TargetOS, $Arch = $Target -split '/', 2
}

$osAliases = @{
    macos = 'darwin'
}
$archAliases = @{
    x64     = 'amd64'
    aarch64 = 'arm64'
}

$TargetOS = $TargetOS.ToLowerInvariant()
$Arch = $Arch.ToLowerInvariant()
if ($osAliases.ContainsKey($TargetOS)) {
    $TargetOS = $osAliases[$TargetOS]
}
if ($archAliases.ContainsKey($Arch)) {
    $Arch = $archAliases[$Arch]
}

$targetPair = "$TargetOS/$Arch"
$targets = @(Get-ProjectTarget)
if ($targetPair -notin $targets) {
    Write-Error "Unsupported target: $targetPair`n`nSupported project targets:`n$($targets -join "`n")"
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = Get-Version
}

if ([string]::IsNullOrWhiteSpace($Out)) {
    $Out = Get-DefaultOutput $Name $TargetOS $Arch
}

$cgoEnabled = if ([string]::IsNullOrWhiteSpace($env:CGO_ENABLED)) { '0' } else { $env:CGO_ENABLED }
$hostOS = Get-GoEnv 'GOHOSTOS'
$hostArch = Get-GoEnv 'GOHOSTARCH'

Write-Information 'Generating embedded browser assets...'
$env:GOOS = $hostOS
$env:GOARCH = $hostArch
Invoke-Native go @('generate', './assets')

if (-not $SkipTests) {
    if (-not $SkipChecks) {
        Write-Information 'Checking formatting...'
        $goFiles = Get-ChildItem -Recurse -Filter '*.go' |
            Where-Object { $_.FullName -notmatch '[\\/]+vendor[\\/]+' } |
            ForEach-Object { $_.FullName }
        $unformatted = & gofmt -l @goFiles
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt exited with code $LASTEXITCODE"
        }
        if ($unformatted) {
            throw "gofmt required for:`n$($unformatted -join "`n")"
        }

        Write-Information 'Running go vet...'
        $env:GOOS = $hostOS
        $env:GOARCH = $hostArch
        Invoke-Native go @('vet', './...')
    }

    Write-Information 'Running tests...'
    $env:GOOS = $hostOS
    $env:GOARCH = $hostArch
    Invoke-Native go @('test', './...')

    if (-not $SkipChecks) {
        if (-not (Test-Command 'golangci-lint')) {
            throw 'golangci-lint is required for build checks; install it or pass -SkipChecks'
        }

        Write-Information 'Running golangci-lint...'
        $env:GOOS = $hostOS
        $env:GOARCH = $hostArch
        Invoke-Native golangci-lint @('run', './...')

        Write-Information 'Running modern Go lint checks...'
        Invoke-Native golangci-lint @(
            'run',
            '--enable=modernize',
            '--enable=usestdlibvars',
            '--enable=exptostd',
            '--enable=prealloc',
            '--enable=perfsprint',
            '--enable=gocritic',
            './...'
        )
    }
}

$outDir = Split-Path -Parent $Out
if (-not [string]::IsNullOrWhiteSpace($outDir)) {
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
}

$buildArgs = @(
    'build',
    '-trimpath',
    '-ldflags', "-s -w -X main.version=$Version",
    '-o', $Out,
    '.'
)

Write-Information "Building optimized $targetPair binary: $Out"
$env:CGO_ENABLED = $cgoEnabled
$env:GOOS = $TargetOS
$env:GOARCH = $Arch
Invoke-Native go $buildArgs
Write-Information "Built $Out"
