# S9 — Audit IDOR Mendalam (OWASP IDOR Prevention)

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif
> (read + 1x aksi ilegal yang ditolak; tanpa ubah data)
> Acuan OWASP: Insecure Direct Object Reference Prevention
> Pelengkap S2 (otorisasi) — fokus khusus semua referensi objek langsung

---

## 1. Cakupan

Seluruh endpoint berparameter objek (`:id`, `:nomor`, `?key=`, `?q=`, token
di body): pendaftaran/nik/aksi, anggota/detail/kta/reset-password,
`presign-view`, `track`, `revisi` (token↔nomor), `kta`, `cek`, `wilayah`,
`auth/me|password`, notifikasi.

## 2. Metode

Inventarisasi referensi → review binding/otorisasi per endpoint → probe live
(horizontal lintas-wilayah, vertikal peran, token lintas-nomor via kode +
gate negatif live).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S9-01 | Info | Token revisi terikat per-baris (`WHERE id AND hash`, atomik, sekali pakai NULL, error generik) — token nomor A tak bisa dipakai di nomor B by construction | Review `SubmitRevisionTx` + `SetRevisiToken` | ✅ Lolos (kode; live penuh ditunda agar tak mutasi data dev bersama — gate negatif live di S9-05) |
| S9-02 | Info | `ResetMemberPassword` cek scope + khusus role USER + cabut sesi + audit | Review + probe Q2 `resetpw-luar` **403** | ✅ Lolos |
| S9-03 | Info | Anggota detail/KTA lintas-wilayah 403; superadmin 200 | Probe Q2 | ✅ Lolos |
| S9-04 | Info | NIK reveal + presign-view lintas-wilayah 403; superadmin 200 | Probe Q3 | ✅ Lolos |
| S9-05 | Info | Token revisi sampah → 422 generik, status tetap `DIAJUKAN` (tanpa efek samping; UPDATE atomik 0-baris) | Probe Q4 | ✅ Lolos |
| S9-06 | Info | Referensi mandiri aman by-design (tanpa param): `/auth/me`, `/auth/password`, `/user/kta` (ownership `anggota.user_id`), refresh/logout via cookie | Review | ✅ Lolos |
| S9-07 | Info | Referensi publik aman by-design: `track` (DTO minimal + rate-limit), `kta`/`cek` (whitelist NIA-only), `wilayah` (master publik read-only) | Review + probe S3/S6 | ✅ Lolos |
| S9-08 | Info | Notifikasi `MarkRead` scope `user_id` (tanpa oracle); tanpa endpoint admin-users (provisioning via seeder) | Review repo + S2 | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
