<#
.SYNOPSIS
    Reproduksi temuan SEC-AGT-PANIC (CRITICAL) pada SIM-KIPAN Core API.

.DESCRIPTION
    Skrip ini membuktikan bahwa server MENOLAK start (panic) karena nama
    kebijakan rate limit "agt_pub" dipakai di wiring rute
    (internal/handler/routes.go) tetapi TIDAK terdaftar pada peta
    ratePolicies (internal/middleware/ratelimit.go).

    Middleware.RateLimit() sengaja fail-fast (panic) saat nama tak dikenal,
    dan panic itu terjadi di main() saat wiring rute — DI LUAR middleware
    recover Fiber — sehingga seluruh proses mati sebelum listen.

    Langkah:
      1. Build binary API.
      2. Jalankan server dengan guard timeout, tangkap stdout + stderr.
      3. Deteksi pola "panic" + nama kebijakan yang hilang + lokasi stack.
      4. Pastikan /health tidak dapat diakses (outage total).
      5. Cetak verdict.

    Non-destruktif: hanya menjalankan proses lokal; tidak menyentuh data.

.PARAMETER TimeoutSeconds
    Batas tunggu server (detik) sebelum dianggap "hidup". Default 30.

.PARAMETER Port
    Port yang diasumsikan dipakai API (default 8080).

.OUTPUTS
    Exit code (mengikuti konvensi release gate proyek):
      1 = VULNERABILITY CONFIRMED (server panic / API tidak listen) -> BLOCKED
      0 = tidak tereproduksi (server start normal -> kemungkinan sudah diperbaiki)
      2 = gagal build / prasyarat tidak terpenuhi

.EXAMPLE
    cd backend
    ./scripts/repro_agt_panic.ps1
#>
[CmdletBinding()]
param(
    [int]$TimeoutSeconds = 30,
    [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

$scriptDir  = Split-Path -Parent $MyInvocation.MyCommand.Path
$moduleRoot = Split-Path -Parent $scriptDir
$binDir     = Join-Path $moduleRoot "bin"
$exePath    = Join-Path $binDir "kipan-api-pentest.exe"
$outLog     = Join-Path $binDir "repro_agt_panic.out.log"
$errLog     = Join-Path $binDir "repro_agt_panic.err.log"
$policyName = "agt_pub"
$baseUrl    = "http://127.0.0.1:$Port"

function Write-Section([string]$text) {
    Write-Host ""
    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host $text -ForegroundColor Cyan
    Write-Host "==========================================================" -ForegroundColor Cyan
}

Write-Section "REPRO SEC-AGT-PANIC — panic wiring rate limit '$policyName'"

if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
Remove-Item $exePath, $outLog, $errLog -ErrorAction SilentlyContinue

# --- 1. Build -------------------------------------------------------------
Write-Host "[1/4] Build binary API..." -ForegroundColor Yellow
Push-Location $moduleRoot
try {
    $build = & go build -o $exePath ./cmd/api 2>&1
    $buildCode = $LASTEXITCODE
}
finally {
    Pop-Location
}

if ($buildCode -ne 0 -or -not (Test-Path $exePath)) {
    Write-Host "GAGAL build — prasyarat tidak terpenuhi." -ForegroundColor Red
    $build | Out-String | Write-Host
    exit 2
}
Write-Host "      OK -> $exePath" -ForegroundColor Green

# --- 2. Jalankan server dengan guard --------------------------------------
Write-Host "[2/4] Menjalankan server (guard ${TimeoutSeconds}s)..." -ForegroundColor Yellow
$proc = Start-Process -FilePath $exePath -WorkingDirectory $moduleRoot -NoNewWindow `
    -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru

$exited = $proc.WaitForExit($TimeoutSeconds * 1000)

# Cek keterjangkauan SELAGI proses hidup (sebelum kill): setelah fix,
# server tetap jalan sehingga health harus 200; sebelum fix proses sudah
# exit via panic sehingga health refused.
$reachable = $false
if (-not $exited) {
    try {
        $resp = Invoke-WebRequest -Uri "$baseUrl/health" -TimeoutSec 5
        if ($resp.StatusCode -eq 200) { $reachable = $true }
    }
    catch {
        $reachable = $false
    }
    $proc.Kill()
    if (-not $reachable) {
        Write-Host "      Server hidup tetapi /health tidak merespons (startup lambat? periksa log)." -ForegroundColor Yellow
    } else {
        Write-Host "      Server hidup + /health 200 (kemungkinan sudah diperbaiki)." -ForegroundColor Green
    }
} else {
    Write-Host "      Server exit sendiri (kemungkinan panic)." -ForegroundColor Yellow
}
Start-Sleep -Milliseconds 500

# --- 3. Analisis log ------------------------------------------------------
Write-Host "[3/4] Menganalisis output server..." -ForegroundColor Yellow
$errText  = if (Test-Path $errLog) { Get-Content $errLog -Raw } else { "" }
$outText  = if (Test-Path $outLog) { Get-Content $outLog -Raw } else { "" }
$combined = "$errText`n$outText"

$hasPanic  = $combined -match [regex]::Escape("panic:")
$hasPolicy = $combined -match [regex]::Escape($policyName)
$atWiring  = $combined -match "ratelimit\.go" -and $combined -match "routes\.go"

Write-Host ""
Write-Host "----- cuplikan output -----" -ForegroundColor DarkGray
$combined -split "`n" |
    Where-Object { $_ -match "panic|RateLimit|ratelimit\.go|routes\.go|main\.go" } |
    ForEach-Object { Write-Host ("  " + $_.TrimEnd()) }
Write-Host "---------------------------" -ForegroundColor DarkGray

# --- 4. Cek keterjangkauan API (sudah dilakukan selagi proses hidup) -----
Write-Host "[4/4] Hasil pemeriksaan keterjangkauan ($baseUrl/health, selagi proses hidup)..." -ForegroundColor Yellow
Write-Host ("      reachable = {0}" -f $reachable)

# --- Verdict --------------------------------------------------------------
Write-Section "VERDICT"
Write-Host ("  panic terdeteksi      : {0}" -f $hasPanic)  -ForegroundColor DarkGray
Write-Host ("  '$policyName' disebut : {0}" -f $hasPolicy) -ForegroundColor DarkGray
Write-Host ("  panic di wiring rute  : {0}" -f $atWiring)  -ForegroundColor DarkGray
Write-Host ("  /health reachable     : {0}" -f $reachable) -ForegroundColor DarkGray
Write-Host ""

if ($hasPanic -and $hasPolicy) {
    Write-Host "RELEASE GATE: BLOCKED — SEC-AGT-PANIC TERBUKTI." -ForegroundColor Red
    Write-Host "Seluruh endpoint API tidak dapat diakses (availability outage)." -ForegroundColor Red
    Write-Host "Perbaikan: daftarkan kebijakan '$policyName' pada ratePolicies di" -ForegroundColor Yellow
    Write-Host "internal/middleware/ratelimit.go, lalu jalankan ulang skrip ini." -ForegroundColor Yellow
    exit 1
}

if (-not $reachable) {
    Write-Host "RELEASE GATE: BLOCKED — API tidak merespons (bukan panic wiring; periksa log server)." -ForegroundColor Red
    exit 1
}

Write-Host "RELEASE GATE: PASSED — panic tidak tereproduksi, API merespons 200." -ForegroundColor Green
Write-Host "Kemungkinan temuan sudah FIXED. Jalankan pentest_suite.ps1 untuk regresi penuh." -ForegroundColor DarkGray
exit 0
