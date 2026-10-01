# Panduan Kompatibilitas: Port UI `KIPAN_INDONESIA` ke Backend Go

> Branch: `backend-fase-1-fase-2-penyesuaian`
> Tujuan: frontend React SPA baru meniru UX proyek lama tanpa
> menghidupkan kembali 42 temuan keamanan (`docs/daftar_masalah_*`).

## 1. Peta Endpoint (lama → baru)

| Kebutuhan UI lama | Endpoint lama (Next.js) | Endpoint baru (Go `/api/v1`) | Catatan |
|---|---|---|---|
| Submit pendaftaran | `POST /api/pendaftaran` | `POST /pendaftaran` | Body `snake_case`, `provinsi_id/kabupaten_id` int, dokumen berupa `*_key` (bukan base64) |
| Lacak status | `GET /api/pendaftaran/track?nomor=` | `GET /pendaftaran/track/:nomor` | Path param; respons **MINIMAL: nomor, status, label, timestamp** (TANPA PII). Limit 15/mnt per-nomor + 30/mnt per-IP. Data kaya (nama/wilayah/timeline) menyusul via jalur berpruf pemilik (SEC-TRACK-PII). |
| Minta token revisi | — (tidak ada, langsung PUT) | `POST /pendaftaran/revisi/request-token` | Body `{nomor, email, whatsapp}`; limit 3/24 jam per nomor |
| Kirim revisi | `PUT /api/pendaftaran/perbaikan/[nomor]` | `PUT /pendaftaran/revisi/:nomor` | Body `{token, *_key, catatan}` |
| Antrean admin | `GET /api/pendaftaran?role=&wilayah=` | `GET /admin/pendaftaran?page&limit&status` | Auth Bearer JWT; scope dari klaim, bukan query |
| Detail admin | (satu respons berisi NIK) | `GET /admin/pendaftaran/:id` + `GET /admin/pendaftaran/:id/nik` | NIK dibuka per-item, teraudit |
| Verifikasi | `PATCH /api/pendaftaran/[id]/verifikasi {status}` | `POST /admin/pendaftaran/:id/verifikasi\|perbaikan\|tolak\|setujui {catatan}` | State machine: `DIAJUKAN→DIVERIFIKASI→…`; setuju langsung dari `DIAJUKAN` ditolak 422 |
| Daftar anggota | `GET /api/anggota?role=&wilayah=` | `GET /admin/anggota?page&limit&status&search` | Proyeksi non-PII; NIK tidak dibulk-decrypt |
| Detail anggota | `GET /api/anggota/[id]/detail` (NIK decrypt) | `GET /admin/anggota/:id` | NIK tetap `json:"-"`; buka via pola RevealNIK bila perlu |
| Cek publik | `GET /api/anggota/cek?q=NIK_atau_NIA` | `GET /anggota/cek?q=NIA` | Hanya NIA; pencarian NIK publik ditolak by design. Respons **whitelist: nama, wilayah, status** saja (tanpa NIK/kontak/alamat/tanggal angkat). Limit 30/mnt per-IP. |
| Verifikasi KTA | — (QR teks biasa) | `GET /pendaftaran/kta/:nia?sig=` | Verdict HMAC `{nia, valid, nama, status}` |
| Upload | `POST /api/upload` (base64) | `POST /storage/presign-upload` → PUT biner ke S3 | Key server-generated; `GET /storage/presign-view` wajib auth |
| Wilayah dropdown | `MASTER_WILAYAH` statis + `/wilayah/kecamatan`, `/wilayah/kodepos` | `GET /wilayah/provinsi`, `GET /wilayah/kabupaten/:provinsi_id` | Kecamatan/kodepos dikirim sebagai teks bebas (lihat §5) |

## 2. Peta Field Form → Submit

`PendaftaranAnggota.tsx` (`camelCase`) → `PendaftaranSubmitRequest` (`snake_case`):

`namaLengkap→nama_lengkap`, `nik→nik`, `tempatLahir→tempat_lahir`,
`tanggalLahir→tanggal_lahir` (**RFC3339**, cth `1998-05-15T00:00:00Z`;
`YYYY-MM-DD` saja ditolak 422), `jenisKelamin→jenis_kelamin` (`L`/`P`),
`agama→agama`, `pendidikan→pendidikan`, `pekerjaan→pekerjaan`,
`status→status_pribadi`, `alamat→alamat` (10–2000 karakter),
`provinsiKode/kabupatenKode→provinsi_id/kabupaten_id`
(lookup via §1, **tanpa auto-create**), `kecamatan→kecamatan`,
`kodePos→kode_pos`, `email→email`, `whatsapp→whatsapp`
(`^(+62|62|0)8[1-9]…`, cth `081234567890`), `motivasi→motivasi` (≤1000),
`persyaratan[bool]→persyaratan[string]` (maks 20 item @1–100 karakter;
kirim label yang dicentang),
`foto/ktp/cv/sk/suratPernyataan/suratSehat (base64)→*_key` (via presign §3).

Aturan keras yang dipertahankan: tolak `<>` (XSS), NIK 16 digit +
segmen tanggal plausibel, umur **16–30** (keputusan pemilik, selaras lama),
duplikat NIK di 2 tabel → 409.

## 3. Alur Upload Baru (pengganti base64)

1. `POST /storage/presign-upload {kategori, nama_file, mime, ukuran}` →
   `{upload_url, object_key, kedaluwarsa}` (kategori: `foto/ktp/cv/sk/
   surat_pernyataan/surat_sehat`; foto/ktp maks 2 MB).
2. `PUT` biner langsung ke `upload_url` (bypass backend).
3. Submit pendaftaran dengan `object_key` sebagai `*_key`.
Backend memverifikasi via `HeadObject` + magic bytes **sebelum**
mengalokasikan nomor REG (key fiktif → 422).

## 4. Auth: `?role=&wilayah=` → Bearer JWT

Hapus seluruh `?role=${role}&wilayah=${wilayah}` (spoofable, No.7).
Admin login via `POST /auth/login` → access token 15 mnt (Bearer) +
refresh HttpOnly cookie; klaim `role/provinsi_id/kabupaten_id` menegakkan
scope di service (`CanAccessWilayah`). Tombol verifikasi UI dipetakan ke
4 endpoint aksi (§1); respons 422 = transisi ilegal, 403 = lintas wilayah.

## 5. Keputusan Kecamatan/Kodepos: Teks Bebas

Backend **sengaja tidak** menambah tabel master kecamatan/kodepos:
tidak ada sumber seed otoritatif di repo dan server sudah menerima
keduanya sebagai teks bebas (`kecamatan` ≤100, `kode_pos` ≤10).
Frontend boleh memakai `MASTER_KECAMATAN` lama murni sebagai bantuan
dropdown UX; nilai terpilih dikirim apa adanya.

## 6. Format Nomor

- REG: generator selalu `REG-YYYYMM-XXXXX` (5 digit, atomik); validator
  menerima `XXXX` warisan selama migrasi data (`^REG-\d{6}-\d{4,5}$`).
- NIA: `KIPAN-[kodeBPSprov]-[kodeBPSkab]-[tahun]-[seq per wilayah-tahun]`
  (cth `KIPAN-32-3273-2026-00001`); cek publik menerima varian huruf lama
  (`KIPAN-JB-…`) apa adanya dari database hasil migrasi.

## 7. Yang Tidak Diport (Fase 3–5)

Notifikasi in-app (`?userId=` IDOR — diganti desain aman §Batch D),
berita/galeri/program (CMS Laravel, Fase 4), SK/pengurus (Fase 3),
kirim token via WA/email (Fase 5; token masih via respons + bukti ganda).
