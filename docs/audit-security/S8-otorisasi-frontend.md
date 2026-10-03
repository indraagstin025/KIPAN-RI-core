# S8 — Audit Otorisasi Frontend

> Tanggal: 02 Oktober 2026 · Target: frontend React (`:5176` → backend dev)
> Acuan OWASP: Authorization, Web Frontend Security
> Pelengkap: S2 (otorisasi backend) — dokumen ini menutup sisi klien

---

## 1. Cakupan

Guard route (`RequireAuth`, `RequireRole`, `AdminOnly`), UI per-role
(navbar, sidebar, dashboard link), penanganan token (memori vs storage/URL),
konsistensi role dengan RBAC backend per halaman.

## 2. Metode

Audit kode + probe CDP headless end-to-end memakai akun USER asli
(`user.kader@kipan.id`): alur login → pendaratan → akses halaman admin
→ akses halaman miliknya.

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S8-01 | Info | Token tidak pernah di `localStorage`/`sessionStorage`/URL (hanya memori + cookie HttpOnly) | Grep nihil + `apiClient`/`env.ts` | ✅ Lolos |
| S8-02 | Info | Seluruh cek role via `isAdminRole`/`RequireRole`; tanpa bypass hardcode | Grep 10 titik, semua konsisten | ✅ Lolos |
| S8-03 | Info | Semua `/admin/*` terbungkus `AdminOnly`; `/akun/kta` khusus `USER` | `App.tsx` | ✅ Lolos |
| S8-04 | Info | Login USER mendarat `/akun/kta` (bukan `/admin`) | CDP probe | ✅ Lolos |
| S8-05 | Info | USER di `/admin/*` mendapat "Akses Ditolak" + tombol kembali (tanpa API admin terpanggil) | CDP probe | ✅ Lolos |
| S8-06 | Info | USER membuka `/akun/kta` + `/akun/password` normal | CDP probe | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode.

## 4. Verifikasi

`npm run build` hijau, `lint` tanpa warning baru; probe CDP hijau 5/5
(1 anomali awal terbukti artefak profil browser, konfirmasi profil bersih).

## 5. Kesimpulan otorisasi ujung-ke-ujung (S2 + S8)

Backend menolak (401/403) dan frontend memblokir + mengarahkan dengan benar
pada setiap kombinasi peran × rute yang diuji.
