# USER REQUIREMENTS DOCUMENT (URD) — SIM-KIPAN

> **Versi**: 1.0-draft · **Tanggal**: 28 September 2026 · **Bahasa**: Indonesia
> **Status**: DRAF — menunggu pengesahan pemilik (§0.3).
> **Ruang lingkup kebutuhan**: seluruh sistem (3 aplikasi, §2). Status implementasi
> dicatat jujur per kebutuhan: ✅ sudah jalan / 🔶 diputuskan-belum-dibangun /
> ⏸ ditunda (butuh frontend/infra) / ⬜ belum dibangun (Fase 3+).
> Aturan main: bila dokumen ini beda dengan kode hijau, **kode yang menang**;
> selisih dicatat di `docs/Task-Fase3` (deviasi D1–D5) dan diperbaiki dokumennya.

---

## 0. Informasi dokumen

### 0.1 Riwayat

| Versi | Tanggal | Perubahan | Penulis |
|---|---|---|---|
| 1.0-draft | 28 Sep 2026 | Rilis draf: ekstraksi dari proyek lama + perilaku build Fase 1–2 | Tim Engineering |

### 0.2 Referensi

- `KIPAN_INDONESIA/PRD.md`, `DESIGN.md`, `hasil_bab4.md` (proyek lama)
- `docs/DOKUMEN_ALUR_BISNIS_DAN_CORE_SISTEM_KIPAN.md`, `docs/ROADMAP_*`
- `docs/Task-Fase3/TASK_FASE3_SK_PENGURUS.md` (keputusan D1–D5)
- Perilaku terverifikasi: backend `backend-fase-2-membership-pendaftaran`

### 0.3 Lembar pengesahan (diisi manual)

| Peran | Nama | Tanda tangan | Tanggal |
|---|---|---|---|
| Pemilik produk | _(diisi manual)_ | | |
| Pimpinan DPP | _(diisi manual)_ | | |
| Lead engineering | _(diisi manual)_ | | |

---

## 1. Latar belakang & visi

KIPAN Indonesia membutuhkan sistem informasi keanggotaan skala nasional
(38 provinsi, 514 kab/kota, ±150rb anggota tercatat manual). Proyek lama
(Next.js monolit) memiliki 42 temuan keamanan/arsitektur sehingga ditulis
ulang dengan arsitektur terpisah:

| Aplikasi | Peran | Status |
|---|---|---|
| **Laravel CMS** (Inertia + PostgreSQL, sudah jadi, repo terpisah) | Landing utama + CMS konten; ber-link ke pendaftaran | Berjalan terpisah |
| **React SPA** (pendaftaran + mini landing) | Formulir publik | ⬜ Belum dibangun |
| **Go Core API** (auth, pendaftaran, SK, audit) | Mesin transaksi | ✅ Fase 1–2 jalan |

---

## 2. Pemangku kepentingan & peran

| Peran | Aplikasi | Wewenang inti | Batasan penting |
|---|---|---|---|
| Pengunjung | Laravel | Baca profil/berita/galeri | — |
| Pendaftar reguler | SPA → Core | Daftar, lacak, revisi bertoken | Usia 16–30 (UR-030) |
| Anggota lama (kanal khusus, parkir) | SPA → Core | Daftar via kanal berjendela | ⏸ Menunggu data contoh |
| Admin DPC (kab/kota) | Core | Verifikasi + setujui pendaftaran wilayahnya; lihat NIK teraudit | Dilarang lintas wilayah |
| Admin DPD (provinsi) | Core | Review SK tahap-1; **dilarang verifikasi pendaftaran** | Warisan PRD lama, dipertahankan |
| Admin DPP (nasional) | Core | Override nasional; pengesahan final SK; **konfirmasi demisioner eksplisit (D1)** | — |
| Super Admin | Core | Penuh + audit trail | — |
| Admin konten | Laravel CMS | Kelola konten publik | Terpisah dari Core |

---

## 3. Kebutuhan per aplikasi

### 3.1 Laravel — landing + CMS

| ID | Kebutuhan | Status |
|---|---|---|
| UR-001 | Halaman utama profil organisasi (visi, struktur, kontak) | ⬜ Verifikasi ke repo Laravel |
| UR-002 | CMS berita/galeri/program berjenjang (nasional/prov/kab) | ⬜ Verifikasi ke repo Laravel |
| UR-003 | Link/tombol menuju halaman pendaftaran SPA | ⬜ Kontrak integrasi TBD (§8) |
| UR-004 | Batasan wilayah konten (admin daerah hanya konten daerahnya) | ⬜ Verifikasi ke repo Laravel |

### 3.2 React SPA — pendaftaran

| ID | Kebutuhan | Status |
|---|---|---|
| UR-010 | Formulir pendaftaran multi-langkah + upload via presign | ⬜ Frontend belum ada (API ✅) |
| UR-011 | Lacak status mandiri (hibrida: nama tersamar + status + timeline; isi catatan hanya via token terbukti) | 🔶 Diputuskan, API track minimal ✅, hibrida ⬜ |
| UR-012 | Revisi berkas bertoken (bukti email+WA, sekali pakai, 24 jam) | ✅ API; UI ⬜ |
| UR-013 | Verifikasi KTA publik via QR (verdict valid/tidak) | ✅ API; UI ⬜ |
| UR-014 | Dropdown wilayah (provinsi → kabupaten) | ✅ API; UI ⬜ |

### 3.3 Go Core — mesin transaksi

| ID | Kebutuhan | Status |
|---|---|---|
| UR-020 | Login JWT + refresh rotation + logout idempoten; kebijakan sandi kuat | ✅ |
| UR-021 | RBAC 4 level + yurisdiksi wilayah server-side | ✅ |
| UR-022 | Pendaftaran: validasi kuat, NIK terenkripsi + blind index, duplikat → 409, nomor `REG-YYYYMM-XXXX` | 🔶 Nomor masih 5 digit di kode; format 4-digit diputuskan-belum-dibangun |
| UR-023 | State machine DIAJUKAN→DIVERIFIKASI→PERBAIKAN/DITOLAK/DISETUJUI; data dipertahankan | ✅ |
| UR-024 | NIA format warisan `KIPAN-{2 huruf}-{kab4}-{tahun}-{seq global}` | 🔶 Kode masih format baru; diputuskan-belum-dibangun |
| UR-025 | KTA: QR HMAC terverifikasi + PDF server-side + unduhan teraudit | ✅ |
| UR-026 | NIK reveal teraudit untuk verifikator sewilayah | ✅ |
| UR-027 | Storage presign + validasi MIME/ukuran/magic bytes; dokumen privat via URL 5 menit + audit | ✅ |
| UR-028 | Audit trail append-only semua mutasi sensitif (aktor riil, tanpa `"Admin"`) | ✅ |
| UR-029 | SK & pengurus berjenjang (D1–D5), mutasi, demisioner eksplisit | ⬜ Fase 3 (`docs/Task-Fase3/`) |
| UR-030 | Usia pendaftar 16–30 tahun (aturan warisan) | 🔶 Kode masih 17–100; diputuskan-belum-dibangun |

---

## 4. Alur end-to-end + status

| Alur | Status |
|---|---|
| Pendaftaran → verifikasi DPC → setujui → NIA → anggota (4.1 PRD lama) | ✅ Jalan (format nomor/NIA menunggu UR-022/024) |
| Revisi bertoken → DIAJUKAN ulang | ✅ Jalan |
| Birokrasi SK DPC→DPD→DPP (4.2) + demisioner konfirmasi D1 | ⬜ Fase 3 |
| Publikasi CMS berjenjang (4.3) | ⬜ Verifikasi repo Laravel |
| Kanal khusus anggota lama (berjendela; data Excel parkir) | ⏸ Menunggu data contoh |

---

## 5. Siklus hidup & kasus tepi (dari PRD §5)

| Skenario | Aturan (warisan, dipertahankan kecuali dicatat) | Status |
|---|---|---|
| Promosi jabatan | 1 orang 1 aktif (D4 versi warisan); promosi = demisioner otomatis jabatan lama; inti tunggal per SK | ⬜ Fase 3 |
| Demisioner/regenerasi | Konfirmasi eksplisit DPP (D1, deviasi resmi dari §5.1 lama) | ⬜ Fase 3 |
| Pengunduran/sanksi | Status nonaktif + riwayat, tanpa hapus | ⬜ Fase 3 |
| Mutasi domisili | Validasi wilayah baru vs master | ⬜ Fase 3 |
| Pergantian admin wilayah | Akun terikat wilayah; serah terima tercatat audit | ✅ Parsial (akun + audit ada; serah terima formal ⬜) |
| SK ditolak | Baris pengurus dibiarkan; query aktif filter SK sah (D5) | ⬜ Fase 3 |

---

## 6. Kebutuhan non-fungsional sisi pengguna

| ID | Kebutuhan | Status/keterangan |
|---|---|---|
| UN-001 | Bahasa Indonesia; aksesibilitas dasar (kontras, keyboard) | ⬜ Dinilai saat frontend ada |
| UN-002 | SLA verifikasi berkas | **TBD** (belum jelas — pemilik menentukan) |
| UN-003 | Batas operasional: file foto/KTP ≤2MB, PDF ≤5MB; presign kedaluwarsa | ✅ Ditegakkan API |
| UN-004 | Respons publik tidak membocorkan PII (NIK tersamar, dokumen via token) | ✅ Ditegakkan API |
| UN-005 | Kapasitas: gelombang provinsi (puluhan–ratusan konkuren) + 150rb anggota | ✅ Arsitektur cukup (Tahap A); uji beban pra-event |

---

## 7. Kriteria terima (sampel; lengkap menyusul di SRS)

- **UR-012**: diberikan pendaftar PERBAIKAN + bukti benar → ketika minta token → maka token 64-hex + kedaluwarsa 24 jam; dan token salah → 403 generik; dan pakai ulang → 403.
- **UR-023**: diberikan status DIAJUKAN → ketika approve langsung → maka 422; dan data + riwayat tidak terhapus saat DISETUJUI.
- **UR-025**: diberikan QR asli → verdict valid + data publik; QR ubah 1 karakter → tidak valid.
- **UR-030**: diberikan umur 15 atau 31 → maka 422 (menunggu implementasi).

---

## 8. Hal terbuka (TBD)

| # | Item | Pengganti sementara |
|---|---|---|
| TBD-01 | Kontrak integrasi Laravel↔SPA (link vs SSO) | Opsi di `docs/legacy` menyusul; mulai link biasa |
| TBD-02 | Strategi domain/subdomain | Kode agnostik via env (`KTA_VERIFY_BASE_URL`, `APP_ALLOW_ORIGIN`) |
| TBD-03 | Skema Laravel PG + verifikasi UR-001/002/004 | Menunggu akses repo Laravel |
| TBD-04 | SLA verifikasi (UN-002) | Pemilik menentukan |
| TBD-05 | Data 150rb + kanal khusus (parkir) | Prasyarat di `docs/Task-Fase1-Fase2` |
| TBD-06 | Pengesah §0.3 | Diisi manual |

---

## 9. Deviasi resmi dari perilaku lama (ringkas; detail di ADR menyusul)

D1 demisioner eksplisit (vs otomatis) · D2 nomor fisik · D3 PDF wajib sejak draf ·
D4 aturan warisan 3 lapis · D5 reject tak sentuh pengurus · umur 16–30 ·
format NIA/REG warisan · track hibrida. Semua ✅ disetujui kecuali TBD di atas.
