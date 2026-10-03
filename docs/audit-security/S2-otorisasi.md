# S2 — Audit Otorisasi

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif (read + 1x aksi ilegal yang ditolak 422)
> Acuan OWASP: Authorization, IDOR Prevention, Mass Assignment
> (+ Transaction Authorization untuk aksi verifikasi)

---

## 1. Cakupan

Guard: `RequireRoles`, `ScopeWilayah`, `CanAccessWilayah`, `actorOf` (JWT-only).
Rute: seluruh grup `/admin/*`, `/user/kta`, `/notifications/*`,
`/storage/presign-view`, `/auth/me|password`.
DTO: `PendaftaranSubmitRequest`, `RevisionSubmitRequest`, payload approval
`{catatan}`, query list (page/limit/status/search).
State machine: `IsAllowedTransition` + `ProcessApproval`.

## 2. Metode

Review guard per route + DTO per handler + unit test existing
(`CanAccessWilayah` matrix, `storage_view_authz`, RevealNIK lintas wilayah)
+ probe live IDOR (anon, USER, lintas-wilayah kab/prov, transisi ilegal).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S2-01 | Info | Lapisan guard lengkap & konsisten: `Authenticate` → `RequireRoles` → `ScopeWilayah` (route) + `CanAccessWilayah` di service (independen dari middleware, dari klaim JWT — bukan input klien) | `routes.go`, `rbac.go`, 6 call-site `CanAccessWilayah` | ✅ Lolos, tanpa perubahan |
| S2-02 | Info | IDOR lintas-wilayah ditolak; list ter-scope (kosong, bukan error/bocor) | Probe: kabadm(kab=3)→detail kab=2 = **403**; list = **200 total=0** | ✅ Lolos |
| S2-03 | Info | USER ditolak di semua endpoint admin; anon 401; USER tanpa anggota → 404 (bukan 403/oracle) | Probe: anon 401; USER `/admin/*` 403; `/user/kta` 404 | ✅ Lolos |
| S2-04 | Info | State machine ditegakkan: `DIAJUKAN→SETUJI` langsung **422**, status tetap `DIAJUKAN` (tanpa efek samping — cek transisi sebelum `IssueMember`) | Probe Q5 + `IsAllowedTransition` (tanpa rule tersebut) | ✅ Lolos |
| S2-05 | Info | Mass-assignment aman: submit tanpa role/status/wilayah; aksi approval dari path (status dari server); revisi pakai token + verifikasi HeadObject; list query di-clamp | Review DTO + handler thin | ✅ Lolos |
| S2-06 | Info | BOLA storage tertutup (owner-resolve + scope + fail-closed resolver-nil + audit VIEW); notifikasi `MarkRead` scope `user_id` (tanpa oracle); `/user/kta` ownership via `anggota.user_id` | Review + `storage_view_authz_test` | ✅ Lolos |
| S2-07 | Info | Provinsi **boleh lihat** provinsinya (200) — sesuai keputusan URD Opsi A + relaksasi vs proyek lama (yang 403) | Probe Q4 **200** | ✅ Lolos (by-design) |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode pada Batch S2.

## 4. Catatan keselarasan URD (bukan temuan S2, ditunda ke eksekusi URD)

Keputusan URD "provinsi boleh lihat, **tidak boleh verifikasi**" memerlukan
satu pengetatan `ProcessApproval` (tolak aksi untuk `ADMIN_PROVINSI`, 422).
Sengaja TIDAK dikerjakan di S2 karena (a) kode saat ini konsisten-internal,
(b) perubahannya milik track URD yang sudah direncanakan. Dicatat agar tidak hilang.

## 5. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
