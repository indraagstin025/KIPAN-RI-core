# USER REQUIREMENTS DOCUMENT (URD) — SIM-KIPAN

> **Versi**: 1.0-draft · **Tanggal**: 28 September 2026 · **Bahasa**: Indonesia
> **Status**: DRAF — menunggu pengesahan pemilik (§0.3).
> Dokumen ini mendefinisikan **kebutuhan pengguna** sebagai persiapan pembangunan
> website. Dokumen ini tidak membahas status pengerjaan, branch, maupun kode.

---

## 0. Informasi dokumen

### 0.1 Riwayat

| Versi | Tanggal | Perubahan | Penulis |
|---|---|---|---|
| 1.0-draft | 28 Sep 2026 | Rilis draf awal | Tim Engineering |

### 0.2 Referensi

- `KIPAN_INDONESIA/PRD.md`, `DESIGN.md` (sistem sebelumnya)
- `docs/DOKUMEN_ALUR_BISNIS_DAN_CORE_SISTEM_KIPAN.md`
- `docs/ROADMAP_DAN_TAHAPAN_PENGERJAAN_SIM_KIPAN.md`

### 0.3 Lembar pengesahan (diisi manual)

| Peran | Nama | Tanda tangan | Tanggal |
|---|---|---|---|
| Pemilik produk | _(diisi manual)_ | | |
| Pimpinan DPP | _(diisi manual)_ | | |
| Lead engineering | _(diisi manual)_ | | |

---

## 1. Latar belakang & visi

KIPAN Indonesia membutuhkan sistem informasi keanggotaan skala nasional
(38 provinsi, 514 kabupaten/kota, ±150rb anggota tercatat). Sistem dibangun
dengan arsitektur terpisah:

| Aplikasi | Peran |
|---|---|
| **Laravel CMS** (Inertia + PostgreSQL) | Halaman utama + pengelolaan konten; ber-link ke pendaftaran |
| **React SPA** | Halaman pendaftaran + halaman utama kecil |
| **Go Core API** | Mesin transaksi: autentikasi, pendaftaran, SK, audit |

---

## 2. Pemangku kepentingan & peran

| Peran | Aplikasi | Wewenang inti | Batasan penting |
|---|---|---|---|
| Pengunjung | Laravel | Baca profil/berita/galeri | — |
| Pendaftar reguler | SPA | Daftar, lacak status, revisi berkas bertoken | Usia 16–30 tahun (UR-030) |
| Anggota lama (kanal khusus) | SPA | Mendaftar melalui kanal berjendela waktu | Syarat kanal khusus §3.2 |
| Admin DPC (kab/kota) | Core | Verifikasi + setujui pendaftaran wilayahnya; membuka NIK teraudit | Dilarang lintas wilayah |
| Admin DPD (provinsi) | Core | Review SK tahap-1 | Dilarang verifikasi pendaftaran |
| Admin DPP (nasional) | Core | Override nasional; pengesahan final SK; konfirmasi demisioner eksplisit | — |
| Super Admin | Core | Penuh termasuk audit trail | — |
| Admin konten | Laravel CMS | Kelola konten publik | Terpisah dari Core |

---

## 3. Kebutuhan per aplikasi

### 3.1 Laravel — halaman utama + CMS

| ID | Kebutuhan |
|---|---|
| UR-001 | Halaman utama profil organisasi (visi, struktur, kontak) |
| UR-002 | CMS berita/galeri/program berjenjang (nasional/provinsi/kabupaten) |
| UR-003 | Link/tombol menuju halaman pendaftaran SPA |
| UR-004 | Batasan wilayah konten (admin daerah hanya konten daerahnya) |

### 3.2 React SPA — pendaftaran

| ID | Kebutuhan |
|---|---|
| UR-010 | Formulir pendaftaran multi-langkah + unggah berkas (maks foto/KTP 2MB, PDF 5MB) |
| UR-011 | Lacak status mandiri: nama tersamar + status + lini masa (tanpa isi catatan); isi catatan perbaikan hanya tampil setelah bukti pemilik terverifikasi |
| UR-012 | Revisi berkas bertoken: bukti email DAN whatsapp terdaftar, token sekali pakai 24 jam |
| UR-013 | Verifikasi KTA publik via QR (putusan valid/tidak valid) |
| UR-014 | Dropdown wilayah (provinsi → kabupaten/kota) |
| UR-015 | Kanal khusus anggota lama: dibuka-tutup per periode; di luar periode sistem menolak dengan pesan jelas |

### 3.3 Go Core — mesin transaksi

| ID | Kebutuhan |
|---|---|
| UR-020 | Masuk JWT + refresh rotation + keluar idempoten; kebijakan sandi kuat |
| UR-021 | RBAC 4 level + yurisdiksi wilayah di sisi server |
| UR-022 | Pendaftaran: validasi kuat, NIK terenkripsi + indeks buta, NIK ganda ditolak (409), nomor `REG-YYYYMM-XXXX` |
| UR-023 | Alur status DIAJUKAN → DIVERIFIKASI → PERBAIKAN/DITOLAK/DISETUJUI; data + riwayat dipertahankan |
| UR-024 | NIA format `KIPAN-{2 huruf}-{kab4}-{tahun}-{urut global 5 digit}` (contoh `KIPAN-JB-3204-2026-00001`) |
| UR-025 | KTA: QR HMAC terverifikasi + PDF sisi server + unduhan teraudit |
| UR-026 | Pembukaan NIK teraudit untuk verifikator sewilayah |
| UR-027 | Unggah presign + validasi MIME/ukuran/magic bytes; dokumen privat via URL kedaluwarsa + audit |
| UR-028 | Audit trail append-only semua mutasi sensitif (aktor riil, tanpa nama statis) |
| UR-029 | SK & pengurus berjenjang: draf → review DPD → sah DPP; demisioner atas konfirmasi eksplisit DPP; mutasi tercatat |
| UR-030 | Usia pendaftar 16–30 tahun |

---

## 4. Alur end-to-end

- **Pendaftaran & verifikasi**: daftar → antrean DPC → verifikasi → setujui/tolak/perbaikan → terbit NIA → anggota; riwayat utuh per pendaftaran.
- **Revisi**: status PERBAIKAN → minta token (bukti pemilik) → unggah ulang → kembali DIAJUKAN.
- **Birokrasi SK**: draf DPC/DPD → review DPD → pengesahan DPP (SK nasional lewati review provinsi) → tolak bersifat terminal; demisioner SK lama hanya atas konfirmasi DPP bila ada SK pengganti yang sah.
- **Keanggotaan SK**: 1 orang maks 1× per SK; jabatan inti tunggal per SK; pengangkatan baru mendemisionerkan jabatan aktif lama otomatis; SK ditolak tidak mengubah baris pengurus.
- **Publikasi CMS**: draf → terbit berjenjang sesuai wilayah penulis.
- **Kanal khusus**: sama seperti reguler + gerbang periode + antrean terpisah.

---

## 5. Siklus hidup & kasus tepi

| Skenario | Aturan |
|---|---|
| Promosi jabatan | Otomatis demisionerkan jabatan aktif lama + alasan promosi |
| Demisioner/regenerasi | Hanya atas konfirmasi DPP bila ada SK pengganti sah sewilayah+selevel |
| Pengunduran/sanksi | Status nonaktif + riwayat, tanpa hapus data |
| Mutasi domisili | Validasi wilayah baru terhadap master |
| Pergantian admin wilayah | Akun terikat wilayah; serah terima tercatat audit |
| SK ditolak | Baris pengurus dibiarkan; seluruh query aktif memfilter SK yang sah |
| NIK ganda lintas kanal | Entri kedua ditolak; dimediasi manual |

---

## 6. Kebutuhan non-fungsional sisi pengguna

| ID | Kebutuhan |
|---|---|
| UN-001 | Bahasa Indonesia; aksesibilitas dasar (kontras, keyboard) |
| UN-002 | SLA verifikasi berkas — **TBD** (pemilik menentukan) |
| UN-003 | Batas operasional: foto/KTP ≤2MB, PDF ≤5MB; token revisi 24 jam sekali pakai; URL privat 5 menit |
| UN-004 | Respons publik tidak membocorkan PII (NIK tersamar, dokumen via token) |
| UN-005 | Kapasitas: gelombang provinsi (puluhan–ratusan konkuren) + 150rb anggota |

---

## 7. Kriteria terima (sampel; lengkap menyusul di SRS)

- **UR-012**: diberikan pendaftar PERBAIKAN + bukti benar → ketika minta token → maka token + kedaluwarsa 24 jam; dan bukti salah → 403 generik; dan pakai ulang → 403.
- **UR-023**: diberikan status DIAJUKAN → ketika disetujui langsung → maka ditolak 422; dan data + riwayat tidak terhapus saat DISETUJUI.
- **UR-025**: diberikan QR asli → putusan valid + data publik; QR ubah 1 karakter → tidak valid.
- **UR-030**: diberikan umur 15 atau 31 → maka ditolak 422.
- **UR-029**: diberikan SK tanpa pengganti sah → ketika demisioner → maka ditolak; dan eksekusi ganda → tepat 1 berlaku.

---

## 8. Hal terbuka (TBD)

| # | Item |
|---|---|
| TBD-01 | Kontrak integrasi Laravel↔SPA (tautan vs SSO) |
| TBD-02 | Strategi domain/subdomain |
| TBD-03 | Verifikasi UR-001/002/004 ke repo Laravel (menunggu akses) |
| TBD-04 | SLA verifikasi (UN-002) |
| TBD-05 | Data 150rb: struktur berkas contoh, kolom NIA lama, peran pengunggah |
| TBD-06 | Pengesah §0.3 (diisi manual) |
