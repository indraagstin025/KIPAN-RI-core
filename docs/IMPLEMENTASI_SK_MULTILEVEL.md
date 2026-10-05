# Implementasi Lanjutan: SK Multi-Level, Single Active SK, & Ganti Jabatan Pengurus

> Status: **SELESAI (D2–D4)** — 03 Oktober 2026
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_SK_MULTILEVEL.md`
> Acuan: `IMPLEMENTASI_PENGANGKATAN.md` (modul dasar), `PERBANDINGAN_ADMIN_LAMA_BARU.md` (perilaku project lama).

---

## 1. Latar Belakang

Modul pengangkatan dasar (lihat `IMPLEMENTASI_PENGANGKATAN.md`) sudah selesai. Hasil perbandingan dengan project lama (`KIPAN_INDONESIA`) menemukan beberapa perilaku yang perlu diselaraskan/ditambah:

- **G1** — Aturan *Single Active SK* (SK lama selevel+wilayah otomatis nonaktif & pengurusnya demisioner) belum ada.
- **G2** — Pembuatan SK masih terbatas Kabupaten; project lama mengizinkan Provinsi & Nasional/Super (dengan jalur approval berbeda).
- **G3/G4/G5** — Aturan baru dipertahankan (SK terkunci saat final; Admin Provinsi boleh melihat antrean; penolakan SK hanya Nasional/Super).
- **G6** — Fitur **Ganti Jabatan Pengurus** belum ada.

Referensi perilaku project lama ada di `PERBANDINGAN_ADMIN_LAMA_BARU.md` §5–6. Catatan penting: project lama **tidak konsisten** (ada dua endpoint "tambah pengurus" dengan aturan berbeda). Dokumen ini menetapkan **satu aturan final yang paling ketat & konsisten**.

---

## 2. Keputusan Final (dikunci)

| Kode | Keputusan |
|---|---|
| **G1** | Terapkan **Single Active SK rule**: saat sebuah SK menjadi `DISETUJUI` (final), SK lain dengan **level & wilayah sama** yang berstatus `Aktif` → `TidakAktif`, dan seluruh pengurus aktifnya → `Demisioner`. |
| **G2** | Pembuat SK: Kabupaten, Provinsi, Nasional, Super — **level & approval ditentukan server dari role** (bukan input klien). |
| **G3** | SK terkunci saat `DISETUJUI` (tidak bisa tambah/hapus/ganti jabatan pengurus). |
| **G4** | Admin Provinsi **boleh melihat** antrean provinsinya, tetapi **tidak boleh** aksi verifikasi (dipertahankan). |
| **G5** | Aksi `TERUSKAN` = Provinsi; `SAHKAN`/`TOLAK` = Nasional/Super (dipertahankan). |
| **G6** | Tambah fitur **Ganti Jabatan Pengurus**. |
| **B1** | Pengelola pengurus: `KABUPATEN` → Admin Kabupaten (sekab); `PROVINSI` → Admin Provinsi (seprov); **`KABUPATEN` juga boleh dikelola Admin Provinsi yang sama provinsinya (Opsi A)**; `NASIONAL` → Nasional/Super; **Super Admin** sebagai oversight semua. Berlaku untuk tambah, hapus, dan ganti jabatan. |
| **B2** | Status pengurus yang dipromosikan/dinonaktifkan otomatis = **`Demisioner`** (seragam). |
| **B3** | Semua aksi pengurus **dikunci saat SK `DISETUJUI`**. |
| — | Cek jabatan inti memakai kolom **`is_inti`** (bukan daftar nama hardcoded). |

Semua keputusan **security by design**: identitas aktor diambil dari JWT terverifikasi, bukan dari body/query.

---

## 3. Desain Baru

### 3.1 Pembuatan SK per Role (G2)

`POST /api/v1/admin/sk` — level & approval diturunkan dari `actor.Role`:

| Actor | Level SK | `provinsi_id` | `kabupaten_id` | Approval awal |
|---|---|---|---|---|
| `ADMIN_KABUPATEN` | `KABUPATEN` | actor.provinsi | actor.kabupaten | `MENUNGGU_PROVINSI` |
| `ADMIN_PROVINSI` | `PROVINSI` | actor.provinsi | `NULL` | `MENUNGGU_NASIONAL` |
| `ADMIN_NASIONAL` / `SUPER_ADMIN` | `NASIONAL` | `NULL` | `NULL` | `DISETUJUI` (langsung final) |

- Validasi: `ADMIN_KABUPATEN` wajib punya `KabupatenID`; `ADMIN_PROVINSI` wajib punya `ProvinsiID`; Nasional/Super tanpa wilayah.
- Nomor SK unik; file SK wajib (`file_sk_key`).
- Bila langsung `DISETUJUI` (Nasional/Super) → jalankan **Single Active SK rule** untuk level `NASIONAL`.

### 3.2 Rantai Persetujuan SK (G5, dipertahankan)

- `MENUNGGU_PROVINSI` → `MENUNGGU_NASIONAL` (aksi `TERUSKAN`, hanya Admin Provinsi wilayah SK).
- `MENUNGGU_NASIONAL` → `DISETUJUI` (aksi `SAHKAN`, hanya Nasional/Super) atau `DITOLAK` (aksi `TOLAK`, wajib catatan).
- Transisi dijaga dengan **compare-and-swap** pada `approval_status` (anti balapan).

### 3.3 Single Active SK rule (G1)

Dipicu saat SK menjadi `DISETUJUI`:
1. Pada aksi `SAHKAN` (chain selesai), dan
2. Pada `CreateSK` oleh Nasional/Super (langsung final).

Efek (atomik, satu transaksi):
- Cari `surat_keputusan` lain dengan `status='Aktif'`, **level sama**, **wilayah sama** (`id <> sk.id`).
- Set `status='TidakAktif'`.
- Set seluruh `pengurus` berstatus `Aktif` pada SK lama → `status='Demisioner'`, `keterangan_status='Otomatis demisioner karena SK baru <nomor_sk> disetujui'`, `tanggal_selesai = sk.tanggal_terbit`.
- Audit setiap perubahan.

### 3.4 Pengelolaan Pengurus per Level (B1) + Lock Final (B3)

Untuk aksi **tambah / hapus / ganti jabatan**:

| Level SK | Role yang boleh | Catatan yurisdiksi |
|---|---|---|
| `KABUPATEN` | `ADMIN_KABUPATEN` (sekab) **atau** `ADMIN_PROVINSI` (provinsi SK = milik aktor) | `kabupaten_id`/`provinsi_id` SK cocok aktor |
| `PROVINSI` | `ADMIN_PROVINSI` | `provinsi_id` SK = milik aktor |
| `NASIONAL` | `ADMIN_NASIONAL`, `SUPER_ADMIN` | tanpa batas wilayah |

- **Opsi A**: Admin Provinsi boleh mengelola pengurus SK **Provinsi dan Kabupaten di dalam provinsinya**.
- `SUPER_ADMIN` selalu boleh (oversight).
- Syarat umum: SK `status='Aktif'` dan `approval_status <> 'DISETUJUI'` (B3). Setelah final → `403` ("buat SK baru").
- Anggota wajib `AKTIF` dan satu wilayah dengan SK.

### 3.5 Jabatan Inti (is_inti)

- Jika jabatan `is_inti = TRUE`, di dalam satu SK hanya boleh **satu** pemegang.
- Cek: hitung pemegang jabatan tsb di SK (kecuali anggota yang sedang diproses) → jika ada, `409`.

### 3.6 Ganti Jabatan Pengurus (G6)

`PATCH /api/v1/admin/pengurus/:id/jabatan` — body `{ "jabatan_id": <int> }`.

Alur service (semua validasi server-side):
1. Ambil `pengurus` (join SK) → 404 bila tak ada.
2. Otorisasi: role sesuai level SK (B1) + yurisdiksi (`CanAccessWilayah`).
3. SK `Aktif` dan **belum** `DISETUJUI` (B3) → jika final, `403`.
4. Jabatan baru ada & `is_active`; bila sama dengan sekarang → no-op sukses.
5. Bila jabatan baru `is_inti` → cek tidak ada pemegang lain di SK (exclude pengurus ini) → `409`.
6. `UPDATE pengurus SET jabatan_id=..., updated_at=... WHERE id=...` (bukan insert baru, karena `UNIQUE(surat_keputusan_id, anggota_id)`).
7. Audit `pengurus_ganti_jabatan` (from/to jabatan).

> Berbeda dari project lama yang **membuat baris baru** dan memakai status `Selesai`; project baru memakai `UPDATE` in-place + `Demisioner` (B2) agar konsisten dengan constraint unik.

### 3.7 Status Pengurus (B2)

- Dipromosikan/dinonaktifkan otomatis → `Demisioner` (dengan `keterangan_status` + `tanggal_selesai`).
- Ubah status manual (endpoint yang sudah ada `PATCH /admin/pengurus/:id`) tetap memakai allowlist enum `PengurusStatus`.

---

## 4. Keamanan (Security by Design)

- **Aktor dari JWT** (`actorOf`) — role/wilayah/level tidak pernah dari body/query.
- **Level & approval diturunkan server** dari role (G2) — klien tidak menentukan level SK.
- **Otorisasi berlapis**: `RequireRoles` (route) + yurisdiksi (`CanAccessWilayah` / sekab-seprov) + aturan domain (rantai SK, lock final, inti tunggal, anti-duplikat).
- **Concurrency**: CAS pada `approval_status`; finalisasi + deaktivasi SK lama dalam **satu transaksi**.
- **Validasi input**: allowlist status/jabatan, panjang, non-kosong; SQL parameterized (`$n`).
- **Audit** setiap mutasi (`activity_logs`, append-only, PII dimasking).
- **Rate limit** grup mutasi `kep_mut` mencakup endpoint baru.
- **Fail-closed**: dependensi tak dikonfigurasi → `503` (bukan bypass).

---

## 5. Perubahan Backend

| File | Perubahan |
|---|---|
| `internal/domain/kepengurusan.go` | `UpdateJabatanRequest { JabatanID int }` |
| `internal/repository/kepengurusan_repository.go` | `DeactivatePriorActiveSKs(ctx, sk)` (transaksional); `UpdateJabatan(ctx, pengurusID, jabatanID)` |
| `internal/service/kepengurusan/kepengurusan_service.go` | `CreateSK` multi-level (G2) + trigger Single Active (G1) saat final langsung; `ApproveSK` `SAHKAN` → finalisasi + Single Active (transaksional); `UpdatePengurusJabatan` (G6); sesuaikan otorisasi add/remove ke level SK (B1) + lock final (B3) |
| `internal/handler/kepengurusan_handler.go` | Handler `UpdatePengurusJabatan` |
| `internal/handler/routes.go` | `PATCH /admin/pengurus/:id/jabatan` |
| `internal/service/kepengurusan/kepengurusan_service_test.go` | Tes baru (lihat §7) |

Route yang ada tidak berubah selain penambahan di atas (`POST /admin/sk` tetap; gate role sudah 4 admin).

---

## 6. Perubahan Frontend

| File | Perubahan |
|---|---|
| `features/kepengurusan/api/kepengurusanService.ts` | `adminUpdatePengurusJabatan(id, jabatanId)` |
| `features/kepengurusan/roles.ts` | `canManagePengurusForLevel(role, level)`; sesuaikan `canCreateSK` (4 admin) |
| `features/kepengurusan/pages/PengurusListPage.tsx` | Aksi **"Ganti Jabatan"** (pilih jabatan aktif) |
| `features/kepengurusan/pages/SkListPage.tsx` | Tombol **"+ Buat SK"** tampil untuk Provinsi/Nasional/Super juga (form sama; level ditentukan server) |
| `features/kepengurusan/pages/SkDetailPage.tsx` | Tampilkan level & tombol aksi sesuai level; tandai SK terkunci saat final |

---

## 7. Skema DB

**Tidak ada migrasi baru.** Semua memakai kolom/enum yang sudah ada:
`surat_keputusan.status` (`Aktif`/`TidakAktif`/`Digantikan`), `approval_status`, `pengurus.status` (`Demisioner`), `pengurus.keterangan_status`, `pengurus.tanggal_selesai`, `pengurus.jabatan_id`, `jabatan.is_inti`.

---

## 8. Testing & Verifikasi

**Unit (`kepengurusan_service_test.go`)**
1. `CreateSK`: Kab→`KABUPATEN`/`MENUNGGU_PROVINSI`; Prov→`PROVINSI`/`MENUNGGU_NASIONAL`; Nas/Super→`NASIONAL`/`DISETUJUI`.
2. `ApproveSK`: rantai KAB→PROV→NAS; tolak tanpa catatan ditolak; `SAHKAN` memicu deaktivasi SK lama + demisioner.
3. Single Active: SK lama nonaktif, pengurus lama `Demisioner`.
4. `UpdatePengurusJabatan`: lock final `403`; inti ganda `409`; jabatan nonaktif ditolak; yurisdiksi per level.
5. `CreateSK` final (Nas/Super) → Single Active `NASIONAL`.

**Regresi**: `gofmt` + `go build ./...` + `go vet ./...` + `go test ./...`; `npm run build` + `lint`.

**E2E smoke**: Kab buat SK → Prov teruskan → Nas sahkan (SK lama nonaktif & pengurus demisioner) · Prov buat SK → Nas sahkan · Nas buat SK (final, Single Active Nasional) · Ganti jabatan (audit + inti unik).

---

## 9. Tahapan Eksekusi

1. **D1 — Dokumentasi** (dokumen ini) ✅ selesai.
2. **D2 — Backend**: domain → repository → service → handler → route + unit test.
3. **D3 — Frontend**: service API → roles → halaman (Pengurus ganti jabatan, SK multi-level, lock final).
4. **D4 — Verifikasi**: regresi + E2E + update `AUTH_GUIDE.md` §8 & `IMPLEMENTASI_PENGANGKATAN.md` (addendum).

---

## 10. Kriteria Selesai

- Kab/Prov/Nas-Super dapat membuat SK pada levelnya masing-masing (level dari server).
- SK final `DISETUJUI` otomatis menonaktifkan SK selevel+wilayah lama & mendemisionerkan pengurusnya (transaksional, teraudit).
- Pengurus dapat ditambah/dihapus/diganti jabatan sesuai level SK, **terkunci** saat SK final.
- Ganti Jabatan tervalidasi (yurisdiksi, lock final, jabatan aktif, inti tunggal) + teraudit.
- Seluruh validasi server-side; tidak ada role/level/wilayah dari klien.
