# ============================================================
# SIM-KIPAN Core — Migrasi: Preflight + Snapshot + Klon
# Tujuan : mencegah migrasi destruktif (Batch A/B) gagal atau merusak data.
#          Preflight menjalankan cek HARD (harus 0 baris) sebelum migrasi yang
#          menambah partial UNIQUE INDEX, dan cek INFO untuk ditinjau.
# Pakai  :
#   pwsh -File scripts/migrate-preflight.ps1
#   pwsh -File scripts/migrate-preflight.ps1 -Snapshot
#   pwsh -File scripts/migrate-preflight.ps1 -Snapshot -CloneTo kipan_core_clone
# Catatan: DATABASE_URL diambil dari env bila -DatabaseUrl tidak diisi.
# ============================================================
param(
    [string]$DatabaseUrl = $env:DATABASE_URL,
    [string]$PsqlPath = "",
    [string]$PgDumpPath = "",
    [string]$SnapshotDir = "backups",
    [switch]$Snapshot,
    [string]$CloneTo = ""
)

$ErrorActionPreference = "Stop"

# Resolve-Tool: pakai path eksplisit, lalu PATH, lalu default Laragon.
function Resolve-Tool {
    param([string]$Explicit, [string]$Name, [string]$Fallback)
    if ($Explicit -and (Test-Path $Explicit)) { return $Explicit }
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    if ($Fallback -and (Test-Path $Fallback)) { return $Fallback }
    throw "Tidak menemukan '$Name'. Isi -PsqlPath / -PgDumpPath atau tambahkan ke PATH."
}

if (-not $DatabaseUrl) {
    throw "DATABASE_URL kosong. Set env DATABASE_URL atau pakai -DatabaseUrl."
}

$laragonBin = "C:\laragon\bin\postgresql\postgresql-14.5-1\bin"
$psql = Resolve-Tool -Explicit $PsqlPath -Name "psql" -Fallback (Join-Path $laragonBin "psql.exe")
$pgDump = Resolve-Tool -Explicit $PgDumpPath -Name "pg_dump" -Fallback (Join-Path $laragonBin "pg_dump.exe")
$pgBinDir = Split-Path -Parent $pgDump

# Invoke-Sql: jalankan SQL, kembalikan baris (text, tuples-only).
function Invoke-Sql {
    param([string]$Sql)
    $out = & $psql -d $DatabaseUrl -tA -c $Sql 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "psql gagal: $out"
    }
    return @($out | Where-Object { $_ -ne "" })
}

Write-Host "== SIM-KIPAN migrasi preflight ==" -ForegroundColor Cyan

# --- Cek HARD (harus 0 baris sebelum migrasi partial unique) ---
$hardChecks = @(
    @{
        Name  = "Pengurus 'Aktif' ganda lintas SK (blocker partial unique anggota_id)"
        Sql   = "SELECT anggota_id || ' => ' || count(*) FROM pengurus WHERE status='Aktif' GROUP BY anggota_id HAVING count(*) > 1"
    },
    @{
        Name  = "Pendaftaran nik_hash non-DITOLAK ganda (blocker partial unique nik_hash)"
        Sql   = "SELECT left(nik_hash, 12) || '... => ' || count(*) FROM pendaftaran WHERE status <> 'DITOLAK' GROUP BY nik_hash HAVING count(*) > 1"
    }
)

$hardFailed = $false
foreach ($c in $hardChecks) {
    $rows = Invoke-Sql -Sql $c.Sql
    if ($rows.Count -gt 0) {
        $hardFailed = $true
        Write-Host "[HARD][GAGAL] $($c.Name)" -ForegroundColor Red
        $rows | ForEach-Object { Write-Host "        $_" -ForegroundColor Red }
    } else {
        Write-Host "[HARD][OK]    $($c.Name)" -ForegroundColor Green
    }
}

# --- Cek INFO (ditinjau; sebagian akan ditangani otomatis oleh migrasi) ---
$infoChecks = @(
    @{
        Name = "Jabatan nama ganda (akan didedupe migrasi A1/000026)"
        Sql  = "SELECT nama || ' => ' || count(*) FROM jabatan GROUP BY nama HAVING count(*) > 1"
    },
    @{
        Name = "SK DISETUJUI aktif ganda per wilayah (tinjau Single Active SK)"
        Sql  = "SELECT level || ' ' || coalesce(provinsi_id,0) || '/' || coalesce(kabupaten_id,0) || ' => ' || count(*) FROM surat_keputusan WHERE status='Aktif' AND approval_status='DISETUJUI' GROUP BY level, provinsi_id, kabupaten_id HAVING count(*) > 1"
    }
)

foreach ($c in $infoChecks) {
    $rows = Invoke-Sql -Sql $c.Sql
    if ($rows.Count -gt 0) {
        Write-Host "[INFO] $($c.Name): $($rows.Count) temuan" -ForegroundColor Yellow
        $rows | ForEach-Object { Write-Host "        $_" -ForegroundColor Yellow }
    } else {
        Write-Host "[INFO][BERSIH] $($c.Name)" -ForegroundColor Green
    }
}

# --- Snapshot (pg_dump custom format) ---
if ($Snapshot) {
    if (-not (Test-Path $SnapshotDir)) { New-Item -ItemType Directory -Path $SnapshotDir | Out-Null }
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $file = Join-Path $SnapshotDir "kipan-$stamp.dump"
    Write-Host "== Snapshot -> $file ==" -ForegroundColor Cyan
    & $pgDump -Fc -f $file $DatabaseUrl
    if ($LASTEXITCODE -ne 0) { throw "pg_dump gagal" }
    Write-Host "Snapshot selesai." -ForegroundColor Green
}

# --- Klon ke database lain (createdb + pg_restore) ---
if ($CloneTo) {
    $createdb = Join-Path $pgBinDir "createdb.exe"
    if (-not (Test-Path $createdb)) { $createdb = "createdb" }
    Write-Host "== Klon -> $CloneTo ==" -ForegroundColor Cyan

    if (-not (Test-Path $SnapshotDir)) { New-Item -ItemType Directory -Path $SnapshotDir | Out-Null }
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $file = Join-Path $SnapshotDir "kipan-clone-$stamp.dump"
    & $pgDump -Fc -f $file $DatabaseUrl
    if ($LASTEXITCODE -ne 0) { throw "pg_dump gagal" }

    & $createdb $CloneTo 2>&1 | Out-Null
    # createdb boleh gagal bila DB sudah ada; lanjutkan restore (drop manual bila perlu).
    $cloneUrl = ($DatabaseUrl -replace "/[^/?]+(\?|$)", "/$CloneTo`$1")
    & $pgDump --version | Out-Null
    & (Join-Path $pgBinDir "pg_restore.exe") -d $cloneUrl --clean --if-exists $file
    if ($LASTEXITCODE -ne 0) { throw "pg_restore gagal" }
    Write-Host "Klon selesai -> $CloneTo" -ForegroundColor Green
}

if ($hardFailed) {
    Write-Host "PREFLIGHT GAGAL: bersihkan data pada cek HARD sebelum migrasi." -ForegroundColor Red
    exit 1
}
Write-Host "PREFLIGHT OK." -ForegroundColor Green
exit 0
