# vyx — Getting Started installer for Windows (PowerShell 5.1+)
# Usage: irm https://raw.githubusercontent.com/ElioNeto/vyx/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$VYX_MIN_GO     = [Version]'1.22'
$VYX_MIN_NODE   = 20
$VYX_MIN_PYTHON = [Version]'3.11'

function Write-Vyx  { param($msg) Write-Host "[vyx] $msg" -ForegroundColor Cyan }
function Write-Ok   { param($msg) Write-Host "[vyx] OK   $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[vyx] WARN $msg" -ForegroundColor Yellow }
function Write-Fail { param($msg) Write-Host "[vyx] ERR  $msg" -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "  ██╗   ██╗██╗   ██╗██╗  ██╗" -ForegroundColor Cyan
Write-Host "  ██║   ██║╚██╗ ██╔╝╚██╗██╔╝" -ForegroundColor Cyan
Write-Host "  ██║   ██║ ╚████╔╝  ╚███╔╝ " -ForegroundColor Cyan
Write-Host "  ╚██████╔╝  ╚██╔╝   ██╔██╗ " -ForegroundColor Cyan
Write-Host "   ╚═════╝    ╚═╝   ╚═╝  ╚═╝" -ForegroundColor Cyan
Write-Host "  Polyglot Full-Stack Framework"
Write-Host ""

# ── 1. Check prerequisites ────────────────────────────────────────────────────
Write-Vyx "Checking prerequisites..."

# Go
try {
  $goRaw = & go version 2>&1
  if ($goRaw -match 'go(\d+\.\d+)') {
    $goVer = [Version]$Matches[1]
    if ($goVer -ge $VYX_MIN_GO) { Write-Ok "Go $goVer found" }
    else { Write-Fail "Go $VYX_MIN_GO+ required (found $goVer). Download: https://go.dev/dl/" }
  }
} catch {
  Write-Fail "Go not found. Install Go $($VYX_MIN_GO)+: https://go.dev/dl/"
}

# Node.js
try {
  $nodeRaw = (& node --version 2>&1).ToString().TrimStart('v')
  $nodeMajor = [int]($nodeRaw -split '\.')[0]
  if ($nodeMajor -ge $VYX_MIN_NODE) { Write-Ok "Node.js v$nodeRaw found" }
  else { Write-Warn "Node.js $VYX_MIN_NODE+ recommended (found v$nodeRaw). Download: https://nodejs.org/" }
} catch {
  Write-Warn "Node.js not found (required for Node workers and SSR). Download: https://nodejs.org/"
}

# Python
try {
  $pyRaw = & python --version 2>&1
  if ($pyRaw -match '(\d+\.\d+)') {
    $pyVer = [Version]$Matches[1]
    if ($pyVer -ge $VYX_MIN_PYTHON) { Write-Ok "Python $pyVer found" }
    else { Write-Warn "Python $VYX_MIN_PYTHON+ recommended (found $pyVer). Download: https://python.org/" }
  }
} catch {
  Write-Warn "Python not found (required for Python workers). Download: https://python.org/"
}

# Git
try {
  & git --version | Out-Null
  Write-Ok "Git found"
} catch {
  Write-Fail "Git not found. Install: https://git-scm.com/"
}

# ── 2. Clone repository ───────────────────────────────────────────────────────
$dest = if ($args.Count -gt 0) { $args[0] } else { 'vyx-project' }

if (Test-Path $dest) {
  Write-Warn "Directory '$dest' already exists. Skipping clone."
} else {
  Write-Vyx "Cloning vyx into .\$dest ..."
  & git clone --depth 1 https://github.com/ElioNeto/vyx.git $dest
  Write-Ok "Repository cloned into .\$dest"
}

Set-Location $dest

# ── 3. Build the core ─────────────────────────────────────────────────────────
Write-Vyx "Building vyx core..."

New-Item -ItemType Directory -Force -Path '.\bin' | Out-Null
Set-Location core
& go mod download
& go build -o '..\bin\vyx.exe' '.\cmd\vyx'
Set-Location ..

$binPath = "$((Get-Location).Path)\bin"
Write-Ok "vyx binary built at $binPath\vyx.exe"

# ── 4. PATH hint ──────────────────────────────────────────────────────────────
Write-Host ""
Write-Host ("━" * 45) -ForegroundColor Green
Write-Host "  vyx is ready! 🚀" -ForegroundColor Green
Write-Host ("━" * 45) -ForegroundColor Green
Write-Host ""
Write-Host "  Add vyx to your PATH (current session):"
Write-Host ""
Write-Host "    `$env:PATH += ';$binPath'" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Or permanently (run as Administrator):"
Write-Host ""
Write-Host "    [Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$binPath', 'Machine')" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Start your first project:"
Write-Host ""
Write-Host "    vyx new my-app" -ForegroundColor Yellow
Write-Host "    cd my-app" -ForegroundColor Yellow
Write-Host "    vyx dev" -ForegroundColor Yellow
Write-Host ""
