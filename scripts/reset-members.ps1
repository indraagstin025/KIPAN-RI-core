# ============================================================
# SIM-KIPAN — Reset Data Anggota & Pengurus (DEV/STAGING)
# Tujuan : Hapus seluruh anggota, pengurus, pendaftaran, SK, akun USER
#          anggota, dan antrian email; reset penomoran NIA/registrasi.
# Pakai  :
#   pwsh -File scripts/reset-members.ps1            # dry-run (hitung saja)
#   pwsh -File scripts/reset-members.ps1 -Yes       # eksekusi
# Peringatan: DESTRUKTIF. Backup dulu
#   (pwsh -File scripts/migrate-preflight.ps1 -Snapshot).
# ============================================================
param(
    [string]$DatabaseUrl = $env:DATABASE_URL,
    [string]$PsqlPath = "",
    [switch]$Yes
)

$ErrorActionPreference = "Stop"

function Resolve-Tool {
    param([string]$Explicit, [string]$Name, [string]$Fallback)
    if ($Explicit -and (Test-Path $Explicit)) { return $Explicit }
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    if ($Fallback -and (Test-Path $Fallback)) { return $Fallback }
    throw "Tidak menemukan '$Name'. Isi -PsqlPath atau tambahkan ke PATH."
}

if (-not $DatabaseUrl) {
    throw "DATABASE_URL kosong. Set env DATABASE_URL atau pakai -DatabaseUrl."
}

$laragonBin = "C:\laragon\bin\postgresql\postgresql-14.5-1\bin"
$psql = Resolve-Tool -Explicit $PsqlPath -Name "psql" -Fallback (Join-Path $laragonBin "psql.exe")
$sqlFile = Join-Path $PSScriptRoot "reset-members.sql"

function Get-Counts {
    $q = "SELECT 'anggota=' || (SELECT count(*) FROM anggota) || ' pengurus=' || (SELECT count(*) FROM pengurus) || ' pendaftaran=' || (SELECT count(*) FROM pendaftaran) || ' sk=' || (SELECT count(*) FROM surat_keputusan) || ' user_anggota=' || (SELECT count(*) FROM users WHERE role='USER') || ' outbox=' || (SELECT count(*) FROM email_outbox);"
    return (& $psql -d $DatabaseUrl -tA -c $q)
}

Write-Host "== SIM-KIPAN reset anggota & pengurus ==" -ForegroundColor Cyan
Write-Host ("SEBELUM: " + (Get-Counts))

if (-not $Yes) {
    Write-Host "DRY-RUN (tidak menghapus). Tambahkan -Yes untuk mengeksekusi." -ForegroundColor Yellow
    exit 0
}

& $psql -d $DatabaseUrl -v ON_ERROR_STOP=1 -f $sqlFile
if ($LASTEXITCODE -ne 0) { throw "reset gagal (lihat pesan psql di atas)" }

Write-Host ("SESUDAH: " + (Get-Counts))
Write-Host "Reset selesai." -ForegroundColor Green
