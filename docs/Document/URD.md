# USER REQUIREMENTS DOCUMENT (URD) — SIM-KIPAN

---

## 0. Informasi Dokumen

### 0.1 Identitas Dokumen

| Atribut | Nilai |
|---|---|
| Judul | User Requirements Document — SIM-KIPAN |
| Nomor | URD-KIPAN-2026-01 |
| Versi | 1.0-draft |
| Tanggal | 28 September 2026 |
| Bahasa | Indonesia |
| Penulis | Tim Engineering |

### 0.2 Status Dokumen

DRAF — menunggu pengesahan pemilik (§14). Dokumen ini mendefinisikan kebutuhan
pengguna sebagai persiapan pembangunan website, tanpa membahas status pengerjaan.

### 0.3 Riwayat Perubahan

| Versi | Tanggal | Perubahan | Penulis |
|---|---|---|---|
| 1.0-draft | 28 Sep 2026 | Rilis draf awal terstruktur (§0–§15) | Tim Engineering |

### 0.4 Referensi

- `KIPAN_INDONESIA/PRD.md`, `DESIGN.md` (sistem sebelumnya)
- `docs/DOKUMEN_ALUR_BISNIS_DAN_CORE_SISTEM_KIPAN.md`
- `docs/ROADMAP_DAN_TAHAPAN_PENGERJAAN_SIM_KIPAN.md`
- `docs/Task-Fase3/TASK_FASE3_SK_PENGURUS.md` (keputusan D1–D5)

### 0.5 Lembar Pengesahan

_(Diisi manual saat pengesahan — lihat §14.)_

### 0.6 Definisi & Singkatan

| Istilah | Arti |
|---|---|
| NIA | Nomor Induk Anggota, format `KIPAN-{2 huruf}-{kab4}-{tahun}-{urut 5 digit}` (contoh `KIPAN-JB-3204-2026-00001`) |
| KTA | Kartu Tanda Anggota (fisik + QR terverifikasi + PDF) |
| DPC / DPD / DPP | Dewan Pimpinan Cabang (kab/kota) / Daerah (provinsi) / Pusat (nasional) |
| SK | Surat Keputusan kepengurusan |
| Blind index | Hash HMAC-SHA256 NIK untuk pencarian tanpa dekripsi |
| Kanal khusus | Jalur pendaftaran anggota lama, dibuka-tutup per periode |
| PII | Data pribadi (NIK, KTP, kontak) yang dilindungi UU PDP |

---

## 1. Pendahuluan

### 1.1 Latar Belakang

KIPAN Indonesia membutuhkan sistem informasi keanggotaan skala nasional
(38 provinsi, 514 kabupaten/kota, ±150rb anggota tercatat manual). Sistem
dibangun dengan arsitektur terpisah: Laravel CMS (halaman utama + konten),
React SPA (pendaftaran), Go Core API (mesin transaksi).

### 1.2 Permasalahan

Sistem sebelumnya (monolit) memiliki 42 temuan keamanan dan arsitektur,
di antaranya: tanpa sesi server, otorisasi dari parameter klien, enkripsi
statis, dokumen Base64 di database, audit aktor statis, dan SK dapat
disahkan pihak tak berwenang. Dokumen ini memastikan kebutuhan penggantinya
dirumuskan eksplisit agar masalah tersebut tidak terulang.

### 1.3 Tujuan Dokumen

Menjadi acuan tunggal kebutuhan pengguna yang disepakati pemilik sebelum
pembangunan danuntingkatkan ke SRS/desain; dasar traceability pengujian.

### 1.4 Tujuan Sistem

1. Pendaftaran mandiri yang aman, terverifikasi berjenjang, dan teraudit.
2. Identitas anggota tunggal (NIA) + KTA anti-pemalsuan yang dapat diverifikasi publik.
3. Tata kelola SK dan kepengurusan yang sah secara berjenjang.
4. Konten publik terpisah dari data sensitif kependudukan.
5. Kepatuhan UU PDP atas seluruh PII.

### 1.5 Outcome yang Diharapkan

- Nol pendaftaran ganda (satu NIK = satu anggota).
- Setiap status punya jejak aktor + waktu (non-repudiasi).
- KTA palsu terdeteksi saat dipindai.
- SK ilegal (tanpa naskah/tanpa wewenang) tidak dapat terbit.
- Gelombang pendaftaran per provinsi (puluhan–ratusan konkuren) berjalan lancar.

---

## 2. Ruang Lingkup

### 2.1 In Scope

Autentikasi admin; pendaftaran reguler + khusus; verifikasi berjenjang;
revisi bertoken; penerbitan NIA/KTA; SK + pengurus + demisioner + mutasi;
storage dokumen; audit trail; landing + CMS; verifikasi KTA publik.

### 2.2 Out of Scope

Aplikasi mobile native; pembayaran/iuran; integrasi Dukcapil real-time;
notifikasi WhatsApp/email otomatis tahap awal (direncanakan Fase 5);
single sign-on antar aplikasi (kontrak integrasi diputuskan terpisah).

### 2.3 Batasan

- Pendaftar wajib WNI ber-NIK 16 digit, usia 16–30 tahun.
- Satu NIK hanya untuk satu anggota aktif.
- Dokumen KTP/surat sehat tidak pernah publik; hanya via URL kedaluwarsa.
- NIK tidak pernah tersimpan/m tampil plaintext di luar proses terotorisasi.

### 2.4 Asumsi

- DPC memegang daftar anggota lokal untuk verifikasi kanal khusus.
- Admin memahami batas wilayah kerjanya masing-masing.
- Naskah SK fisik tersedia sebelum draf digital dibuat.
- Pengguna memiliki koneksi internet dan peramban modern.

### 2.5 Dependensi

- Akses repo Laravel (sudah jadi) untuk verifikasi UR CMS.
- Keputusan domain/subdomain untuk tautan dan QR.
- Data contoh 150rb anggota (struktur berkas) untuk kanal khusus.
- Penentuan SLA verifikasi oleh pemilik.

---

## 3. Gambaran Umum Sistem

### 3.1 Konteks Sistem

```text
[Masyarakat] --baca--> [Laravel: landing + CMS]
[Calon anggota] --daftar/lacak/revisi--> [React SPA] --API--> [Go Core] --simpan--> [PostgreSQL + S3]
[Admin DPC/DPD/DPP] --verifikasi/SK--> [Go Core]
[Aparat/publik] --pindai QR--> [Verifikasi KTA]
```

### 3.2 Kanal / Aplikasi

| Kanal | Fungsi | Pengguna |
|---|---|---|
| Laravel (Inertia + PostgreSQL) | Halaman utama, profil, berita, galeri, program; ber-link ke pendaftaran | Pengunjung, admin konten |
| React SPA | Formulir pendaftaran, lacak status, revisi, verifikasi KTA, dropdown wilayah, mini landing | Pendaftar, publik |
| Go Core API | Auth, RBAC, pendaftaran, NIA/KTA, SK, pengurus, storage, audit | Admin internal, SPA |

### 3.3 Interaksi Antar Sistem

- Laravel → SPA: tautan halaman pendaftaran (kontrak integrasi diputuskan terpisah; opsi SSO menyusul).
- SPA → Core: REST API berautentikasi token untuk admin; endpoint publik ber-limit untuk pendaftar.
- Core → Storage S3-kompatibel: unggah langsung via presign; baca privat via URL kedaluwarsa.
- QR KTA → URL verifikasi publik Core (tanpa login, putusan valid/tidak).

---

## 4. Pemangku Kepentingan & Peran

### 4.1 Stakeholder

Pemilik produk, Pimpinan DPP, admin wilayah (DPD/DPC), tim engineering, aparat
verifikator lapangan, masyarakat/publik.

### 4.2 User Roles

Super Admin · Admin Nasional (DPP) · Admin Provinsi (DPD) · Admin
Kabupaten/Kota (DPC) · Admin Konten (Laravel) · Pendaftar · Anggota ·
Pengunjung.

### 4.3 Hak Akses Tingkat Tinggi

| Kemampuan | Super | Nasional | Provinsi | Kab/Kota |
|---|---|---|---|---|
| Kelola akun admin | ✅ | ❌ | ❌ | ❌ |
| Verifikasi/setujui pendaftaran | Override | Override | ❌ (khusus) | ✅ wilayahnya |
| Buka NIK teraudit | ✅ | ✅ | ❌ | ✅ saat verifikasi |
| Review SK tahap-1 | ✅ | ❌ | ✅ wajib | ❌ |
| Pengesahan final SK | ✅ | ✅ otoritas | ❌ | ❌ |
| Konfirmasi demisioner | ✅ | ✅ | ❌ | ❌ |
| Baca audit penuh | ✅ | ❌ | ❌ | ❌ |

### 4.4 Yurisdiksi Wilayah

Provinsi terkungkung provinsinya; kabupaten terkungkung kabupaten/kotanya;
nasional/super akses seluruh Indonesia. Seluruh batas dipaksakan server-side;
parameter wilayah dari klien tidak pernah dipercaya untuk otorisasi.

---

## 5. User Journey & Business Process

### 5.1 Pendaftaran

Isi biodata → unggah berkas → terima nomor `REG-YYYYMM-XXXX` → data DIAJUKAN.
Aturan: NIK 16 digit valid, email/WA valid, usia 16–30, NIK belum terdaftar.

### 5.2 Verifikasi

Admin DPC membuka antrean wilayahnya → periksa berkas (NIK teraudit bila perlu)
→ DIVERIFIKASI → PERBAIKAN (wajib catatan) / DITOLAK (wajib alasan) / DISETUJUI.
Admin provinsi dilarang memverifikasi pendaftaran.

### 5.3 Perbaikan

Status PERBAIKAN → pendaftar membuktikan identitas (email DAN whatsapp
terdaftar) → terima token sekali pakai 24 jam → unggah ulang → kembali DIAJUKAN.

### 5.4 Keanggotaan

Persetujuan menerbitkan record anggota + NIA dalam satu transaksi atomik;
data pendaftaran dipertahankan (tidak dihapus) dan tertaut ke anggota.

### 5.5 KTA

QR berisi URL verifikasi bertanda tangan; hasil pindai menampilkan data publik
anggota atau putusan tidak valid. PDF KTA diterbitkan server, diunduh admin
melalui tiket kedaluwarsa yang teraudit.

### 5.6 SK & Pengurus

Draf (wajib naskah PDF) → review DPD → pengesahan DPP (SK nasional lewati
review provinsi) → DISETUJUI/DITOLAK terminal. Pengangkatan dari anggota
ber-NIA sewilayah; 1 orang maks 1× per SK; jabatan inti tunggal per SK;
promosi mendemisionerkan jabatan aktif lama otomatis.

### 5.7 CMS

Draf konten → terbit berjenjang sesuai wilayah penulis; admin daerah hanya
konten daerahnya.

### 5.8 Kanal Khusus

Sama seperti reguler + gerbang periode (di luar periode ditolak dengan pesan
jelas) + antrean terpisah. Detail menunggu data contoh.

---

## 6. Functional Requirements

### 6.1 Informasi Publik

- UR-001 Halaman profil organisasi (visi, struktur, kontak).
- UR-002 Portal berita/galeri/program berjenjang.
- UR-003 Tautan menuju pendaftaran SPA.

### 6.2 Pendaftaran

- UR-010 Formulir multi-langkah + unggah presign.
- UR-011 Nomor registrasi unik `REG-YYYYMM-XXXX`.
- UR-012 Duplikat NIK (termasuk lintas kanal) ditolak 409.

### 6.3 Verifikasi

- UR-020 Antrean terfilter wilayah + pagination + filter status.
- UR-021 Aksi verifikasi/perbaikan/tolak/setujui dengan guard status.
- UR-022 Catatan wajib untuk perbaikan dan penolakan.

### 6.4 Perbaikan

- UR-030 Token terbit hanya dengan bukti email+WA; tanpa/keliru → 403 generik.
- UR-031 Token sekali pakai, 24 jam, kuota 3/24 jam per nomor.
- UR-032 Revisi berhasil mengembalikan status DIAJUKAN + riwayat.

### 6.5 Keanggotaan

- UR-040 Penerbitan atomik pendaftaran→anggota; dilarang hapus data asal.
- UR-041 Detail anggota untuk admin sewilayah; publik tidak dapat mengakses.

### 6.6 NIA

- UR-050 Format `KIPAN-{2 huruf}-{kab4}-{tahun}-{urut global 5 digit}`;
  unik nasional; contoh `KIPAN-JB-3204-2026-00001`.

### 6.7 KTA

- UR-060 QR = URL verifikasi + HMAC; putusan seragam tanpa oracle.
- UR-061 PDF server-side; unduhan bertiket + teraudit; rotasi kunci tanpa
  mematikan kartu lama.

### 6.8 SK

- UR-070 Draf bernomor dokumen fisik (pola + panjang + unik nasional).
- UR-071 State machine DRAFT→MENUNGGU_PROVINSI→MENUNGGU_NASIONAL→DISETUJUI/DITOLAK
  (nasional lewati tahap provinsi); tolak terminal + alasan.
- UR-072 SK sah dikunci dari perubahan susunan kecuali jalur resmi.

### 6.9 Pengurus

- UR-080 Pengangkatan: anggota ber-NIA + AKTIF + sewilayah + jabatan wajib.
- UR-081 Aturan warisan: 1× per SK, inti tunggal, promosi demisionerkan yang lama.
- UR-082 Mutasi menutup jabatan lama + membuka baru; riwayat masa bakti utuh.
- UR-083 SK ditolak tidak mengubah baris pengurus; query aktif memfilter SK sah.

### 6.10 Audit

- UR-090 Setiap mutasi sensitif mencatat aktor riil, peran, IP, user-agent,
  request-ID, entitas, aksi, sebelum/sesudah (PII dimasking); append-only.

### 6.11 CMS

- UR-100 CRUD konten + status draf/terbit + scoping wilayah penulis.

### 6.12 Verifikasi Publik

- UR-110 Tanpa login; hanya data publik anggota; NIA asing = tidak valid.

### 6.13 Kanal Khusus

- UR-120 Gerbang periode; antrean terpisah; aturan verifikasi sama dengan reguler.

---

## 7. Business Rules

### 7.1 Pendaftaran

Satu NIK satu antrean aktif; field wajib lengkap; wilayah valid terhadap master
(kabupaten harus milik provinsi yang dipilih).

### 7.2 Verifikasi

Hanya DPC sewilayah (atau override nasional/super); provinsi dilarang;
setiap aksi butuh status asal yang sah; tolak/perbaikan butuh catatan.

### 7.3 NIK

16 digit dengan segmen tanggal waras; disimpan terenkripsi (AES-256-GCM,
nonce acak) + indeks buta HMAC; tampil tersamar kecuali dibuka teraudit;
tidak pernah di log.

### 7.4 NIA

Format §6.6; segmen wilayah dari kode BPS resmi; urut global anti-duplikat.

### 7.5 Keanggotaan

Status AKTIF/NONAKTIF/DEMISIONER/DIBERHENTIKAN/MENINGGAL; transisi bermakna
hukum, bukan hapus teknis.

### 7.6 KTA

QR tidak valid hanya karena NIA ada di database — signature wajib cocok
(perbandingan constant-time); kunci dapat dirotasi tanpa mematikan kartu lama.

### 7.7 SK

Nomor dari dokumen fisik (pola + unik); naskah PDF wajib sejak draf;
persetujuan berjenjang sesuai level; penolakan terminal dan beralasan.

### 7.8 Pengurus

Wajib anggota ber-NIA + jabatan; aturan warisan §6.9/UR-081.

### 7.9 Wilayah

Master tidak pernah dihapus (nonaktif saat pemekaran); relasi kabupaten⊂provinsi
ditegakkan; 38 provinsi / 514 kab-kota mengacu kode BPS.

### 7.10 Demisioner

Hanya atas konfirmasi eksplisit DPP bila ada SK pengganti yang sah
sewilayah+selevel; eksekusi 1 transaksi + audit; idempoten.

### 7.11 Mutasi

Menutup jabatan lama (tanggal selesai + status) dan membuka baris baru;
riwayat tidak boleh terputus.

### 7.12 Kanal Khusus

Dibuka-tutup per periode; di luar periode seluruh aksinya ditolak dengan pesan;
aturan verifikasi dan dedup sama dengan kanal reguler.

---

## 8. Data & Information Requirements

### 8.1 Data Pendaftar

Identitas (nama, NIK terenkripsi+indeks, tempat/tanggal lahir, JK),
kontak (email, WA), domisili (wilayah + kecamatan/desa/kodepos),
dokumen (6 key object), status + riwayat, token revisi (hash + kedaluwarsa).

### 8.2 Data Anggota

Data pendaftar yang disahkan + NIA unik + status + angkatan +
tanggal daftar/angkat + tautan pendaftaran asal.

### 8.3 Data Wilayah

Provinsi (kode BPS + kode huruf), kabupaten/kota (kode 4 digit +
relasi provinsi), flag aktif, jabatan (nama + level + urutan).

### 8.4 Data KTA

Signature QR (HMAC), key PDF, URL verifikasi + kunci penanda.

### 8.5 Data SK

Nomor fisik unik, judul, level, wilayah, tanggal terbit/berakhir,
key naskah PDF, status + status approval + penolakan, pembuat/pengesah.

### 8.6 Data Pengurus

Anggota + SK + jabatan + level/wilayah + status + keterangan +
tanggal mulai/selesai.

### 8.7 Data Audit

Aktor (id/nama/peran), IP, user-agent, request-ID, entitas + id,
aksi, sebelum/sesudah termasking, waktu. Append-only.

### 8.8 Data Publik & Privat

Publik: profil, berita terbit, hasil verifikasi KTA, status lacak tersamar.
Privat (URL kedaluwarsa + audit): NIK utuh, KTP, surat sehat, naskah SK, PDF KTA.

---

## 9. Non-Functional Requirements

### 9.1 Performance

Gelombang provinsi (puluhan–ratusan konkuren) tanpa timeout; render PDF
tidak memblokir approval lebih dari batas waktu request.

### 9.2 Availability

Target operasional dirumuskan pra-produksi; health check publik minimal,
detail internal.

### 9.3 Scalability

Arsitektur stateless; penskalaan horizontal tanpa perubahan kode;
bottleneck teridentifikasi: sequence DB, agregasi dashboard, tabel audit.

### 9.4 Security

OWASP API Top 10; tanpa secret di kode; JWT pendek + rotasi; rate limit
berlapis; presign scoped; penegakan ukuran di server.

### 9.5 Privacy

UU PDP: enkripsi PII, masking tampilan, akses teraudit, residensi data
Indonesia untuk produksi.

### 9.6 Accessibility

Kontras, navigasi keyboard, fokus terlihat (dinilai saat frontend ada).

### 9.7 Usability

Bahasa Indonesia; error berbahasa pengguna; alur pendaftaran ≤ 3 langkah.

### 9.8 Auditability

Seluruh §8.7 dapat ditelusuri per aktor/entitas/periode oleh Super Admin.

### 9.9 File Constraints

Foto/KTP ≤2MB (JPG/PNG), PDF ≤5MB; magic bytes harus cocok; key
server-generated; presign kedaluwarsa singkat.

### 9.10 Data Retention

Riwayat dan audit dipertahankan (append-only); token kedaluwarsa dibersihkan
berkala; kebijakan arsip SK lama ditetapkan pra-produksi.

---

## 10. Acceptance Criteria

### 10.1 Positive Scenarios

- Pendaftar valid menerima nomor; DPC sewilayah dapat memverifikasi hingga terbit NIA + KTA.
- Pemilik dengan bukti benar menerima token dan revisinya kembali DIAJUKAN.
- DPP mengesahkan SK; demisioner eksplisit menutup SK lama + pengurusnya.
- QR asli → valid; PDF KTA terunduh admin sewilayah.

### 10.2 Negative Scenarios

- NIK duplikat/umur di luar 16–30/nomor SK duplikat/format salah → ditolak dengan pesan aman.
- Lintas wilayah, token salah/pakai-ulang, approve dari status salah, demisioner tanpa pengganti → ditolak.
- JWT palsu/kedaluwarsa, tanpa token, rate berlebih → 401/403/429.

### 10.3 Boundary Cases

- Umur tepat 16 dan 30 diterima; 15 dan 31 ditolak.
- File tepat di batas ukuran diterima; 1 byte lebih ditolak.
- Token dipakai 2× bersamaan → tepat 1 berhasil.
- Nomor SK 5 dan 100 karakter diterima; di luar itu ditolak.

---

## 11. Reporting, Audit & Monitoring

### 11.1 Operational Reports

Antrean per status/wilayah; berkas macet; tiket presign; kuota limiter.

### 11.2 Membership Reports

Anggota per wilayah/angkatan/status; pertumbuhan per periode; NIA terbit.

### 11.3 Verification Reports

Verifikasi per verifikator; waktu tanggap; penolakan per alasan.

### 11.4 Audit Trail

Jejak §8.7; peringatan anomali (reuse token, gagal bukti massal, 429 massal).

---

## 12. Traceability Matrix

| Kebutuhan | Journey (§5) | Aturan (§7) | Kriteria (§10) |
|---|---|---|---|
| UR-010–012 | 5.1 | 7.1, 7.3 | 10.1, 10.2, 10.3 |
| UR-020–022 | 5.1–5.2 | 7.2, 7.9 | 10.2 |
| UR-030–032 | 5.3 | 7.1 | 10.1, 10.2 |
| UR-040–041, UR-050 | 5.4 | 7.4, 7.5 | 10.1 |
| UR-060–061 | 5.5 | 7.6 | 10.1, 10.2 |
| UR-070–072 | 5.6 | 7.7 | 10.1, 10.2 |
| UR-080–083 | 5.6 | 7.8, 7.10, 7.11 | 10.1, 10.2 |
| UR-090 | 11.4 | 7.5 | 10.1 |
| UR-100 | 5.7 | 7.9 | 10.1 |
| UR-110 | 5.5 | 7.6 | 10.1 |
| UR-120 | 5.8 | 7.12 | 10.1, 10.2 |

---

## 13. Open Issues / TBD

### 13.1 Open Issues

- OI-01: SLA verifikasi berkas belum ditetapkan.
- OI-02: Struktur berkas 150rb data (kolom NIA lama, peran pengunggah).
- OI-03: Kebijakan dokumen anggota lama (bebas sementara vs wajib).

### 13.2 Pending Decisions

- PD-01: Kontrak integrasi Laravel↔SPA (tautan vs SSO).
- PD-02: Strategi domain/subdomain.
- PD-03: Cakupan verifikasi repo Laravel untuk UR-001/002/004.

### 13.3 Dependencies

- DEP-01: Akses repo Laravel (blokir PD-03, UR-001/002/004).
- DEP-02: Keputusan domain (blokir PD-01 final, QR/presign URL produksi).
- DEP-03: Data contoh kanal khusus (blokir UR-120 final, OI-02/03).

---

## 14. Approval & Sign-off

_(Diisi manual saat pengesahan — lihat §0.5.)_

| Peran | Nama | Keputusan | Tanggal |
|---|---|---|---|
| Pemilik produk | _(diisi manual)_ | Setuju / Revisi | |
| Pimpinan DPP | _(diisi manual)_ | Setuju / Revisi | |
| Lead engineering | _(diisi manual)_ | Setuju / Revisi | |

---

## 15. Appendix

- A: Glosarium istilah (NIA, KTA, DPC/DPD/DPP, status-status) — dirujuk, tidak diduplikasi.
- B: Daftar dokumen acuan (§0.2) + peta ke bagian URD ini.
- C: Aturan penomoran requirement (`UR-`, `UN-`, `OI-`, `PD-`, `DEP-`) untuk traceability SRS menyusul.
