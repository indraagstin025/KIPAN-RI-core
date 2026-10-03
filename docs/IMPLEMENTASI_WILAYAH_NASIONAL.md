# Implementasi Wilayah Nasional (Seed + Proxy Kecamatan + Tampil Superadmin)

> Status: **Rancangan implementasi (belum dieksekusi)**
> Tanggal: 02 Oktober 2026 (revisi: gabung backend + frontend, per batch + testing + hardening)
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_WILAYAH_NASIONAL.md`

---

## 1. Latar Belakang & Tujuan

Dropdown pendaftaran saat ini hanya memuat **4 provinsi + 3 kabupaten** (`000002`), sehingga pendaftar di luar itu tidak bisa memilih wilayahnya. Proyek lama punya data lebih lengkap tetapi sumbernya bermasalah (duplikat `Bandung Barat`, `3204` salah peta). Sumber resmi `wilayah.id` (Kepmendagri 2025) dipakai sebagai kebenaran tunggal.

Tujuan:
- Dropdown pendaftaran memuat **seluruh Indonesia (38 provinsi + 514 kab/kota)** + bantuan dropdown **kecamatan**.
- API tetap **read-only**; submit kecamatan/kodepos tetap teks bebas.
- **Detail superadmin menampilkan Provinsi + Kabupaten/Kota** sesuai pilihan pendaftar; blok Alamat tidak diubah.
- Setiap batch mencakup **testing** dan **hardening keamanan** (skenario terbaik/terburuk).

---

## 2. Keputusan Final (dikunci — 02 Okt 2026)

1. **Sumber: `wilayah.id`** (resmi Kepmendagri No 300.2.2-2138/2025, update 04-07-2025). Bukan `master-wilayah.ts` lama, bukan emsifa.
2. **Nama: persis `wilayah.id`** (Title Case ber-prefix, cth. `Kota Bandung`) + normalisasi 7 baris existing via `DO UPDATE`.
3. **Kecamatan via `wilayah.id`** (proxy backend, bukan emsifa).
4. **Detail superadmin wajib menampilkan Provinsi + Kabupaten/Kota** sesuai pilihan pendaftar; blok Alamat (teks bebas) **tidak diubah**.
5. **Read-only** (tanpa CRUD wilayah). Proxy kodepos **ditunda** (backlog).
6. Mekanisme seed: **migrasi SQL idempoten** (di-generate skrip dari hasil fetch, bukan ketik manual).
7. Eksekusi dibagi **3 batch**, masing-masing dengan testing + hardening keamanan.

---

## 3. Sumber Data `wilayah.id`

| Level | Endpoint | Contoh |
|---|---|---|
| Provinsi (38) | `GET https://wilayah.id/api/provinces.json` | `{code:"32", name:"Jawa Barat"}` |
| Kab/Kota (514) | `GET https://wilayah.id/api/regencies/{PROV}.json` | `{code:"32.04", name:"Kabupaten Bandung"}` |
| Kecamatan | `GET https://wilayah.id/api/districts/{KAB}.json` | `{code:"32.73.01", name:"Sukasari"}` |

- Respons: `{ data: [{ code, name }], meta }`.
- **Kode bertitik** (`32.04`) → hilangkan titik untuk kolom `kode` DB (`3204`), selaras konvensi existing.
- `name` dipakai **apa adanya** (termasuk prefix `Kabupaten/Kota/Kota Administrasi`).

---

## 4. Batch 1 — Seed Nasional 38 + 514 (backend)

### 4.1 Tujuan
Tabel `wilayah_provinsi`/`wilayah_kabupaten` terisi penuh dan kanonis, tanpa merusak FK existing, agar dropdown pendaftaran lengkap.

### 4.2 Lingkup file
- Baru: `migrations/000014_seed_wilayah_nasional.up/.down.sql` (di-generate, bukan tulis tangan).
- Baru (sekali jalan, tidak ikut runtime): skrip generator `scripts/gen_wilayah_seed.*` (fetch → validasi → emit SQL; assert `meta.updated_at` + warning drift).
- Baru: `migrations/000015_wilayah_name_guard.up/.down.sql` (CHECK tolak `<>"` pada `nama` kedua tabel).
- Tidak menyentuh: service, handler, route, frontend.

### 4.3 Langkah
1. Skrip fetch `provinces.json` + 38× `regencies/{prov}.json` (HTTPS, timeout 15 dtk, verifikasi `meta.updated_at`).
2. Validasi tiap baris: `code` cocok `^\d{2}(\.\d{2})?$` (prov) / `^\d{2}\.\d{2}$` (kab); `name` 1–100 char, **tanpa `<>"`**; tolak duplikat `code` dalam payload.
3. Emit SQL pola `000002` (`INSERT ... SELECT p.id ... WHERE p.kode = ... ON CONFLICT (kode) DO UPDATE SET nama, is_active`), strip titik pada kode.
4. Review diff (khusus 7 baris existing, cth. `KOTA BANDUNG`→`Kota Bandung`) → `migrate up` di dev.
5. Verifikasi §4.5.

### 4.4 Testing
- **Unit/skrip**: kode lolos regex; nama lolos panjang/charset; duplikat `code` terdeteksi; emit SQL lolos quoting (kutip tunggal di-escape).
- **Integrasi DB**: `COUNT` = 38 prov + 514 kab (± toleransi terdokumentasi bila upstream berubah; gagal keras bila selisih besar); tanpa FK yatim (`pendaftaran`/`anggota`/`users` LEFT JOIN → 0); `id` 7 baris existing **tidak berubah** (cek sebelum/sesudah).
- **API**: `GET /wilayah/provinsi` → 38; `GET /wilayah/kabupaten/2` memuat Bandung Raya; `GetNames` untuk ID lama mengembalikan nama baru.

### 4.5 Hardening / keamanan

| # | Skenario terburuk | Skenario terbaik (yang diterapkan) | Status |
|---|---|---|---|
| S1 | Upstream disusupi/MITM → nama/kode beracun masuk DB dan menetap | Fetch HTTPS + timeout; generator **assert `meta.updated_at` + warning drift** vs META lama; **review diff manual** (7 baris) sebelum `migrate`; payload di-vendor + hash SHA256 di META | ✅ Diterapkan + diverifikasi |
| S2 | Migrasi gagal di tengah → setengah ter-seed | Satu file migrasi = satu transaksi; idempoten `ON CONFLICT`; verifikasi count/FK/id pasca-migrate (v13→v14 bersih) | ✅ Diterapkan + diverifikasi |
| S3 | `down.sql` destruktif menghapus master → FK yatim/rusak | `down.sql` **non-destruktif** (`SELECT 1`, tanpa `DELETE`/`DROP`) | ✅ Diterapkan |
| S4 | `DO UPDATE` menimpa koreksi lokal admin tanpa jejak | Kebijakan master-kanonis; diff 7 baris direview + dilaporkan eksplisit (laporan Batch 1) | ✅ Review dilakukan |
| S5 | `kode`/`nama` aneh merusak `UNIQUE`/FK atau injeksi via file SQL | Validasi pra-emit (regex, panjang, tolak `<>"`, trim, escape kutip) + **`UNIQUE` + CHECK `000015`** sebagai pagar DB | ✅ Diterapkan (migrasi v15) |
| S6 | Nama beracun lolos → stored-XSS di dropdown/detail admin | Aturan S5 + **CHECK DB** + React escape default di semua render `nama` | ✅ Diterapkan |

### 4.6 Kriteria selesai
38 prov + 514 kab aktif, nol FK yatim, `id` existing stabil, API mengembalikan penuh, diff nama terdokumentasi.

---

## 5. Batch 2 — Proxy Kecamatan + Dropdown Frontend

### 5.1 Tujuan
Form pendaftaran mendapat bantuan dropdown kecamatan (fail-open), tanpa tabel baru dan tanpa mengubah kontrak submit (tetap string nama).

### 5.2 Lingkup file
- Backend: `internal/service/wilayah_service.go` (+`ListKecamatan`), `internal/handler/wilayah_handler.go` (+`ListKecamatan`), registrasi route di `routes.go` (grup `wil_pub`), config timeout bila perlu.
- Frontend: `pendaftaran/types.ts` (+`WilayahKecamatan`), `pendaftaran/api/wilayahService.ts` (+`listKecamatan()`), `pendaftaran/hooks/useWilayah.ts` (+state kecamatan), `pendaftaran/pages/DaftarPage.tsx` (`f-kec` teks → dropdown + fallback teks).

### 5.3 Spesifikasi endpoint (backend)

```
GET /api/v1/wilayah/kecamatan?kabupaten_kode=3273
→ 200 { success, data: [{ kode: "327301", nama: "Sukasari" }, ...] }
```

- Input `kabupaten_kode` **wajib `^\d{4}$`** (else 400); konversi ke bertitik (`3273`→`32.73`) **dari segmen tervalidasi saja**.
- Fetch `wilayah.id/api/districts/{dotted}.json` (timeout 8 dtk, batasi 1 MB, **tanpa follow-redirect** ke host lain).
- **Fail-open**: upstream error/timeout/bentuk tak valid → `200 { data: [] }`.
- **Cache 24 jam** in-memory per kode; **hanya sukses non-kosong** yang disimpan (fail-open tidak menempel).
- Rate-limit grup `wil_pub` (seperti endpoint wilayah existing).
- `kode` respons dinormalisasi tanpa titik (konsisten konvensi `kode`).

### 5.4 Spesifikasi frontend

- `listKecamatan(kabupatenKode: string)`; `useWilayah` resolve `kode` dari daftar `kabupaten` yang sudah dimuat (cocokkan by `id` terpilih) — tanpa request tambahan.
- `f-kec`: `SelectInput` berisi `nama`; **yang dikirim tetap `nama` string** (kontrak submit/validasi/draft tidak berubah).
- Daftar kosong/gagal → otomatis kembali ke input teks (pendaftar tidak terblokir).
- `desa`, `kode_pos`, validasi, draft, payload: **tidak berubah**.

### 5.5 Testing
- **Unit (backend)**: validasi `kabupaten_kode` (terima `3273`, tolak `../../`, `http://`, kosong); parse respons (array valid, non-array → `[]`); konversi titik; non-cache hasil kosong.
- **Unit (handler)**: `wilayah_handler_test.go` — 400 untuk kode invalid, 200 passthrough.
- **Unit (frontend)**: resolve kode dari id; dropdown terisi; fallback teks saat `[]`/error; nilai terkirim = `nama`.
- **Integrasi**: mock upstream (sukses, lambat, rusak) → perilaku cache + fail-open benar; live smoke 1 kode (`3273` → 30 baris).
- **Regression**: `go test ./...`; `npm run build` + `lint`; submit pendaftaran dengan kecamatan dropdown lolos validasi.

### 5.6 Hardening / keamanan

| # | Skenario terburuk | Skenario terbaik (yang diterapkan) | Status |
|---|---|---|---|
| P1 | **SSRF** via `kabupaten_kode` (path traversal / URL penuh / host internal) | Allowlist `^\d{4}$` **sebelum** bangun URL; host konstanta HTTPS; tanpa follow-redirect; timeout 8 dtk; **test handler 400** (`../x`, URL, pendek/panjang) | ✅ Diterapkan + diverifikasi |
| P2 | Upstream disusupi → nama kecamatan beracun/menyesatkan tampil di dropdown | Validasi bentuk + cap panjang + tolak `<>` (item tak-valid dilewati); tanpa log isi body; React escape | ✅ Diterapkan + diverifikasi |
| P3 | Upstream lambat/mati → request gantung, thread habis, efek domino | Timeout 8 dtk + cap 1 MB + cache + rate-limit `wil_pub` + fail-open `[]` | ✅ Diterapkan + diverifikasi |
| P4 | Cache menyimpan respons rusak / membengkak | Hanya **sukses non-kosong** yang di-cache (fail-open tidak menempel; memori ≤ wilayah nyata); tanpa cache negatif | ✅ Diterapkan + diverifikasi (`KosongTidakDiCache`) |
| P5 | Privasi: query membocorkan pilihan wilayah user ke pihak ketiga | Hanya **kode 4-digit** (tanpa NIK/nama/kontak) | ✅ By-design |
| P6 | Frontend mengirim `kode` (bukan `nama`) → lolos validasi tapi salah makna | `value` opsi = `nama` (saat dropdown dibangun); backend tetap teks bebas 1–100 | ⏳ Backend siap; dropdown frontend menyusul |

### 5.7 Kriteria selesai
Dropdown kecamatan terisi per kabupaten, fallback teks bekerja, tidak ada tabel baru, tidak ada perubahan kontrak submit.

### 5.8 Koreksi ejaan upstream (peta `koreksiNamaUpstream`)

Upstream `wilayah.id` sesekali salah eja (terbukti: `32.17.02.2003` = `Cihanjuangrahayu`, seharusnya `Cihanjuang Rahayu`; desa tetangga `Cihanjuang` memang berbeda dan tidak diubah). Perbaikan: tabel koreksi by kode upstream di `wilayah_service.go`, diterapkan ke kecamatan + desa **sebelum** validasi/cache/invalidate, sehingga dropdown/submit menerima ejaan benar.

| Kode upstream | Tertulis | Dikoreksi menjadi |
|---|---|---|
| `32.17.02.2003` | Cihanjuangrahayu | Cihanjuang Rahayu |

Prosedur tambah entri: tambahkan baris + unit test (`TestKoreksiNamaUpstream`) + laporkan ke upstream (`wilayah.id`/cahyadsn). Status: ✅ Diterapkan + diverifikasi live.

---

## 6. Batch 3 — Tampil Superadmin Sesuai Pilihan

### 6.1 Tujuan
Detail superadmin menampilkan **Provinsi + Kabupaten/Kota** sesuai yang dipilih pendaftar (saat ini tidak tampil sama sekali); blok Alamat tidak diubah.

### 6.2 Lingkup file
- Backend: `internal/service/pendaftaran_service.go` (`GetDetail` + DTO `PendaftaranAdminDetail{ Pendaftaran, ProvinsiNama, KabupatenNama }` via `WilayahRepo.GetNames` — sudah di-inject).
- Frontend: `verification/types.ts` (+`provinsi_nama?`, `kabupaten_nama?`), `verification/pages/DetailPage.tsx` (+2 `Row` di bawah Alamat).

### 6.3 Testing
- **Unit**: `GetDetail` tetap sukses bila `GetNames` gagal (fallback string kosong); DTO memuat nama benar untuk ID lama maupun baru.
- **Integrasi**: buat 1 pendaftaran (Jawa Barat/Kota Bandung) → detail menampilkan `Jawa Barat` + `Kota Bandung`; data lama (ID existing) menampilkan nama hasil normalisasi.
- **Regression**: `go test ./...`; `npm run build` + `lint`; smoke browser detail.

### 6.4 Hardening / keamanan

| # | Skenario terburuk | Skenario terbaik (yang diterapkan) | Status |
|---|---|---|---|
| D1 | Lookup nama menggagalkan seluruh detail (500) | **Fail-open** + test (`TetapSuksesBilaNamaGagal`) + log warn | ✅ Diterapkan + diverifikasi |
| D2 | Nama bocor ke pihak tak berhak | Resolve server-side dalam request ber-`CanAccessWilayah`; test lintas-wilayah tetap 403; tanpa endpoint baru | ✅ Diterapkan + diverifikasi |
| D3 | Stored-XSS via `nama` master | Validasi seed + **CHECK `000015`** + React escape | ✅ Diterapkan |
| D4 | N+1 / detail melambat | Satu `GetNames` terindeks per detail; tanpa loop | ✅ By-design |

### 6.5 Kriteria selesai
Detail menampilkan Provinsi + Kabupaten/Kota yang cocok dengan pilihan; blok Alamat verbatim tidak berubah.

---

## 7. Backlog (di luar cakupan batch)

- **Proxy kodepos** (`kodepos.vercel.app`, pola sama seperti Batch 2: validasi input, timeout, cache, fail-open `""`).
- **Admin CRUD wilayah** (`POST/PUT/DELETE` + audit) — bila kelak dibutuhkan, dengan RBAC + `activity_logs`.
- **Kelurahan/desa** (`villages/…`) — tersedia di `wilayah.id` bila suatu saat `desa` ingin jadi dropdown.

---

## 8. Yang TIDAK Berubah

- Generator NIA (`pkg/nia`) — Opsi A (cetak apa adanya).
- Validasi submit (`provinsi_id`/`kabupaten_id` by DB id; `kecamatan`/`kode_pos` teks bebas).
- Kontrak `PendaftaranSubmit` / `PendaftaranDetail` selain penambahan `provinsi_nama`/`kabupaten_nama` (Batch 3, opsional di tipe).
- Blok Alamat di detail (verbatim).

---

## 9. Urutan Eksekusi & Kriteria Global

1. **Batch 1** → migrasi + verifikasi data (pintu masuk Batch 2–3).
2. **Batch 2** → proxy + dropdown (butuh `kode` master yang sudah benar).
3. **Batch 3** → tampil detail (butuh nama master yang sudah dinormalisasi).
- Setiap batch: implementasi → testing (§-masing-masing) → hardening (§-masing-masing) → `go test ./...` + `go vet` + `gofmt` (backend) dan `npm run build` + `lint` (bila menyentuh frontend) → smoke browser.

### Status eksekusi (02 Okt 2026)

- **Batch 1 ✅**: vendor 38+514 (+META hash), migrasi `000014` (v13→v14 bersih), `000015` guard (v14→v15 bersih); verifikasi count 38/514, FK yatim 0, ID stabil, API penuh.
- **Batch 2 (backend) ✅**: endpoint `GET /wilayah/kecamatan` + 6 unit test + 2 handler test hijau; live smoke `3273` → 30 baris, invalid → 400; dropdown frontend menyusul.
- **Batch 3 ✅**: DTO + 3 unit test hijau; live verify detail `provinsi_nama`/`kabupaten_nama` sesuai; frontend 2 baris; build/lint hijau.
