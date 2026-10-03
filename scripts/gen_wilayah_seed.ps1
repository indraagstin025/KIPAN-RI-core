# Generator seed wilayah nasional dari wilayah.id (Kepmendagri 2025).
# Sekali jalan saat penyusunan migrasi (BUKAN runtime). Output:
#   scripts/wilayah-id-20250704/provinces.json + regencies/*.json + META.json
#   migrations/000014_seed_wilayah_nasional.up/.down.sql
# Idempoten: aman dijalankan ulang (fetch ulang + timpa file + emit ulang).
# Gagal keras bila validasi tidak lolos (jumlah, format, duplikat, karakter).

$ErrorActionPreference = 'Stop'
$Backend = 'C:\Users\Indra\Documents\Documents\Project\PROJECT_KIPAN_INDONESIA\backend'
$OutDir = Join-Path $Backend 'scripts\wilayah-id-20250704'
$RegDir = Join-Path $OutDir 'regencies'
$MigDir = Join-Path $Backend 'migrations'
$Base = 'https://wilayah.id/api'

New-Item -ItemType Directory -Force -Path $RegDir | Out-Null

function Get-Json($url) {
    $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 30
    return ($r.Content | ConvertFrom-Json)
}

function Assert-Name($name, $ctx) {
    if ([string]::IsNullOrWhiteSpace($name) -or $name.Length -gt 100) { throw "nama invalid [$ctx]: '$name'" }
    if ($name -match '[<>"\\]') { throw "nama mengandung karakter terlarang [$ctx]: '$name'" }
}

function Assert-NoDup($codes, $ctx) {
    $seen = @{}
    foreach ($c in $codes) { if ($seen.ContainsKey($c)) { throw "duplikat kode [$ctx]: $c" }; $seen[$c] = $true }
}

function Escape-Sql($s) { return "$s" -replace "'", "''" }

function Write-Utf8NoBom($path, $text) {
    [System.IO.File]::WriteAllText($path, $text, (New-Object System.Text.UTF8Encoding($false)))
}

# --- 1. Provinsi ---
$prov = Get-Json "$Base/provinces.json"
$upUpdated = "$($prov.meta.updated_at)"
if ([string]::IsNullOrWhiteSpace($upUpdated)) { throw "meta.updated_at upstream kosong (sumber tak dikenal)" }
$metaPath = Join-Path $OutDir 'META.json'
if (Test-Path -LiteralPath $metaPath) {
    try {
        $prev = (Get-Content -LiteralPath $metaPath -Raw | ConvertFrom-Json).upstream_updated_at
        if ($prev -and "$prev" -ne $upUpdated) {
            Write-Warning "DRIFT UPSTREAM: dulu '$prev', kini '$upUpdated'. Review diff sebelum migrate."
        }
    } catch { Write-Warning "META lama tak terbaca, lanjut tanpa pembanding." }
}
$provData = @($prov.data)
if ($provData.Count -ne 38) { throw "jumlah provinsi = $($provData.Count), harap 38" }
foreach ($p in $provData) {
    if ($p.code -notmatch '^\d{2}$') { throw "kode provinsi invalid: $($p.code)" }
    $p.name = "$($p.name)".Trim()
    Assert-Name $p.name "prov $($p.code)"
}
Assert-NoDup ($provData | ForEach-Object { $_.code }) 'provinsi'
Write-Utf8NoBom (Join-Path $OutDir 'provinces.json') ($provData | ConvertTo-Json -Depth 4)
Write-Output "provinsi OK: $($provData.Count)"

# --- 2. Kabupaten/kota per provinsi ---
$allKab = @()
foreach ($p in $provData) {
    $rj = Get-Json "$Base/regencies/$($p.code).json"
    $kab = @($rj.data)
    if ($kab.Count -eq 0) { throw "regencies kosong untuk prov $($p.code)" }
    foreach ($k in $kab) {
        if ($k.code -notmatch '^\d{2}\.\d{2}$') { throw "kode kab invalid [$($p.code)]: $($k.code)" }
        $k.name = "$($k.name)".Trim()
        Assert-Name $k.name "kab $($k.code)"
        $flat = $k.code -replace '\.', ''
        $allKab += [pscustomobject]@{ prov = $p.code; kode = $flat; nama = $k.name }
    }
    Write-Utf8NoBom (Join-Path $RegDir "$($p.code).json") ($kab | ConvertTo-Json -Depth 4)
    Write-Output "  prov $($p.code): $($kab.Count) kab/kota"
}
Assert-NoDup ($allKab | ForEach-Object { $_.kode }) 'kabupaten-global'
if ($allKab.Count -lt 500 -or $allKab.Count -gt 530) { throw "jumlah kab/kota = $($allKab.Count), di luar rentang 500-530" }
Write-Output "kabupaten/kota OK: $($allKab.Count)"

# --- 3. META (hash + jejak audit) ---
$files = @()
$files += [pscustomobject]@{ path = 'provinces.json'; sha256 = (Get-FileHash -LiteralPath (Join-Path $OutDir 'provinces.json') -Algorithm SHA256).Hash }
foreach ($p in $provData) {
    $fp = Join-Path $RegDir "$($p.code).json"
    $files += [pscustomobject]@{ path = "regencies/$($p.code).json"; sha256 = (Get-FileHash -LiteralPath $fp -Algorithm SHA256).Hash }
}
$meta = [pscustomobject]@{
    source = 'https://wilayah.id (Kepmendagri No 300.2.2-2138/2025)'
    upstream_updated_at = "$($prov.meta.updated_at)"
    fetched_at = (Get-Date).ToUniversalTime().ToString('o')
    provinces = $provData.Count
    regencies = $allKab.Count
    files = $files
}
Write-Utf8NoBom (Join-Path $OutDir 'META.json') ($meta | ConvertTo-Json -Depth 5)
Write-Output "META OK"

# --- 4. Emit migrasi 000014 ---
$header = @'
-- ============================================================
-- SEED WILAYAH NASIONAL (38 PROVINSI + KAB/KOTA)
-- Version: 000014_seed_wilayah_nasional.up.sql
-- ============================================================
-- Sumber: https://wilayah.id (Kepmendagri No 300.2.2-2138 Tahun 2025).
-- Dibangkitkan oleh scripts/gen_wilayah_seed.ps1 dari JSON vendored di
-- scripts/wilayah-id-20250704/ (lihat META.json untuk hash + tanggal).
-- JANGAN edit manual; generate ulang bila sumber berubah.
--
-- Sifat: IDEMPOTEN (aman dijalankan berkali-kali).
--   - ON CONFLICT (kode): pertahankan id (SERIAL stabil, FK aman),
--     normalisasi nama + aktifkan kembali.
--   - TANPA DELETE: baris existing tidak pernah dihapus.
-- ============================================================

'@
$provVals = ($provData | ForEach-Object { "('$($_.code)', '$(Escape-Sql $_.name)')" }) -join ",`n"
$kabVals = ($allKab | ForEach-Object { "('$($_.prov)', '$($_.kode)', '$(Escape-Sql $_.nama)')" }) -join ",`n"
$up = $header + @"
INSERT INTO wilayah_provinsi (kode, nama) VALUES
$provVals
ON CONFLICT (kode) DO UPDATE SET
  nama = EXCLUDED.nama,
  is_active = TRUE,
  updated_at = CURRENT_TIMESTAMP;

INSERT INTO wilayah_kabupaten (provinsi_id, kode, nama)
SELECT p.id, k.kode, k.nama FROM wilayah_provinsi p,
(VALUES
$kabVals
) AS k(prov_kode, kode, nama)
WHERE p.kode = k.prov_kode
ON CONFLICT (kode) DO UPDATE SET
  nama = EXCLUDED.nama,
  is_active = TRUE,
  updated_at = CURRENT_TIMESTAMP;
"@
Write-Utf8NoBom (Join-Path $MigDir '000014_seed_wilayah_nasional.up.sql') $up

$down = @'
-- ROLLBACK 000014_seed_wilayah_nasional
-- SENGAJA NON-DESTRUKTIF: data master wilayah dipertahankan agar FK
-- (pendaftaran/anggota/users) tidak yatim. Rollback = no-op.
SELECT 1;
'@
Write-Utf8NoBom (Join-Path $MigDir '000014_seed_wilayah_nasional.down.sql') $down
Write-Output "migrasi 000014 OK"
