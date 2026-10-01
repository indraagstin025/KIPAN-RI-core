<#
.SYNOPSIS
    Commit perubahan bekerja Fase 3 + hardening keamanan, dikelompokkan per area
    dengan pesan commit yang menjelaskan TIAP BERKAS.

.DESCRIPTION
    Men-stage & meng-commit perubahan working tree dalam 3 commit terkelompok
    (fitur membership, hardening keamanan, script+docs). TIDAK melakukan push.
    Aman dijalankan berkali-kali: grup tanpa perubahan baru akan dilewati.

.PARAMETER DryRun
    Hanya menampilkan berkas yang akan di-add, tanpa commit.

.EXAMPLE
    pwsh -NoProfile -ExecutionPolicy Bypass -File ./commit_fase3_security.ps1
    pwsh -NoProfile -ExecutionPolicy Bypass -File ./commit_fase3_security.ps1 -DryRun
#>
[CmdletBinding()]
param(
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

function Commit-Group {
    param(
        [Parameter(Mandatory = $true)][string]$Message,
        [Parameter(Mandatory = $true)][string[]]$Files
    )
    $title = ($Message -split "`n")[0]
    Write-Host ""
    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host "COMMIT: $title" -ForegroundColor Cyan
    Write-Host "==========================================================" -ForegroundColor Cyan

    foreach ($f in $Files) {
        $abs = Join-Path $repoRoot $f
        if (-not (Test-Path -LiteralPath $abs)) {
            Write-Host "  (lewati, tidak ada) $f" -ForegroundColor DarkGray
            continue
        }
        if ($DryRun) { Write-Host "  add $f" -ForegroundColor DarkGray; continue }
        & git -C $repoRoot add -- $f | Out-Null
    }
    if ($DryRun) { return }

    $staged = & git -C $repoRoot diff --cached --name-only
    if (-not $staged) {
        Write-Host "  (tidak ada perubahan baru untuk di-commit; dilewati)" -ForegroundColor Yellow
        return
    }
    Write-Host "  staged: $($staged.Count) berkas" -ForegroundColor DarkGray
    & git -C $repoRoot commit -m $Message | Out-String | Write-Host
}

# ============================================================
# GROUP 1 - Fitur Fase 3: modul anggota & notifikasi
# ============================================================
$msg1 = @'
feat(membership): Fase 3 — modul anggota & notifikasi + refactor layanan

Ruang lingkup per berkas:
- internal/domain/anggota.go: DTO anggota (list/detail/publik); DTO publik
  dibatasi whitelist {nia, nama_lengkap, status, provinsi_nama, kabupaten_nama}.
- internal/domain/anggota_dto_test.go: regression test whitelist DTO publik
  (menolak tanggal_angkat, NIK, kontak, alamat, object key).
- internal/domain/pendaftaran.go: DTO pendaftaran; tracking publik dibuat minimal
  (SEC-TRACK-PII) + DTO detail/revisi pendukung.
- internal/domain/tracking_dto_test.go: regression test respons tracking tanpa PII.
- internal/domain/pendaftaran_test.go, internal/domain/schema.go: penyesuaian tipe/DTO.
- internal/handler/anggota_handler.go: handler list/detail admin + cek publik (by NIA).
- internal/service/anggota_service.go(+_test): list/detail terfilter wilayah + cek publik.
- internal/repository/anggota_repository.go: query anggota (proyeksi non-PII) terfilter wilayah.
- internal/handler/notification_handler.go: handler notifikasi milik sendiri (tanpa ?userId=).
- internal/service/notification_service.go(+_test) & repository/notification_repository.go:
  notifikasi in-app anti-IDOR (identitas dari JWT).
- internal/service/verification_service.go & revision_service.go: pemecahan god-service
  (verifikasi/revisi) + sebar notifikasi admin.
- internal/repository/pendaftaran_repository.go & wilayah_repository.go: query pendukung.
- internal/service/pendaftaran_service.go(+_test): refactor + GetTracking minimal.
- internal/service/reveal_nik_test.go: test reveal NIK teraudit.
- migrations/000008_persyaratan_checklist.(up|down).sql: kolom checklist persyaratan.
- migrations/000009_notifications.(up|down).sql: tabel notifikasi.
'@
Commit-Group -Message $msg1 -Files @(
    "backend/internal/domain/anggota.go",
    "backend/internal/domain/anggota_dto_test.go",
    "backend/internal/domain/pendaftaran.go",
    "backend/internal/domain/tracking_dto_test.go",
    "backend/internal/domain/pendaftaran_test.go",
    "backend/internal/domain/schema.go",
    "backend/internal/handler/anggota_handler.go",
    "backend/internal/service/anggota_service.go",
    "backend/internal/service/anggota_service_test.go",
    "backend/internal/repository/anggota_repository.go",
    "backend/internal/handler/notification_handler.go",
    "backend/internal/service/notification_service.go",
    "backend/internal/service/notification_service_test.go",
    "backend/internal/repository/notification_repository.go",
    "backend/internal/service/verification_service.go",
    "backend/internal/service/revision_service.go",
    "backend/internal/repository/pendaftaran_repository.go",
    "backend/internal/repository/wilayah_repository.go",
    "backend/internal/service/pendaftaran_service.go",
    "backend/internal/service/pendaftaran_service_test.go",
    "backend/internal/service/reveal_nik_test.go",
    "backend/migrations/000008_persyaratan_checklist.up.sql",
    "backend/migrations/000008_persyaratan_checklist.down.sql",
    "backend/migrations/000009_notifications.up.sql",
    "backend/migrations/000009_notifications.down.sql"
)

# ============================================================
# GROUP 2 - Hardening keamanan (hasil pentest)
# ============================================================
$msg2 = @'
fix(security): hardening authz & minimalisasi PII (hasil pentest Fase 3)

Temuan yang ditutup, per berkas:
- SEC-AGT-PANIC (CRITICAL): internal/middleware/ratelimit.go(+_test) —
  daftarkan kebijakan "agt_pub" (30/mnt) agar wiring rute tidak panic saat boot.
- internal/middleware/ratelimit_registry_test.go: regression guard — setiap nama
  kebijakan yang dipakai routes.go WAJIB terdaftar di ratePolicies.
- SEC-STORE-BOLA (HIGH): internal/service/storage_service.go(+_test),
  internal/repository/document_repository.go — presign-view me-resolve pemilik
  key lalu cek CanAccessWilayah; fail-closed bila resolver nil.
- internal/service/storage_view_authz_test.go: unit test otorisasi presign-view.
- SEC-TRACK-PII (HIGH): internal/middleware/security.go — TrackLimiter per-nomor
  (15/mnt) untuk pelacakan publik yang nomornya sekuensial.
- SEC-AUDIT-NAME (LOW): internal/service/auth_service.go +
  internal/handler/pendaftaran_handler.go(+_test) — klaim "name" di JWT;
  actorOf memakai nama (fallback email untuk token lama).
- internal/handler/routes.go: wiring document repository + limiter tracking.
'@
Commit-Group -Message $msg2 -Files @(
    "backend/internal/middleware/ratelimit.go",
    "backend/internal/middleware/ratelimit_test.go",
    "backend/internal/middleware/ratelimit_registry_test.go",
    "backend/internal/middleware/security.go",
    "backend/internal/service/auth_service.go",
    "backend/internal/handler/pendaftaran_handler.go",
    "backend/internal/handler/pendaftaran_handler_test.go",
    "backend/internal/handler/routes.go",
    "backend/internal/service/storage_service.go",
    "backend/internal/service/storage_service_test.go",
    "backend/internal/service/storage_view_authz_test.go",
    "backend/internal/repository/document_repository.go"
)

# ============================================================
# GROUP 3 - Script pentest & dokumentasi
# ============================================================
$msg3 = @'
test(pentest): suite v2.8 + repro SEC-AGT-PANIC + pentest SEC-STORE-BOLA + laporan

Per berkas:
- backend/scripts/pentest_suite.ps1: v2.8 — tambah BOOT-01, AGT-01..05,
  NOTIF-01..03, RATELIMIT-AGT, TRACK-01..05 (assert respons tanpa PII).
- backend/scripts/repro_agt_panic.ps1: repro E2E panic wiring rate limit
  (poll /health selagi proses hidup; exit 1 = terbukti rentan).
- backend/scripts/pentest_storage_bola.ps1: pentest BOLA presign-view lintas
  wilayah (BOLA-01..06; exit 1 = terbukti rentan).
- docs/KOMPATIBILITAS_FRONTEND_LAMA.md: peta endpoint lama→baru dan kontrak
  respons minimal (track, cek publik).
- docs/pentest-fase3/LAPORAN_PENTEST_FASE3.md: laporan temuan (ID, severity,
  bukti E2E, fix/retest, gate) sesuai template .agents/rules/pentest-security.md.
- commit_fase3_security.ps1: script commit ini.
'@
Commit-Group -Message $msg3 -Files @(
    "backend/scripts/pentest_suite.ps1",
    "backend/scripts/repro_agt_panic.ps1",
    "backend/scripts/pentest_storage_bola.ps1",
    "docs/KOMPATIBILITAS_FRONTEND_LAMA.md",
    "docs/pentest-fase3",
    "commit_fase3_security.ps1"
)

# ============================================================
# RINGKASAN
# ============================================================
if (-not $DryRun) {
    Write-Host ""
    Write-Host "==========================================================" -ForegroundColor Green
    Write-Host "SELESAI — commit terakhir:" -ForegroundColor Green
    Write-Host "==========================================================" -ForegroundColor Green
    & git -C $repoRoot --no-pager log --oneline -4 | Out-String | Write-Host
    Write-Host "Sisa perubahan (idealnya kosong):" -ForegroundColor DarkGray
    & git -C $repoRoot status --short | Out-String | Write-Host
}

