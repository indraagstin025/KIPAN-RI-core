# Task Pengerjaan Fase 3 — Tata Kelola SK & Kepengurusan (Backend)

> **Branch**: `backend-fase-3-sk-pengurus` (dari HEAD fase-2 `dc3a393`)
> **Status**: DOKUMEN RENCANA — belum ada kode.
> **Aturan skop**: backend saja. Item butuh frontend/infra → §4 (PR tersendiri).
> Acuan: `docs/TAHAPAN_PENGERJAAN_DAN_CHECKLIST_FITUR.md` (Fase 3),
> `docs/ROADMAP_*` §Fase 2, `docs/DOKUMEN_ALUR_BISNIS_*` §5, ERD v3.0.

---

## 0. Keputusan desain (disetujui pemilik)

| # | Keputusan | Implikasi teknis |
|---|---|---|
| D1 | Demisioner **butuh konfirmasi eksplisit admin DPP** (tidak otomatis saat SK baru disetujui) — ✅ disetujui | Endpoint `POST /admin/sk/:id/demisioner` terpisah: hanya `ADMIN_NASIONAL`/`SUPER_ADMIN`, hanya bila ada SK pengganti berstatus DISETUJUI untuk wilayah+level yang sama, SK lama → `Digantikan` + pengurusnya → `Demisioner` dalam 1 tx + audit; idempoten (sudah demisioner → 409 generik). **Tersulit Fase 3**: wajib integration test race 2× eksekusi (tepat 1 menang) |
| D2 | Nomor SK **dari dokumen fisik, diinput admin** (bukan generated) — ✅ disetujui | Validasi format server-side: 5–100 karakter, pola `^[A-Za-z0-9][A-Za-z0-9/._\- ]{0,99}$` (contoh `001/SK/DPP-KIPAN/I/2026`); UNIQUE nasional + 409 generik (tutup balapan 2 admin); CHECK di migrasi |
| D3 | Naskah PDF **wajib sejak draf** (dengan status draft/setujui/not-approve) — ✅ disetujui | Alasan: approve tanpa dokumen = SK bodong. Draf tanpa `sk_pdf_key` valid (HeadObject + magic) → 422; berlaku juga saat submit/review (dokumen tak bisa dicopot di tengah jalan) |
| D4 | Rangkap jabatan ikut **aturan warisan 3 lapis** (`pengurus/route.ts:57-99` proyek lama) — ✅ terjawab: (1) 1 orang maks 1× per SK, (2) jabatan inti tunggal per SK, (3) pengangkatan ke SK baru otomatis demisionerkan seluruh jabatan aktif lama ("dipromosikan") = praktis 1 aktif per orang | Enforce ketiganya di service (400/409) + test tiap lapis |
| D5 | Penolakan SK **tidak menyentuh baris pengurus** (ikut proyek lama: `approve/route.ts:25-46` hanya flip status + catatan) — ✅ diputuskan | Baris tetap terikat di SK DITOLAK yang tak berlaku; **guard wajib**: semua query "pengurus/susunan aktif" filter SK `DISETUJUI` saja agar tak bocor ke tampilan; audit penolakan tercatat |

---

## 1. Target & batas skop

**Masuk**: migrasi `surat_keputusan`/`pengurus`/`jabatan` (+seed jabatan), CRUD draf SK,
state machine approval berjenjang, upload naskah PDF via presign, pengangkatan
pengurus dari anggota ber-NIA, demisioner eksplisit (D1), mutasi jabatan +
riwayat masa bakti, status keaktifan wilayah (dihitung live), RBAC + audit +
limiter + test + pentest suite v2.7.

**Tidak masuk**: notifikasi SK (Fase 5), Turnstile (gerbang frontend), verifikasi
produksi (Fase 6), CMS/Laravel (Fase 4).

---

## 2. Tabel task

### Sprint 3.1 — Fondasi: migrasi, master jabatan, repo

| # | Task | Kriteria selesai |
|---|---|---|
| 3.1.1 | Migrasi `000008_sk_pengurus` (`surat_keputusan`, `pengurus`, `jabatan`): CHECK status/approval/level, FK RESTRICT ke wilayah/anggota/SK, UNIQUE `nomor_sk`, index `(level,prov,kab,status)` | Up/down teruji di DB bersih; CHECK tolak status BOGUS |
| 3.1.2 | Seed master jabatan (Ketua, Sekretaris, Bendahara, Bidang × NASIONAL/PROVINSI/KABUPATEN + `urutan`) via migrasi seed | `SELECT` mengembalikan hierarki lengkap |
| 3.1.3 | `SkRepository` + `PengurusRepository` + `JabatanRepository` (parameterized, kolom eksplisit — pola BE-005) | Integration test CRUD dasar hijau |
| 3.1.4 | Validasi `nomor_sk` (D2: pola + panjang + cek duplikat → 409) + unit test pola | Nomer fisik valid lolos; aneh/dobel ditolak |

### Sprint 3.2 — Lifecycle SK berjenjang

State machine (enum, bukan PATCH status — pola `ProcessApproval`):

```text
DRAFT --submit--> MENUNGGU_PROVINSI --review-prov--> MENUNGGU_NASIONAL --approve-final--> DISETUJUI
   |                      |                         |──tolak──▶ DITOLAK (terminal)
   └── SK level NASIONAL: DRAFT --submit--> MENUNGGU_NASIONAL (lewati tahap provinsi)
```

| # | Task | Kriteria selesai |
|---|---|---|
| 3.2.1 | `POST /admin/sk` (draf: nomor D2 + judul + level + wilayah + file PDF key **wajib valid, D3**) — pembuat = admin wilayahnya / super override | 201; lintas wilayah → 403; tanpa PDF → 422 |
| 3.2.2 | `POST /admin/sk/:id/submit|review|approve|tolak` — guard state + jurisdiction + catatan wajib untuk tolak; **review-prov pada SK NASIONAL → 422** (cabang skip mudah salah) | Transisi ilegal → 422; DITOLAK terminal |
| 3.2.3 | Matriks aktor: review tahap-1 hanya `ADMIN_PROVINSI` (SK kab di provinsinya); final hanya `ADMIN_NASIONAL`/`SUPER_ADMIN`; draf milik wilayah pembuat | Unit + live matrix hijau |
| 3.2.3 | Matriks aktor: review tahap-1 hanya `ADMIN_PROVINSI` (SK kab di provinsinya); final hanya `ADMIN_NASIONAL`/`SUPER_ADMIN`; draf milik wilayah pembuat | Unit + live matrix hijau |
| 3.2.4 | Naskah PDF via presign yang sudah ada (kategori baru `sk_naskah`, maks 5MB, private) + `HeadObject` + magic sebelum simpan referensi | Key fiktif → 422 |
| 3.2.5 | Tiap transisi: 1 tx + riwayat + `activity_logs` (actor riil, bukan `"Admin"`) | Audit tercatat, diverifikasi via DB |

### Sprint 3.3 — Pengurus, demisioner eksplisit, mutasi

| # | Task | Kriteria selesai |
|---|---|---|
| 3.3.1 | Angkat pengurus ke draf SK: anggota harus ber-NIA + AKTIF + sewilayah SK; jabatan wajib; aturan warisan D4 (1× per SK, inti tunggal, promosi = demisioner lama otomatis) | Anggota fiktif → 422/404; dobel/rangkap → 400/409 |
| 3.3.2 | `POST /admin/sk/:id/demisioner` (D1): prasyarat SK pengganti DISETUJUI wilayah+level sama; eksekusi SK lama → `Digantikan`, pengurus aktifnya → `Demisioner` + `tanggal_selesai`, 1 tx + audit; idempoten | Tanpa pengganti → 422; berulang → 409 generik; race 2× eksekusi → 1 menang |
| 3.3.3 | Mutasi/ganti jabatan dalam SK aktif (nasional/super): jabatan lama ditutup (`tanggal_selesai` + status), baris baru dibuka; riwayat masa bakti utuh | Riwayat per anggota since-to-date benar |
| 3.3.4 | Status keaktifan wilayah (dihitung live): `Aktif` (ada SK berlaku) / `Vakum` (kedaluwarsa) / `Belum Terbentuk`; endpoint `GET /wilayah/:id/status` | Nilai cocok dengan data SK |
| 3.3.5 | Kunci SK yang sudah DISETUJUI: susunan tak bisa diubah kecuali via mutasi/demisioner resmi | Tambah pengurus ke SK sah → 422 |
| 3.3.6 | Penolakan SK (D5, ikut warisan): baris pengurus dibiarkan + guard query aktif filter SK DISETUJUI | Tak ada bocor susunan SK mati ke tampilan aktif |

### Sprint 3.4 — Test & pentest suite v2.7

| # | Task | Kriteria selesai |
|---|---|---|
| 3.4.1 | Unit: state machine (semua transisi legal/ilegal), pola nomor SK, guard demisioner | Hijau `-race` |
| 3.4.2 | Integration: race approve ganda + demisioner ganda konkuren; rollback tx teruji | Tepat 1 pemenang |
| 3.4.3 | Suite v2.7 `SK-01…`: submit draf, tolak-tanpa-catatan → 422, lintas-wilayah approve → 403, siklus penuh → DISETUJUI, demisioner tanpa pengganti → 422, PDF naskah fiktif → 422 | Gate hijau |
| 3.4.4 | Gates penuh + migrasi DB bersih + sync checklist TAHAPAN + push + PR | Tree bersih |

---

## 3. Pola reuse dari Fase 1/2 (jangan bikin baru)

| Kebutuhan Fase 3 | Pakai yang sudah ada |
|---|---|
| Otorisasi + jurisdiction | `ActorContext.CanAccessWilayah` + `RequireRoles` + `ScopeWilayah` |
| Tx atomik + heal retry | `IssueMember`, `healKTADocument` |
| Audit | `writeAudit` terpusat (jangan duplikat lagi) |
| Upload PDF naskah | `storage_service` + kategori baru (tanpa ubah arsitektur) |
| Limiter | Registry `RateLimit` (`sk_pub`/`sk_mut` baru di tabel, bukan prefix manual) |
| DI | Struct `*Deps` (jangan positional) |
| Error | `mapDBError` (409/422), `unavailable()`, envelope standar |
| Service split | `SKSvc` + `PengurusSvc` terpisah sejak awal (pelajaran R3) |

---

## 4. Luar skop (PR tersendiri)

| Item | Kebutuhan | Pengganti sementara |
|---|---|---|
| Notifikasi SK (draf masuk, perlu sahkan) | Infra Fase 5 | Polling antrean admin |
| Turnstile endpoint SK | Frontend | Limiter + error generik |
| Verifikasi produksi | Fase 6 | `validateProduction` + docs |

---

## 5. Rencana pentest pra-Fase 4 (dieksekusi setelah §2)

Matriks 3 level × endpoint SK × dalam/luar wilayah; IDOR ID SK/pengurus acak;
state machine ilegal (approve DRAFT, demisioner tanpa pengganti, mutasi di SK
non-aktif); race approve + demisioner; abuse (naskah PDF palsu, nomor duplikat,
burst); leak (nomor SK sensitif? bukan PII — tetap cek NIK/nama tak bocor di
respons SK); gates + migrasi DB bersih. Output: `docs/LAPORAN_PENTEST_PRAFASE4.md`.

---

## 6. Prasyarat & metrik awal

- Prasyarat: branch fase-2 sudah push (`dc3a393`), gates hijau, suite v2.6 45/45.
- Metrik target: endpoint ±10 baru, migrasi 000008, suite v2.7, 0 CRITICAL/HIGH.

---

## 7. Estimasi file (±16–18 baru + 5 edit)

**Baru**: `000008_sk_pengurus.up/down.sql` (2) · `sk_repository.go`,
`pengurus_repository.go` (2) · `sk_service.go`, `pengurus_service.go` (2) ·
`sk_handler.go`, `pengurus_handler.go` (2) · test repo/service/handler (4–5) ·
laporan pentest pra-Fase4 + laporan PR Fase 3 + sync TAHAPAN (3–4 md).

**Edit**: `routes.go` (2 grup + 2 limiter registry), `pentest_suite.ps1` (SK-01…),
`TAHAPAN_…md` (centang Fase 3), `.env.example` (tidak perlu — tanpa env baru),
`go.mod` (tidak perlu — PDF/QR/presign sudah ada).

**Hemat**: struct `SuratKeputusan`/`Pengurus`/`Jabatan` sudah ada di `schema.go`.

---

## 8. Dokumentasi (ikutan Fase 3, murah)

| # | Dokumen | Keterangan |
|---|---|---|
| 8.1 | `docs/openapi.yaml` (bertahap per sprint) | Kontrak endpoint untuk frontend; cegah drift kontrak-teks |
| 8.2 | `docs/adr/ADR-001-demisioner-eksplisit.md`, `ADR-002-nomor-fisik.md` (+ D3–D5 setelah konfirmasi) | Keputusan arsitektur dirujuk konsisten, bukan tersebar |
| 8.3 | Hapus folder kosong `docs/Task-Fase-03/` (duplikat `docs/Task-Fase3/`) | Hindari kebingungan |
| 8.4 | Sinkron ERD ↔ `000008` setelah migrasi ditulis | ERD + Blueprint sudah bahas SK (49 + 24 sebutan) — tinggal selaraskan |
| 8.5 | Runbook operasional (seed jabatan, rotasi kunci, backup/restore, respons insiden) | Ditunda ke pra-produksi bila waktu mepet — catat di PR Fase 3 |
