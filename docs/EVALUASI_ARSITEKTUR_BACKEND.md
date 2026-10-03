# Evaluasi Arsitektur Backend (SIM-KIPAN Core)

> Status: **Catatan evaluasi** (tidak mengubah kode)
> Tanggal: 04 Oktober 2026
> Ruang lingkup: `backend/` (Go Fiber, arsitektur berlapis)
> Kode dokumen: `EVALUASI_ARSITEKTUR_BACKEND.md`
> Aturan acuan: `.agents/rules/RULES.md`

Dokumen ini menilai sejauh mana backend menerapkan **SOLID** dan **DRY**, disertai bukti
dan rekomendasi perbaikan bertahap. Penilaian bersifat teknis, bukan penilaian kualitas tim.

---

## 1. Ringkasan & Verdict

**Ya — sebagian besar SOLID & DRY sudah diterapkan.** Arsitektur berlapis rapi
(`Handler → Service → Repository`), dependensi disuntik via pola `Deps`, dan `domain`
bersih dari framework. Utang teknis utama: **interface repository terlalu gemuk (ISP)**,
beberapa **god-service**, serta **duplikasi resolusi scope wilayah** dan **allowlist status**.

| Area | Status |
|---|---|
| Pemisahan layer & arah dependensi | ✅ Baik |
| Domain bebas framework | ✅ Baik |
| Dependency Inversion (DI/`Deps`) | ✅ Baik (minor: `StorageHandler` konkret) |
| Interface Segregation | 🔴 Perlu perbaikan (interface repo gemuk) |
| Single Responsibility | 🟡 Baik, ada god-service |
| Open/Closed | 🟡 Sedang (banyak `switch` aksi/status) |
| DRY | 🟡 Sebagian (scope/allowlist/email terduplikasi) |

---

## 2. Peta Arsitektur

Struktur paket:

- `domain` — entitas, enum, error, fungsi domain murni (mis. `ActorContext.CanAccessWilayah`, `IsAllowedTransition`).
- `repository` — persistensi (SQL) + interface.
- `service` — business logic, otorisasi, orkestrasi transaksi/audit.
- `handler` — transport HTTP (thin handler).
- `middleware` — keamanan lintas-potong (auth, RBAC, scope, rate-limit, header).
- `gateway` — integrasi eksternal (SMTP, WhatsApp).
- `router` — **composition root** (wiring rute + DI) — dipisah dari `handler`.
- `pkg` — utilitas reusable (crypto, kta, nia, storage, validator, response).

Arah dependensi (sesuai `RULES §3`): `Handler → Service → Repository/Infrastructure`; `domain`
tidak bergantung pada Fiber/SQL/Redis/S3 — **terverifikasi** (tidak ada import terlarang di `internal/domain`).

---

## 3. Penilaian SOLID

### S — Single Responsibility
- ✅ Pemisahan layer konsisten; fitur besar sudah dipecah (auth, otp, password_reset, verification, revision, kta, storage, kepengurusan, wilayah, user, outbox, dashboard).
- 🟡 Beberapa **god-service**: `service/pendaftaran_service.go` **861 baris**, `service/kepengurusan_service.go` **811 baris** menggabungkan beberapa tanggung jawab (submit, tracking, queue, validasi wilayah, notifikasi, expiry; dan SK vs pengurus vs jabatan).

### O — Open/Closed
- 🟡 Menambah aksi/status baru menuntut mengedit beberapa `switch` (mis. `ApproveSK`, `mapStatusForAction`, `IsAllowedTransition`). Bukan pelanggaran fatal, tetapi kurang "terbuka untuk ekstensi".

### L — Liskov Substitution
- ✅ N/A untuk Go (tanpa pewarisan). Substitusi implementasi interface konsisten.

### I — Interface Segregation
- 🔴 **Titik terlemah.** Interface repository gemuk (jumlah method):
  - `WilayahAdminRepository` **17**, `PendaftaranRepository` **16**, `PengurusRepository` **11**, `UserRepository` **10**.
  - Fakes pada test harus meng-`embed` interface lalu menimpa 2–3 method — sinyal interface terlalu besar.
- ✅ Perbaikan yang sudah baik: interface kecil terpisah (`ListKeysetRepository` 2, `DashboardRepository` 1, `DocumentRepository` 1, `AuditLogRepository` 1, `JabatanRepository` 5).

### D — Dependency Inversion
- ✅ Service bergantung pada **interface** repository; handler pada **interface** service; wiring terpusat (kini di paket `router`).
- 🟡 `StorageHandler` menyimpan `*service.StorageService` **konkret** (bukan interface) → mempersulit substitusi/fake.

---

## 4. Penilaian DRY

**Sudah terpusat (positif):**
- Helper: `writeAudit`, `unavailable`, `mapDBError`, `paginationMeta`, `publicURLFrom`, `mustParseDuration`.
- Kontrak respons seragam (`response.Success/Paginated/FromError`).
- Pengiriman email via **outbox worker** (sebelumnya fire-and-forget tersebar).

**Duplikasi tersisa:**
- **Resolusi scope wilayah** — ✅ **DIPERBAIKI (A1)**: kini lewat satu sumber `domain.ActorContext.Scope()`.
- **Allowlist status** — ✅ **DIPERBAIKI (A2)**: kini `domain.PendaftaranStatus.IsValid()` / `domain.AnggotaStatus.IsValid()`.
- **Dua jalur email**: aktivitas kepengurusan memakai `sendAppointmentEmail` (goroutine) di luar outbox — belum konsisten dengan worker.
- Meta pagination masih manual di sebagian handler lama (sebagian sudah memakai `paginationMeta`).

---

## 5. Kekuatan

- Domain murni & mudah diuji; fungsi otorisasi/transisi sebagai fungsi murni.
- DI eksplisit via struct `Deps` (mudah dilacak, tanpa reflection).
- `router` sebagai composition root memisahkan wiring dari transport.
- Interface kecil untuk fitur baru (keyset, admin wilayah/user, dashboard, outbox) — arah yang benar.

---

## 6. Rekomendasi (A1–A6)

| Kode | Rekomendasi | Prinsip | Prioritas | Risiko | Status |
|---|---|---|---|---|---|
| **A1** | `ActorContext.Scope() (*int,*int,error)` di `domain`; ganti semua resolve scope manual | DRY + S | Tinggi | Rendah | ✅ Selesai |
| **A2** | Allowlist status di `domain` (`PendaftaranStatus.IsValid`, `AnggotaStatus.IsValid`) dipakai service/repo | DRY | Tinggi | Rendah | ✅ Selesai |
| **A3** | ISP: pecah interface repo gemuk menjadi per-use-case (Read/Write) + perbarui fakes | I | Sedang | Sedang | Belum |
| **A4** | Pecah god-service (`pendaftaran`, `kepengurusan`) menjadi beberapa service fokus | S | Sedang | Sedang–Tinggi | Belum |
| **A5** | `StorageHandler` bergantung pada interface `StorageService` | D | Sedang | Rendah | Belum |
| **A6** | Pindahkan `sendAppointmentEmail` ke outbox (tambah jenis `PENGANGKATAN`) | DRY | Rendah | Sedang | Belum |

**Catatan:** pemisahan `routes` ke paket `router` (lihat §7) sudah dieksekusi. **A1 & A2 telah dikerjakan**;
A3/A4/A5/A6 belum.

---

## 7. Struktur Routes (diekeskusi)

Wiring rute dipindah dari `internal/handler/routes.go` menjadi paket **`internal/router`**, dipecah per domain:

- `router.go` — `Register` + middleware global + health/internal.
- `deps.go` — composition root (`newDeps`) membangun repo/service/handler + worker.
- `wire.go` — `wireWAGateway`, `wireMailSender`, `wireStorageService`.
- `auth_routes.go`, `membership_routes.go`, `storage_routes.go`, `wilayah_routes.go`,
  `wilayah_admin_routes.go`, `anggota_routes.go`, `user_routes.go`, `user_admin_routes.go`,
  `notification_routes.go`, `kepengurusan_routes.go`, `dashboard_routes.go`, `outbox_routes.go`,
  `admin_routes.go` (me-scope + guard uji).

Manfaat: `handler` tetap **transport-only**, wiring terisolasi, dan mudah menavigasi rute per fitur.

---

## 8. Lampiran — Best Practice Penempatan Test Go

**Ya, file `_test.go` memang best practice diletakkan di folder yang sama dengan file non-test.**

- Konvensi toolchain Go: test berada di **direktori paket yang sama**; `go test ./...` menemukannya otomatis.
- Dua gaya (keduanya di folder yang sama):
  - **Internal/white-box** → `package service` → dapat menguji simbol tak diekspor. Banyak dipakai di repo ini.
  - **Eksternal/black-box** → `package service_test` → hanya API diekspor, tetap di folder sama.
- **Folder `tests/` terpisah tidak dianjurkan** untuk unit test. Untuk data/fixture gunakan subfolder **`testdata/`** (diabaikan toolchain).
- Menjaga test berdampingan membuat test ikut ter-`compile` dengan paket dan mudah ditemukan.

Kesimpulan: struktur test backend saat ini **sudah sesuai best practice**.

---

## 9. Bukti (Lampiran Teknis)

- Paket: `domain(12)`, `repository(16)`, `service(41)`, `handler(19)`, `middleware(11)`, `gateway(4)`, `database(2)` berkas `.go`.
- Berkas terbesar: `service/pendaftaran_service.go` 861, `service/kepengurusan_service.go` 811, `repository/kepengurusan_repository.go` 691, `repository/pendaftaran_repository.go` 618.
- `internal/domain` bersih dari import Fiber/SQL/Redis/S3.
