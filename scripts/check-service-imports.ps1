# check-service-imports.ps1 — mengunci aturan dependensi subpackage service.
# Gagal (exit 1) bila: ada impor antar-subpackage di luar allowlist,
# testutil dipakai kode non-test, atau package service root berisi kode.
# Jalankan dari backend/: pwsh scripts/check-service-imports.ps1

$ErrorActionPreference = 'Stop'
$fail = $false

function Fail($msg) {
    Write-Host "IMPORT-CHECK FAIL: $msg" -ForegroundColor Red
    $script:fail = $true
}

# Subpackage service => service/* lain yang BOLEH diimpor (selain svcutil/mail
# generik di bawah, yang dicantumkan eksplisit per paket).
$allow = @{
    'anggota'      = @('svcutil')
    'auth'         = @('mail', 'svcutil')
    'dokumen'      = @('svcutil')
    'insight'      = @('svcutil')
    'kepengurusan' = @('mail', 'svcutil')
    'mail'         = @()
    'notify'       = @('mail', 'svcutil')
    'pendaftaran'  = @('dokumen', 'mail', 'notify', 'svcutil')
    'platform'     = @('svcutil')
    'svcutil'      = @()
    'testutil'     = @()
    'users'        = @('svcutil')
    'wilayah'      = @('svcutil')
}

$prefix = 'github.com/kipan-indonesia/sim-kipan-core/internal/service/'

# 1) Package service root hanya boleh berisi doc.go (tanpa kode).
$rootFiles = (go list -f '{{join .GoFiles ","}}' ./internal/service).Trim()
if ($rootFiles -ne 'doc.go') {
    Fail "package service root harus hanya berisi doc.go, dapat: [$rootFiles]"
}

# 2) Impor antar-subpackage harus dalam allowlist (siklik otomatis gagal di go list).
foreach ($pkg in $allow.Keys) {
    $line = go list -f '{{.Name}}|{{join .Imports ","}}' "./internal/service/$pkg"
    $name, $imports = $line -split '\|', 2
    if ($name -ne $pkg) {
        Fail "nama package $pkg tidak cocok dengan direktori (dapat: $name)"
    }
    foreach ($imp in ($imports -split ',')) {
        if ($imp.StartsWith($prefix)) {
            $dep = $imp.Substring($prefix.Length)
            if ($allow[$pkg] -notcontains $dep) {
                Fail "$pkg mengimpor service/$dep di luar allowlist"
            }
        }
    }
    # 3) testutil hanya boleh dipakai file test.
    $tline = go list -f '{{join .TestImports ","}}|{{join .XTestImports ","}}' "./internal/service/$pkg"
    $hasTestUtil = ($tline -split '\|') | Where-Object { $_ -match 'service/testutil$' }
    $regularHasTestUtil = ($imports -split ',') | Where-Object { $_ -match 'service/testutil$' }
    if ($regularHasTestUtil) {
        Fail "$pkg mengimpor testutil dari kode non-test"
    }
    if ($pkg -eq 'testutil' -and -not $hasTestUtil) {
        # testutil sendiri tidak perlu mengimpor dirinya; lewati
    }
}

# 4) Semua subpackage yang dikenal harus ada di allowlist (anti paket liar).
$dirs = Get-ChildItem -Path internal/service -Directory | ForEach-Object { $_.Name }
foreach ($d in $dirs) {
    if (-not $allow.ContainsKey($d)) {
        Fail "subpackage baru service/$d belum didaftarkan di allowlist skrip ini"
    }
}

if ($fail) { exit 1 }
Write-Host 'IMPORT-CHECK OK: aturan dependensi service terpenuhi.' -ForegroundColor Green
