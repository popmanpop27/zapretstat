#Requires -RunAsAdministrator
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Config ─────────────────────────────────────────────────────────────────────
$AppName    = "zapretstatd"
$ServiceName = "ZapretstatDaemon"
$DisplayName = "Zapretstat Daemon"
$Description = "Zapretstat background service"
$InstallDir  = "C:\Program Files\zapretstat"
$LogDir      = "C:\ProgramData\zapretstat\logs"
$BinaryPath  = Join-Path $InstallDir "$AppName.exe"
$ScriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Definition

# ── Helpers ────────────────────────────────────────────────────────────────────
function Write-Info  { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn  { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err   { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

# ── Check Go ───────────────────────────────────────────────────────────────────
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Err "Go is not installed. Download from https://go.dev/dl/"
}

# ── Build ──────────────────────────────────────────────────────────────────────
Write-Info "Building $AppName..."
Push-Location $ScriptDir
try {
    & go build -o "$env:TEMP\$AppName.exe" ".\zapretstatd\main.go"
    if ($LASTEXITCODE -ne 0) { Write-Err "go build failed" }
} finally {
    Pop-Location
}

# ── Install binary ─────────────────────────────────────────────────────────────
Write-Info "Installing binary to $InstallDir..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $LogDir     | Out-Null
Copy-Item -Force "$env:TEMP\$AppName.exe" $BinaryPath
Remove-Item -Force "$env:TEMP\$AppName.exe" -ErrorAction SilentlyContinue

# ── Stop & remove existing service ────────────────────────────────────────────
$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Warn "Removing existing service..."
    if ($existing.Status -eq "Running") {
        Stop-Service -Name $ServiceName -Force
    }
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 2
}

# ── Register Windows Service ───────────────────────────────────────────────────
# Вариант А: через sc.exe (встроен в Windows, без зависимостей)
# Вариант Б: через NSSM (раскомментировать ниже, если нужен stdout/stderr лог)

Write-Info "Registering Windows Service..."

# --- Вариант А: sc.exe ---
$binPathWithArgs = "`"$BinaryPath`""
sc.exe create $ServiceName `
    binPath= $binPathWithArgs `
    start= auto `
    DisplayName= $DisplayName | Out-Null

sc.exe description $ServiceName $Description | Out-Null

# Автоперезапуск при сбое: 3 попытки через 5 секунд, сброс счётчика через 1 час
sc.exe failure $ServiceName reset= 3600 actions= restart/5000/restart/5000/restart/5000 | Out-Null

# --- Вариант Б: NSSM (раскомментировать, если NSSM установлен) ---
# $nssmPath = (Get-Command nssm -ErrorAction SilentlyContinue)?.Source
# if (-not $nssmPath) { Write-Err "NSSM not found. Install via: winget install nssm" }
# & nssm install $ServiceName $BinaryPath
# & nssm set $ServiceName AppStdout (Join-Path $LogDir "stdout.log")
# & nssm set $ServiceName AppStderr (Join-Path $LogDir "stderr.log")
# & nssm set $ServiceName AppRotateFiles 1
# & nssm set $ServiceName Start SERVICE_AUTO_START

# ── Start ──────────────────────────────────────────────────────────────────────
Write-Info "Starting service..."
Start-Service -Name $ServiceName
$svc = Get-Service -Name $ServiceName
Write-Info "Service status: $($svc.Status)"

# ── Firewall (опционально, раскомментировать если нужен входящий порт) ─────────
# $port = 8080
# Write-Info "Opening firewall port $port..."
# New-NetFirewallRule -DisplayName $DisplayName -Direction Inbound `
#     -Protocol TCP -LocalPort $port -Action Allow -ErrorAction SilentlyContinue

Write-Host ""
Write-Info "Done! Useful commands:"
Write-Host "  Get-Service $ServiceName"
Write-Host "  Start-Service $ServiceName"
Write-Host "  Stop-Service $ServiceName"
Write-Host "  Restart-Service $ServiceName"
Write-Host "  sc.exe qc $ServiceName"
Write-Host "  Get-EventLog -LogName Application -Source $ServiceName -Newest 20"
Write-Host ""
Write-Host "  Logs (if NSSM): $LogDir"
