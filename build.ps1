<#
.SYNOPSIS
Builds optimized release binaries.

.DESCRIPTION
Generates embedded browser assets, optionally runs checks, and builds trimmed
release binaries for Windows, Linux, or macOS.

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
    [ValidateSet('linux', 'darwin', 'mac', 'macos', 'osx', 'windows', 'win')]
    [string] $TargetOS = $env:GOOS,

    [Alias('Architecture')]
    [ValidateSet('386', 'amd64', 'x64', 'x86_64', 'arm64', 'aarch64', 'x86', 'i386', 'i686')]
    [string] $Arch = $env:GOARCH,

    [string] $Out = $env:OUT,
    [string] $Name = $env:APP_NAME,
    [string] $Version = $env:VERSION,
    [switch] $SkipTests,
    [switch] $SkipChecks,
    [switch] $SkipTools,
    [switch] $ListTargets
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$InformationPreference = 'Continue'

function Invoke-Native {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [string[]] $ArgumentList = @()
    )

    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath exited with code $LASTEXITCODE"
    }
}

function Invoke-InDirectory {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [Parameter(Mandatory)]
        [scriptblock] $ScriptBlock
    )

    Push-Location $Path
    try {
        & $ScriptBlock
    } finally {
        Pop-Location
    }
}

function Invoke-WithGoEnv {
    param(
        [Parameter(Mandatory)]
        [string] $GOOS,

        [Parameter(Mandatory)]
        [string] $GOARCH,

        [Parameter(Mandatory)]
        [string] $CGO_ENABLED,

        [Parameter(Mandatory)]
        [scriptblock] $ScriptBlock
    )

    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH
    $oldCGO = $env:CGO_ENABLED
    try {
        $env:GOOS = $GOOS
        $env:GOARCH = $GOARCH
        $env:CGO_ENABLED = $CGO_ENABLED
        & $ScriptBlock
    } finally {
        $env:GOOS = $oldGOOS
        $env:GOARCH = $oldGOARCH
        $env:CGO_ENABLED = $oldCGO
    }
}

function Get-GoEnv {
    param(
        [Parameter(Mandatory)]
        [string] $Name
    )

    $value = & go env $Name
    if ($LASTEXITCODE -ne 0) {
        throw "go env $Name exited with code $LASTEXITCODE"
    }
    $value.Trim()
}

function Get-ProjectTarget {
    $targets = & go tool dist list
    if ($LASTEXITCODE -ne 0) {
        throw "go tool dist list exited with code $LASTEXITCODE"
    }
    $targets | Where-Object { $_ -match '^(linux|darwin|windows)/(386|amd64|arm64)$' }
}

function Get-Version {
    $gitVersion = & git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($gitVersion)) {
        return $gitVersion.Trim()
    }
    'dev'
}

function Normalize-Target {
    param(
        [Parameter(Mandatory)]
        [string] $OS,

        [Parameter(Mandatory)]
        [string] $Architecture
    )

    $normalizedOS = $OS.ToLowerInvariant()
    $normalizedArch = $Architecture.ToLowerInvariant()

    switch ($normalizedOS) {
        'mac' { $normalizedOS = 'darwin' }
        'macos' { $normalizedOS = 'darwin' }
        'osx' { $normalizedOS = 'darwin' }
        'win' { $normalizedOS = 'windows' }
    }

    switch ($normalizedArch) {
        'x64' { $normalizedArch = 'amd64' }
        'x86_64' { $normalizedArch = 'amd64' }
        'aarch64' { $normalizedArch = 'arm64' }
        'x86' { $normalizedArch = '386' }
        'i386' { $normalizedArch = '386' }
        'i686' { $normalizedArch = '386' }
    }

    "$normalizedOS/$normalizedArch"
}

function Get-TargetSuffix {
    param(
        [Parameter(Mandatory)]
        [string] $OS
    )

    if ($OS -eq 'windows') {
        return '.exe'
    }
    ''
}

function Get-DefaultOutput {
    param(
        [Parameter(Mandatory)]
        [string] $AppName,

        [Parameter(Mandatory)]
        [string] $OS,

        [Parameter(Mandatory)]
        [string] $Architecture
    )

    Join-Path 'dist' "$($AppName)_$($OS)_$($Architecture)$(Get-TargetSuffix $OS)"
}

function Test-Command {
    param(
        [Parameter(Mandatory)]
        [string] $Name
    )

    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Invoke-GoBuild {
    param(
        [Parameter(Mandatory)]
        [string] $Label,

        [Parameter(Mandatory)]
        [string] $Package,

        [Parameter(Mandatory)]
        [string] $OutputPath,

        [Parameter(Mandatory)]
        [string] $CgoEnabled
    )

    $outDir = Split-Path -Parent $OutputPath
    if (-not [string]::IsNullOrWhiteSpace($outDir)) {
        New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    }

    $buildArgs = @(
        '-C', 'go',
        'build',
        '-trimpath',
        '-tags', 'webdist',
        '-ldflags', "-s -w -X main.version=$Version",
        '-o', $OutputPath,
        $Package
    )

    Write-Information "Building optimized $targetPair $Label binary: $OutputPath"
    Invoke-WithGoEnv -GOOS $TargetOS -GOARCH $Arch -CGO_ENABLED $CgoEnabled -ScriptBlock {
        Invoke-Native go $buildArgs
    }
    Write-Information "Built $OutputPath"
}

function Get-ToolCgoEnabled {
    param(
        [Parameter(Mandatory)]
        [string] $ToolName,

        [Parameter(Mandatory)]
        [bool] $RequiresCgo
    )

    if (-not $RequiresCgo) {
        return '0'
    }

    if (-not [string]::IsNullOrWhiteSpace($env:TOOL_CGO_ENABLED)) {
        if ($env:TOOL_CGO_ENABLED -eq '0') {
            Write-Information "Skipping $ToolName; it requires CGO and TOOL_CGO_ENABLED=0."
            return $null
        }
        return $env:TOOL_CGO_ENABLED
    }

    if ($targetPair -ne $hostPair) {
        Write-Information "Skipping $ToolName for cross target $targetPair; set TOOL_CGO_ENABLED=1 when a CGO cross toolchain is configured."
        return $null
    }

    if ($checkCgoEnabled -eq '0') {
        Write-Information "Skipping $ToolName; it requires CGO and go env CGO_ENABLED is 0."
        return $null
    }

    $checkCgoEnabled
}

function Invoke-WebBuild {
    Write-Information 'Building Svelte browser assets...'
    if (-not (Test-Command 'npm')) {
        throw 'npm is required to build browser assets'
    }

    Invoke-Native npm @('--prefix', 'go/web', 'ci')
    Invoke-Native npm @('--prefix', 'go/web', 'rebuild', 'rolldown')
    Invoke-Native npm @('--prefix', 'go/web', 'run', 'generate:openapi')
    if (-not $SkipTests -and -not $SkipChecks) {
        Invoke-Native npm @('--prefix', 'go/web', 'run', 'check')
    }
    Invoke-Native npm @('--prefix', 'go/web', 'run', 'build')
}

function Test-GoFormat {
    Write-Information 'Checking formatting...'
    $goFiles = Get-ChildItem -Path 'go' -Recurse -Filter '*.go' |
        Where-Object { $_.FullName -notmatch '[\/]+vendor[\/]+' } |
        ForEach-Object { $_.FullName }

    if (-not $goFiles) {
        return
    }

    $unformatted = & gofmt -l @goFiles
    if ($LASTEXITCODE -ne 0) {
        throw "gofmt exited with code $LASTEXITCODE"
    }
    if ($unformatted) {
        throw "gofmt required for:`n$($unformatted -join "`n")"
    }
}

function Invoke-GhidraCompile {
    if (-not [string]::IsNullOrWhiteSpace($env:GHIDRA_INSTALL_DIR) -or (Test-Command 'ghidra-analyzeHeadless')) {
        Write-Information 'Compiling Ghidra scripts...'
        if (Test-Command 'gradle') {
            Invoke-Native gradle @('-p', 'decompiled/ghidra_scripts', 'compileJava')
        } elseif (Test-Command 'bash') {
            Invoke-Native bash @('./go/tools/compile-ghidra-scripts.sh')
        } else {
            throw 'gradle or bash is required to compile Ghidra scripts'
        }
    } else {
        Write-Information 'Skipping Ghidra script compile; set GHIDRA_INSTALL_DIR or add ghidra-analyzeHeadless to PATH to enable it.'
    }
}

function Invoke-ChecksAndTests {
    if ($SkipTests) {
        return
    }

    if (-not $SkipChecks) {
        Test-GoFormat

        Write-Information 'Running go vet...'
        Invoke-WithGoEnv -GOOS $hostOS -GOARCH $hostArch -CGO_ENABLED $checkCgoEnabled -ScriptBlock {
            Invoke-Native go @('-C', 'go', 'vet', './...')
        }

        Write-Information 'Checking generated LT2 findings index...'
        Invoke-WithGoEnv -GOOS $hostOS -GOARCH $hostArch -CGO_ENABLED $checkCgoEnabled -ScriptBlock {
            Invoke-Native go @('-C', 'go', 'run', './tools/lt2findings', '-check')
        }

        Invoke-GhidraCompile
    }

    Write-Information 'Running tests...'
    Invoke-WithGoEnv -GOOS $hostOS -GOARCH $hostArch -CGO_ENABLED $checkCgoEnabled -ScriptBlock {
        Invoke-Native go @('-C', 'go', 'test', './...')
    }

    if (-not $SkipChecks) {
        if (-not (Test-Command 'golangci-lint')) {
            throw 'golangci-lint is required for build checks; install it or pass -SkipChecks'
        }

        Write-Information 'Running golangci-lint...'
        Invoke-InDirectory -Path 'go' -ScriptBlock {
            Invoke-WithGoEnv -GOOS $hostOS -GOARCH $hostArch -CGO_ENABLED $checkCgoEnabled -ScriptBlock {
                Invoke-Native golangci-lint @('run', './...')
            }
        }

        Write-Information 'Running modern Go lint checks...'
        Invoke-InDirectory -Path 'go' -ScriptBlock {
            Invoke-WithGoEnv -GOOS $hostOS -GOARCH $hostArch -CGO_ENABLED $checkCgoEnabled -ScriptBlock {
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
    }
}

Set-Location $PSScriptRoot

if ($ListTargets) {
    Get-ProjectTarget
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Name)) {
    $Name = 'lsx-server'
}

if (-not [string]::IsNullOrWhiteSpace($Target)) {
    $TargetOS, $Arch = $Target -split '/', 2
}

if ([string]::IsNullOrWhiteSpace($TargetOS)) {
    $TargetOS = Get-GoEnv 'GOOS'
}
if ([string]::IsNullOrWhiteSpace($Arch)) {
    $Arch = Get-GoEnv 'GOARCH'
}

$targetPair = Normalize-Target $TargetOS $Arch
$TargetOS, $Arch = $targetPair -split '/', 2
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
if (-not [System.IO.Path]::IsPathRooted($Out)) {
    $Out = Join-Path (Get-Location) $Out
}

$checkCgoEnabled = if ([string]::IsNullOrWhiteSpace($env:CHECK_CGO_ENABLED)) { Get-GoEnv 'CGO_ENABLED' } else { $env:CHECK_CGO_ENABLED }
$hostOS = Get-GoEnv 'GOHOSTOS'
$hostArch = Get-GoEnv 'GOHOSTARCH'
$hostPair = Normalize-Target $hostOS $hostArch

$toolCommands = @(
    @{ Name = 'lt2rb'; Package = './tools/lt2rb'; RequiresCgo = $false },
    @{ Name = 'lt2findings'; Package = './tools/lt2findings'; RequiresCgo = $true },
    @{ Name = 'lt2install'; Package = './tools/lt2install'; RequiresCgo = $false },
    @{ Name = 'lt2keygen'; Package = './tools/lt2keygen'; RequiresCgo = $false },
    @{ Name = 'lt2normalize'; Package = './tools/lt2normalize'; RequiresCgo = $false }
)

Invoke-WebBuild
Invoke-ChecksAndTests
Invoke-GoBuild $Name '.' $Out '0'

if (-not $SkipTools) {
    $toolDir = Split-Path -Parent $Out
    if ([string]::IsNullOrWhiteSpace($toolDir)) {
        $toolDir = '.'
    }

    $suffix = Get-TargetSuffix $TargetOS
    foreach ($tool in $toolCommands) {
        $toolCgoEnabled = Get-ToolCgoEnabled $tool.Name $tool.RequiresCgo
        if ($null -ne $toolCgoEnabled) {
            $toolOut = Join-Path $toolDir "$($tool.Name)_$($TargetOS)_$($Arch)$suffix"
            Invoke-GoBuild $tool.Name $tool.Package $toolOut $toolCgoEnabled
        }
    }
}
