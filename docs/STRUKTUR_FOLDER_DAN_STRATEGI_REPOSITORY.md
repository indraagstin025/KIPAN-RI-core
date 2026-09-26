# 📁 STRUKTUR FOLDER & STRATEGI REPOSITORY — CORE SIM-KIPAN
## Go Fiber Backend + ReactJS SPA Frontend

> **Versi**: 1.0.0  
> **Stack**: Go Fiber v2 (Backend API) + React 19 + Vite (Frontend SPA)  
> **Deployment Target**: IDCloudHost VPS (Backend) + IDCloudHost CDN (Frontend)

---

## DAFTAR ISI
1. [Rekomendasi: Monorepo vs Separate Repo](#1-rekomendasi-monorepo-vs-separate-repo)
2. [Struktur Root Monorepo](#2-struktur-root-monorepo)
3. [Struktur Folder Backend (Go Fiber)](#3-struktur-folder-backend-go-fiber)
4. [Struktur Folder Frontend (React SPA + Vite)](#4-struktur-folder-frontend-react-spa--vite)
5. [Strategi Deployment Terpisah di IDCloudHost](#5-strategi-deployment-terpisah-di-idcloudhost)
6. [CI/CD Pipeline Overview](#6-cicd-pipeline-overview)

---

## 1. REKOMENDASI: MONOREPO VS SEPARATE REPO

### Keputusan: ✅ MONOREPO dengan Deployment Terpisah

```
Satu repository Git, dua direktori independen, dua proses deploy.
```

### Perbandingan Mendalam:

| Kriteria | Monorepo | Separate Repo | Keputusan untuk KIPAN |
|:---|:---:|:---:|:---:|
| **Tim kecil / solo developer** | ✅ Lebih mudah | ❌ Context switch | Monorepo ✅ |
| **Sinkronisasi kontrak API** | ✅ Satu `git commit` untuk Go + React | ❌ Harus update 2 repo bersamaan | Monorepo ✅ |
| **Toolchain berbeda (Go vs Node)** | ⚠️ Perlu konfigurasi | ✅ Isolasi sempurna | Keduanya bisa |
| **CI/CD per modul** | ✅ Bisa pakai path filter | ✅ Native | Monorepo ✅ dengan filter |
| **Kontrol akses repositori** | ⚠️ Satu repo = satu akses | ✅ Bisa beda tim | Monorepo ✅ (tim sama) |
| **Deploy backend tanpa rebuild frontend** | ✅ Path-based trigger | ✅ Independen | Monorepo ✅ |
| **Deploy frontend tanpa restart backend** | ✅ Path-based trigger | ✅ Independen | Monorepo ✅ |
| **Onboarding developer baru** | ✅ Clone satu repo, paham segalanya | ❌ Clone 2 repo, pelajari 2 CI | Monorepo ✅ |

### Mengapa BUKAN full-merge (semua di satu proses)?

```
HINDARI ini:
  Go Fiber melayani file HTML React SPA secara langsung (embed FS)

Masalahnya:
  1. Setiap perubahan UI kecil = build ulang binary Go = restart server
  2. Go binary membawa seluruh aset frontend = ukuran binary membengkak
  3. Cache-busting & CDN untuk frontend tidak bisa dioptimalkan
  4. Tim frontend dan backend tidak bisa deploy mandiri

PILIHAN TERBAIK:
  Monorepo → dua folder → dua pipeline deploy → dua infrastruktur terpisah
  Backend Go = VPS (long-running process)
  Frontend React = IDCloudHost CDN / Static Hosting (tidak butuh server)
```

---

## 2. STRUKTUR ROOT MONOREPO

```
kipan-core/                          ← Root repositori utama
├── .github/
│   └── workflows/
│       ├── backend-ci.yml           ← CI/CD untuk Go Fiber (trigger: backend/**)
│       └── frontend-ci.yml          ← CI/CD untuk React SPA (trigger: frontend/**)
│
├── backend/                         ← Go Fiber Core API (lihat Bagian 3)
├── frontend/                        ← React 19 + Vite SPA (lihat Bagian 4)
│
├── docs/                            ← Dokumentasi teknis bersama
│   ├── api/                         ← OpenAPI / Swagger spec (YAML)
│   │   └── openapi.yaml
│   ├── BLUEPRINT_MIGRASI_GOLANG_REACT_SPA.md
│   ├── DOKUMEN_ALUR_BISNIS_DAN_CORE_SISTEM_KIPAN.md
│   └── ERD_DAN_SKEMA_DATABASE_CORE_KIPAN.md
│
├── scripts/                         ← Script utilitas operasional
│   ├── generate-keys.sh             ← Generate AES_MASTER_KEY, BLIND_INDEX_KEY, dll.
│   ├── db-backup.sh                 ← Backup PostgreSQL ke IS3
│   └── seed-admin.sh                ← Seed Super Admin pertama kali
│
├── .gitignore
├── README.md
└── docker-compose.dev.yml           ← Dev: PostgreSQL + Redis lokal
```

---

## 3. STRUKTUR FOLDER BACKEND (GO FIBER)

```
backend/
│
├── cmd/
│   └── api/
│       └── main.go                  ← Entrypoint: wiring DI, startup, graceful shutdown
│
├── config/
│   └── config.go                    ← Struct Config, parsing env, fail-fast validation
│
├── internal/
│   │
│   ├── domain/                      ← Pure domain: entity, interface, error type
│   │   ├── user.go                  ← User entity, Role enum, Status enum
│   │   ├── pendaftaran.go           ← Pendaftaran entity, Status enum
│   │   ├── anggota.go               ← Anggota entity, NIA generator interface
│   │   ├── surat_keputusan.go       ← SK entity, ApprovalStatus enum
│   │   ├── pengurus.go              ← Pengurus entity
│   │   ├── wilayah.go               ← Provinsi, Kabupaten, Jabatan entity
│   │   └── errors.go                ← Custom domain error types (ErrNotFound, ErrForbidden, dll)
│   │
│   ├── handler/                     ← HTTP layer: parse request, validasi, format response
│   │   ├── auth_handler.go          ← POST /auth/login, /refresh, /logout, PUT /auth/password
│   │   ├── pendaftaran_handler.go   ← Pendaftaran publik & admin
│   │   ├── anggota_handler.go       ← Manajemen kader
│   │   ├── sk_handler.go            ← Surat Keputusan & pengurus
│   │   ├── storage_handler.go       ← Presigned upload & view URL
│   │   ├── wilayah_handler.go       ← Master provinsi, kabupaten, jabatan
│   │   ├── user_handler.go          ← Manajemen akun admin (Super Admin only)
│   │   ├── dashboard_handler.go     ← Statistik agregasi (Redis cached)
│   │   ├── public_handler.go        ← Verifikasi KTA publik, statistik publik
│   │   ├── notification_handler.go  ← Notifikasi per admin
│   │   └── audit_handler.go         ← Activity logs (Super Admin only)
│   │
│   ├── service/                     ← Business logic, RBAC enforcement, orchestration
│   │   ├── auth_service.go          ← Login, token rotation, logout, password change
│   │   ├── pendaftaran_service.go   ← Submit, verifikasi, approval, revisi OTP
│   │   ├── anggota_service.go       ← Query kader, update status, search by NIK
│   │   ├── sk_service.go            ← Draft SK, review DPD, approval DPP, demisioner
│   │   ├── kta_service.go           ← Generate NIA, render KTA server-side, QR signature
│   │   ├── storage_service.go       ← Presigned URL generation, validasi S3 object
│   │   ├── wilayah_service.go       ← CRUD master wilayah (protected)
│   │   ├── user_service.go          ← CRUD admin, proteksi root account
│   │   ├── dashboard_service.go     ← Agregasi statistik + Redis caching
│   │   └── notification_service.go  ← Kirim & baca notifikasi
│   │
│   ├── repository/                  ← Data access: query DB, transaksi
│   │   ├── user_repository.go
│   │   ├── refresh_token_repository.go
│   │   ├── pendaftaran_repository.go
│   │   ├── anggota_repository.go
│   │   ├── sk_repository.go
│   │   ├── pengurus_repository.go
│   │   ├── wilayah_repository.go
│   │   ├── audit_log_repository.go
│   │   └── notification_repository.go
│   │
│   └── middleware/                  ← Fiber middleware chain
│       ├── auth.go                  ← JWT verify, inject claims ke context
│       ├── rbac.go                  ← Role guard & yurisdiksi wilayah
│       ├── rate_limiter.go          ← Redis sliding-window rate limit
│       ├── audit_logger.go          ← Auto-log setiap mutasi data sensitif
│       ├── request_id.go            ← Inject X-Request-ID ke setiap request
│       └── security_headers.go      ← Helmet (CSP, HSTS, X-Frame-Options, dll)
│
├── pkg/                             ← Shared library (reusable, zero domain dependency)
│   ├── crypto/
│   │   ├── aes_gcm.go               ← Encrypt/Decrypt NIK dengan AES-256-GCM
│   │   ├── blind_index.go           ← HMAC-SHA256 Blind Index untuk pencarian NIK
│   │   ├── argon2id.go              ← Hash & verify password dengan Argon2id
│   │   └── kta_signature.go         ← HMAC signature untuk QR Code KTA
│   ├── storage/
│   │   └── s3.go                    ← IDCloudHost IS3 client (aws-sdk-go-v2 compatible)
│   ├── kta/
│   │   └── renderer.go              ← Canvas/PDF render KTA server-side (fogleman/gg)
│   ├── response/
│   │   └── response.go              ← Standard JSON response wrapper & error formatter
│   ├── validator/
│   │   └── validator.go             ← go-playground/validator wrapper, NIK format check
│   ├── pagination/
│   │   └── pagination.go            ← Parsing limit/offset, max cap 100
│   └── nia/
│       └── generator.go             ← Generate NIA unik: KIPAN-[PROV]-[KAB]-[TAHUN]-[SEQ]
│
├── migrations/                      ← SQL migration files (golang-migrate)
│   ├── 000001_init_wilayah.up.sql
│   ├── 000001_init_wilayah.down.sql
│   ├── 000002_init_users_auth.up.sql
│   ├── 000002_init_users_auth.down.sql
│   ├── 000003_init_pendaftaran.up.sql
│   ├── 000003_init_pendaftaran.down.sql
│   ├── 000004_init_anggota.up.sql
│   ├── 000004_init_anggota.down.sql
│   ├── 000005_init_sk_pengurus.up.sql
│   ├── 000005_init_sk_pengurus.down.sql
│   └── 000006_init_audit_notif.up.sql
│   └── 000006_init_audit_notif.down.sql
│
├── locales/                         ← i18n pesan end-user
│   ├── id.json                      ← Bahasa Indonesia (default)
│   └── en.json                      ← English (fallback)
│
├── .env.example                     ← Template env (tanpa nilai rahasia!)
├── .gitignore
├── go.mod
├── go.sum
├── Makefile                         ← Shortcut: make run, make test, make migrate-up
└── Dockerfile                       ← Multi-stage build → binary minimal (~15MB)
```

---

## 4. STRUKTUR FOLDER FRONTEND (REACT SPA + VITE)

```
frontend/
│
├── public/
│   ├── favicon.ico
│   ├── manifest.json                ← PWA manifest
│   └── robots.txt
│
├── src/
│   │
│   ├── main.tsx                     ← Entrypoint React + Router + QueryClient Provider
│   │
│   ├── routes/                      ← Definisi routing (React Router v7)
│   │   ├── index.tsx                ← Root router: public routes + lazy admin routes
│   │   ├── ProtectedRoute.tsx       ← Guard: cek token + role sebelum render admin
│   │   └── routes.ts                ← Konstanta path URL
│   │
│   ├── pages/
│   │   │
│   │   ├── public/                  ← Bundle Publik (ringan, tidak ada kode admin)
│   │   │   ├── LandingPage.tsx      ← Halaman utama (widget statistik, profil KIPAN)
│   │   │   ├── PendaftaranPage.tsx  ← Form multi-step pendaftaran calon anggota
│   │   │   ├── LacakStatusPage.tsx  ← Cek status berdasarkan nomor REG-XXXX
│   │   │   ├── PerbaikanBerkasPage.tsx  ← Upload ulang berkas via OTP token
│   │   │   └── VerifikasiKtaPage.tsx    ← Tampilkan hasil scan QR Code KTA
│   │   │
│   │   └── admin/                   ← Bundle Admin: React.lazy() — tidak dikirim ke publik
│   │       ├── LoginPage.tsx        ← /auth/login
│   │       ├── DashboardPage.tsx    ← /admin/dashboard
│   │       ├── PendaftaranPage.tsx  ← /admin/pendaftaran (antrean verifikasi DPC)
│   │       ├── AnggotaPage.tsx      ← /admin/anggota (daftar & detail kader)
│   │       ├── SkPage.tsx           ← /admin/surat-keputusan
│   │       ├── PengurusPage.tsx     ← /admin/pengurus
│   │       ├── UsersPage.tsx        ← /admin/users (Super Admin only)
│   │       ├── AuditLogPage.tsx     ← /admin/audit-logs (Super Admin only)
│   │       └── ProfilPage.tsx       ← /admin/profil (ganti password, setting akun)
│   │
│   ├── components/
│   │   ├── ui/                      ← Komponen primitif (Button, Input, Modal, Table)
│   │   ├── layout/
│   │   │   ├── PublicLayout.tsx     ← Navbar publik + Footer
│   │   │   └── AdminLayout.tsx      ← Sidebar admin + Header + Breadcrumb
│   │   ├── forms/
│   │   │   ├── PendaftaranForm/     ← Multi-step form (Step 1 Biodata, 2 Dokumen, 3 Review)
│   │   │   └── UploadDropzone.tsx   ← Direct S3 upload via Presigned URL
│   │   └── shared/
│   │       ├── KtaCard.tsx          ← Preview KTA (read-only display, bukan generator)
│   │       ├── NikDisplay.tsx       ← Tampil NIK selalu masked: "3204************"
│   │       └── StatusBadge.tsx      ← Badge warna per status (AKTIF, DITOLAK, dll)
│   │
│   ├── hooks/
│   │   ├── useAuth.ts               ← Auth state (role, nama) dari memory/cookie
│   │   ├── useRefreshToken.ts       ← Silent refresh access token tiap 14 menit
│   │   ├── useS3Upload.ts           ← Hook: request presign → upload ke IS3 → return key
│   │   └── usePermission.ts         ← Check hak akses fitur berdasarkan role + wilayah
│   │
│   ├── api/                         ← Axios client & semua request function
│   │   ├── client.ts                ← Axios instance + interceptor silent refresh
│   │   ├── auth.api.ts
│   │   ├── pendaftaran.api.ts
│   │   ├── anggota.api.ts
│   │   ├── sk.api.ts
│   │   ├── storage.api.ts
│   │   ├── dashboard.api.ts
│   │   └── wilayah.api.ts
│   │
│   ├── store/                       ← Zustand state (HANYA metadata, bukan token!)
│   │   ├── auth.store.ts            ← { name, role, provinsiId, kabupatenId }
│   │   └── notification.store.ts
│   │
│   ├── types/                       ← TypeScript interfaces sesuai kontrak API Go
│   │   ├── api.types.ts             ← StandardResponse, PaginatedResponse, ApiError
│   │   ├── user.types.ts
│   │   ├── pendaftaran.types.ts
│   │   ├── anggota.types.ts
│   │   └── sk.types.ts
│   │
│   ├── utils/
│   │   ├── nia.ts                   ← Format & validasi NIA display
│   │   ├── date.ts                  ← Format tanggal Indonesia (dayjs)
│   │   └── mask.ts                  ← Mask NIK, WhatsApp, Email di UI
│   │
│   └── constants/
│       ├── roles.ts                 ← SUPER_ADMIN, ADMIN_NASIONAL, dll
│       └── status.ts                ← Enum status pendaftaran, anggota, SK
│
├── .env.example                     ← VITE_API_BASE_URL, VITE_APP_NAME
├── .gitignore
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── Dockerfile                       ← Build React → Nginx serve static (atau langsung CDN)
```

---

## 5. STRATEGI DEPLOYMENT TERPISAH DI IDCLOUDHOST

### Backend dan Frontend WAJIB di-deploy terpisah. Ini alasannya:

| Aspek | Backend Terpisah ✅ | Alasan |
|:---|:---|:---|
| **Skalabilitas** | Go binary bisa di-scale up di VPS lebih besar tanpa menyentuh frontend | Beban berat ada di Go, bukan di file HTML/JS |
| **Cache & CDN** | File React SPA (JS/CSS) bisa di-cache agresif di CDN (immutable hash) | Go API response tidak bisa di-cache dengan cara yang sama |
| **Zero-downtime deploy** | Frontend bisa di-update tanpa restart Go server | Update UI kecil (typo, warna) tidak perlu gangguan API |
| **Keamanan** | Frontend tidak perlu akses ke Go internals sama sekali | Hanya berkomunikasi via REST API publik |

```
┌─────────────────────────────────────────────────────────┐
│                ALUR DEPLOYMENT FINAL                     │
│                                                         │
│  Git Push ke main                                        │
│       │                                                 │
│       ├──▶ Trigger: backend/** berubah?                  │
│       │         └──▶ Build Go binary                    │
│       │               └──▶ Upload ke VPS #2 (Go Fiber)  │
│       │                     └──▶ Graceful restart        │
│       │                                                 │
│       └──▶ Trigger: frontend/** berubah?                 │
│                 └──▶ Vite Build (npm run build)         │
│                       └──▶ Upload dist/ ke IDCloudHost CDN │
│                             └──▶ Invalidate CDN cache   │
│                                                         │
│  Backend dan Frontend TIDAK saling tunggu!              │
└─────────────────────────────────────────────────────────┘
```

### Konfigurasi Caddy sebagai Reverse Proxy (VPS #1):
```
# Caddyfile di VPS Proxy
sim.kipan.id {
    # Frontend: aset statis dari CDN atau serve langsung dari /var/www/frontend
    root * /var/www/frontend/dist
    try_files {path} /index.html   # SPA fallback untuk React Router
    file_server

    # Backend API: proxy ke Go Fiber
    handle /api/* {
        reverse_proxy localhost:8080 {
            header_up X-Real-IP {remote_host}
            header_up X-Request-ID {uuid}
        }
    }

    # Security Headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server
    }
}
```

---

## 6. CI/CD PIPELINE OVERVIEW

### `.github/workflows/backend-ci.yml`
```yaml
name: Backend CI/CD
on:
  push:
    branches: [main]
    paths: ['backend/**']          # Hanya trigger jika folder backend berubah

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - name: Run tests
        run: |
          cd backend
          go test ./... -race -count=1
          go vet ./...
  
  security:
    needs: test
    steps:
      - run: go install github.com/securego/gosec/v2/cmd/gosec@latest
      - run: cd backend && gosec ./...
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: cd backend && govulncheck ./...

  deploy:
    needs: security
    steps:
      - name: Build binary
        run: |
          cd backend
          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
          go build -ldflags="-s -w" -o kipan-api ./cmd/api
      - name: Deploy to VPS
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.VPS_HOST }}
          username: deploy
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            systemctl stop kipan-api
            cp /tmp/kipan-api /opt/kipan/kipan-api
            chmod +x /opt/kipan/kipan-api
            systemctl start kipan-api
```

### `.github/workflows/frontend-ci.yml`
```yaml
name: Frontend CI/CD
on:
  push:
    branches: [main]
    paths: ['frontend/**']         # Hanya trigger jika folder frontend berubah

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - run: cd frontend && npm ci
      - run: cd frontend && npm run build
      - name: Deploy dist ke CDN / Static Hosting
        run: |
          # Upload ke IDCloudHost Object Storage bucket public
          aws s3 sync frontend/dist/ s3://kipan-frontend/ \
            --endpoint-url https://is3.cloudhost.id \
            --delete --cache-control "public,max-age=31536000,immutable"
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.IS3_KEY }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.IS3_SECRET }}
```

---

## RINGKASAN KEPUTUSAN FINAL

```
┌─────────────────────────────────────────────────────────┐
│  REKOMENDASI FINAL: MONOREPO + DEPLOYED TERPISAH         │
│                                                         │
│  Repository: satu repo (kipan-core/)                    │
│    ├── backend/   → Go Fiber Core API                   │
│    └── frontend/  → React 19 + Vite SPA                 │
│                                                         │
│  Deployment:                                            │
│    ├── Go Binary → IDCloudHost VPS #2 (systemd service) │
│    └── React dist → IDCloudHost IS3 + CDN (static host) │
│                                                         │
│  Keuntungan Utama:                                      │
│    ✅ Satu git clone → paham seluruh sistem             │
│    ✅ Kontrak API berubah → commit atomic Go + React    │
│    ✅ Deploy frontend: tanpa restart backend             │
│    ✅ Frontend di CDN: global latency rendah            │
│    ✅ Backend Go: proses ringan <50MB RAM               │
└─────────────────────────────────────────────────────────┘
```
