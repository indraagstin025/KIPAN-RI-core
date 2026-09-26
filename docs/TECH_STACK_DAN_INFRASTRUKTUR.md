# 🛠️ TECH STACK & INFRASTRUKTUR — CORE SIM-KIPAN INDONESIA
## Dokumen Referensi Teknologi Lengkap (Fase Development → Produksi)

> **Versi**: 1.0.0 | **Terakhir diperbarui**: September 2026  
> **Strategi**: Cloudflare untuk fase awal (hemat biaya, setup cepat) → IDCloudHost untuk produksi penuh (data residency UU PDP)

---

## DAFTAR ISI
1. [Stack Overview (Ringkasan)](#1-stack-overview)
2. [Backend — Go Fiber](#2-backend--go-fiber)
3. [Frontend — React SPA](#3-frontend--react-spa)
4. [Database & Cache](#4-database--cache)
5. [Infrastruktur & Cloud](#5-infrastruktur--cloud)
6. [Keamanan & Kriptografi](#6-keamanan--kriptografi)
7. [Tooling & Developer Experience](#7-tooling--developer-experience)
8. [Cloudflare sebagai Infrastruktur Sementara](#8-cloudflare-sebagai-infrastruktur-sementara)
9. [Jalur Migrasi: Cloudflare → IDCloudHost (Produksi)](#9-jalur-migrasi-cloudflare--idcloudhost-produksi)

---

## 1. STACK OVERVIEW

```
┌──────────────────────────────────────────────────────────────────┐
│                   ARSITEKTUR SISTEM KIPAN CORE                   │
├─────────────────┬────────────────────────────────────────────────┤
│   LAYER         │   TEKNOLOGI                                    │
├─────────────────┼────────────────────────────────────────────────┤
│ Frontend SPA    │ React 19 + Vite 6 + TypeScript 5               │
│ Admin Portal    │ React Router v7 + TanStack Query v5 + Zustand  │
│ UI Component    │ shadcn/ui + Tailwind CSS v4 + Radix UI         │
├─────────────────┼────────────────────────────────────────────────┤
│ Backend API     │ Go 1.23 + Fiber v2                             │
│ Auth            │ JWT (Access 15m) + Refresh Token Rotation      │
│ Validation      │ go-playground/validator v10                    │
│ ORM / Query     │ sqlx + pgx v5 (Raw SQL, bukan ORM)            │
│ Migration       │ golang-migrate                                 │
├─────────────────┼────────────────────────────────────────────────┤
│ Database        │ PostgreSQL 16                                  │
│ Cache           │ Redis 7                                        │
│ Object Storage  │ Cloudflare R2 (dev) → IDCloudHost IS3 (prod)  │
├─────────────────┼────────────────────────────────────────────────┤
│ Reverse Proxy   │ Caddy v2 (SSL auto Let's Encrypt)             │
│ CDN             │ Cloudflare Free (dev) → IDCloudHost CDN (prod) │
│ DNS             │ Cloudflare DNS (gratis, tetap pakai di prod)   │
│ WAF / DDoS      │ Cloudflare Free WAF + DDoS Protection         │
├─────────────────┼────────────────────────────────────────────────┤
│ CI/CD           │ GitHub Actions                                 │
│ Container       │ Docker (build) — tanpa Kubernetes untuk awal  │
│ KMS             │ Env Variable (dev) → HashiCorp Vault (prod)   │
│ Monitoring      │ Prometheus + Grafana (fase lanjutan)          │
│ Log             │ zerolog (Go) + Loki (fase lanjutan)           │
└─────────────────┴────────────────────────────────────────────────┘
```

---

## 2. BACKEND — GO FIBER

### Runtime & Framework

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `go` | **1.23** | Runtime. Gunakan versi ini untuk build reproducible |
| `gofiber/fiber/v2` | **v2.52+** | HTTP framework utama. Lebih cepat dari Gin untuk throughput tinggi |
| `gofiber/helmet/v2` | latest | Security headers: CSP, HSTS, X-Frame-Options, CSRF mitigasi |
| `gofiber/limiter/v2` | latest | Rate limiting berbasis Redis sliding-window |
| `gofiber/cors/v2` | latest | CORS: hanya izinkan domain React SPA |

### Database & Query

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `jmoiron/sqlx` | **v1.4+** | Wrapper `database/sql` dengan struct scanning. **Bukan ORM** |
| `jackc/pgx/v5` | **v5.7+** | PostgreSQL driver terbaik untuk Go. Dipakai sebagai `stdlib` driver |
| `golang-migrate/migrate/v4` | latest | Migration SQL up/down. File di `backend/migrations/*.sql` |

> **Mengapa bukan GORM?**  
> GORM menyembunyikan query yang dihasilkan, mempersulit debugging query kompleks, dan rentan terhadap N+1 yang tidak disadari. `sqlx + pgx` memberikan kontrol penuh dan performa optimal.

### Auth & Kriptografi

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `golang-jwt/jwt/v5` | **v5+** | JWT: RS256 (asymmetric) atau HS256 (symmetric) |
| `alexedwards/argon2id` | latest | Argon2id untuk hash password. Lebih aman dari bcrypt |
| `aws/aws-sdk-go-v2` | latest | S3-compatible client. Dipakai untuk Cloudflare R2 & IDCloudHost IS3 |

### Validasi & Utilitas

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `go-playground/validator/v10` | v10 | Validasi struct tag. Tambahkan custom rule untuk format NIK |
| `rs/zerolog` | latest | Structured logging JSON. Cepat, zero-allocation |
| `spf13/viper` | latest | Config management: env, `.env` file, atau Vault |
| `google/uuid` | v1 | Generate UUID v4 untuk PK tabel `users`, token, dll |
| `fogleman/gg` | latest | Canvas 2D render KTA PDF server-side (Go native) |

### Testing

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `testify/suite` | latest | Test suite terstruktur (Setup/TearDown per test) |
| `DATA-DOG/go-sqlmock` | latest | Mock DB untuk unit test repository layer |
| `stretchr/testify` | latest | Assert, require helper |
| `securego/gosec` | latest | Static analysis keamanan — wajib di CI/CD pipeline |
| `golang.org/x/vuln/cmd/govulncheck` | latest | Scan dependensi Go dari CVE database resmi |

---

## 3. FRONTEND — REACT SPA

### Core

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `react` | **19** | Runtime UI |
| `react-dom` | **19** | DOM renderer |
| `typescript` | **5.6+** | Type safety. `strict: true` wajib di `tsconfig.json` |
| `vite` | **6.x** | Build tool. HMR cepat, output bundle optimal |
| `@vitejs/plugin-react` | latest | React Fast Refresh support |

### Routing & State

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `react-router-dom` | **v7** | Client-side routing SPA dengan lazy loading per route |
| `@tanstack/react-query` | **v5** | Server state: fetch, cache, refetch, optimistic update |
| `zustand` | **v5** | Client state (hanya metadata user: role, nama, wilayah) |

> **Mengapa TanStack Query + Zustand, bukan Redux?**  
> Redux terlalu verbose untuk use case ini. TanStack Query menangani 90% state (server data), Zustand hanya untuk state global ringan yang tidak perlu sync ke server.

### UI & Styling

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `tailwindcss` | **v4** | Utility-first CSS. Generasi terbaru, lebih cepat |
| `shadcn/ui` | latest | Component library berbasis Radix UI + Tailwind. Copy-paste, bukan npm install |
| `@radix-ui/react-*` | latest | Headless accessible UI primitives (dipakai oleh shadcn) |
| `lucide-react` | latest | Icon library konsisten dengan shadcn/ui |
| `class-variance-authority` | latest | Variant styling helper untuk komponen |

### HTTP & Form

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `axios` | **v1** | HTTP client. Interceptor untuk silent token refresh |
| `react-hook-form` | **v7** | Form state management. Performan, validasi ringan |
| `zod` | **v3** | Schema validation sisi client (sinkron dengan type Go via OpenAPI) |
| `@hookform/resolvers` | latest | Bridge antara react-hook-form dan Zod |

### Utilitas & DX

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `dayjs` | latest | Date formatting Indonesia (ringan, ganti moment.js) |
| `react-dropzone` | latest | Drag-and-drop upload berkas ke Presigned URL |
| `react-hot-toast` | latest | Notifikasi toast ringan |
| `@tanstack/react-table` | **v8** | Headless table dengan sorting, pagination, filter |
| `recharts` | latest | Chart statistik dashboard (bar, line, pie) |
| `qrcode.react` | latest | Render QR Code dari URL verifikasi KTA |

### Testing & Quality

| Paket | Versi | Fungsi |
|:---|:---:|:---|
| `vitest` | latest | Test runner (kompatibel Vite, pengganti Jest) |
| `@testing-library/react` | latest | Component testing berbasis DOM behavior |
| `eslint` | **v9** | Linting. Flat config |
| `prettier` | latest | Code formatting konsisten |

---

## 4. DATABASE & CACHE

### PostgreSQL 16

| Fitur yang Dipakai | Keterangan |
|:---|:---|
| **`TIMESTAMPTZ`** | Semua kolom timestamp wajib timezone-aware |
| **`UUID`** | Kolom PK tabel auth menggunakan UUID v4 |
| **`JSONB`** | Kolom `metadata` di `activity_logs` dan `persyaratan_checklist` |
| **`CHECK constraint`** | Validasi `role`, `status`, `level` di level DB |
| **Partial Unique Index** | `UNIQUE ON users(email) WHERE deleted_at IS NULL` |
| **`ON DELETE RESTRICT`** | Relasi master data: provinsi, kabupaten |
| **`ON DELETE CASCADE`** | Token, notifikasi saat user dihapus |
| **`ON DELETE SET NULL`** | Audit log tetap ada saat aktor dihapus |
| **Read Replica** | Fase lanjutan untuk read-heavy query dashboard |

### Redis 7

| Kegunaan | Implementasi | TTL |
|:---|:---|:---:|
| **Rate limiting login** | Sliding window counter per IP + per email | 15 menit window |
| **Dashboard cache** | `SETEX dashboard:stats:{role}:{wilayah}` | 5 menit |
| **JWT blacklist (logout)** | `SET jti:{jti} 1 EX {sisa_exp_token}` | Sesuai exp token |
| **Session flag** | Tandai token yang sedang proses refresh | 30 detik |

---

## 5. INFRASTRUKTUR & CLOUD

### Ringkasan Layanan Cloud

| Komponen | Fase Dev/Staging | Fase Produksi |
|:---|:---|:---|
| **DNS** | Cloudflare DNS (gratis) | Cloudflare DNS (tetap) |
| **CDN & WAF** | Cloudflare Free Plan | Cloudflare Free / IDCloudHost CDN |
| **Object Storage** | Cloudflare R2 | IDCloudHost IS3 |
| **Frontend Hosting** | Cloudflare Pages | IDCloudHost IS3 Static / Cloudflare Pages |
| **Backend VPS** | IDCloudHost VPS / VPS lain | IDCloudHost VPS |
| **Database VPS** | IDCloudHost VPS / lokal Docker | IDCloudHost VPS terpisah |
| **SSL/TLS** | Cloudflare (Full Strict) + Let's Encrypt (Caddy) | Idem |
| **KMS** | Env variable `.env` | HashiCorp Vault (self-hosted VPS) |
| **Monitoring** | Fiber `/metrics` endpoint | Prometheus + Grafana |

### Topologi Produksi di IDCloudHost

```
Internet → Cloudflare Proxy (DNS + WAF + DDoS) → IDCloudHost VPS
                  │
     ┌────────────┴──────────────┐
     ▼                           ▼
sim.kipan.id                api.kipan.id
(React SPA via CDN)         (Go Fiber via Caddy)
     │                           │
     │                    ┌──────┴──────┐
     │                    ▼            ▼
     │              PostgreSQL       Redis
     │              (VPS Private)  (same VPS as Go)
     │
     └── IDCloudHost IS3
         ├── Bucket: kipan-public  (foto profil, aset publik)
         └── Bucket: kipan-private (KTP, SK, surat sehat — Presigned URL only)
```

---

## 6. KEAMANAN & KRIPTOGRAFI

| Kebutuhan | Implementasi | Library / Standar |
|:---|:---|:---|
| **Password hashing** | Argon2id | `alexedwards/argon2id` — mem:64MB, iter:3, salt:16B |
| **Enkripsi NIK** | AES-256-GCM | Go stdlib `crypto/cipher` — unauthenticated encryption |
| **Blind Index NIK** | HMAC-SHA256 | Go stdlib `crypto/hmac` + `crypto/sha256` |
| **Signature KTA** | HMAC-SHA256 | Go stdlib — kunci terpisah dari blind index |
| **JWT** | HS256 + short TTL | `golang-jwt/jwt/v5` — Access: 15m, Refresh: 7d |
| **Token Rotation** | Refresh Token Family | Deteksi reuse → revoke seluruh family |
| **Rate Limiting** | Redis Sliding Window | `gofiber/limiter` + Redis INCRBY + EXPIRE |
| **CORS** | Whitelist origin | Hanya `https://sim.kipan.id` |
| **Security Headers** | Helmet Fiber | CSP, HSTS, X-Frame-Options, Referrer-Policy |
| **Input Validation** | Strict server-side | `go-playground/validator` — tidak pernah trust client |
| **SQL Injection** | Parameterized query | `sqlx` — **tidak ada string concatenation** di query |
| **File Upload** | Presigned PUT ke S3 | Go tidak pernah menyentuh isi file langsung |
| **Audit Trail** | Append-only `activity_logs` | `actor_id`, `ip_address`, `user_agent`, `request_id` |

---

## 7. TOOLING & DEVELOPER EXPERIENCE

### Backend

| Tool | Fungsi |
|:---|:---|
| `make` | Shortcut: `make run`, `make test`, `make migrate-up`, `make lint` |
| `air` | Hot reload Go saat development |
| `golangci-lint` | Linting komprehensif (100+ linter dalam satu tool) |
| `gosec` | Static analysis keamanan Go |
| `govulncheck` | Scan CVE di dependensi Go |
| `golang-migrate CLI` | Jalankan migration manual |
| `swaggo/swag` | Generate OpenAPI spec dari kode annotation |

### Frontend

| Tool | Fungsi |
|:---|:---|
| `pnpm` | Package manager lebih cepat dan hemat disk vs npm |
| `eslint` v9 | Linting TypeScript |
| `prettier` | Format code otomatis |
| `husky` + `lint-staged` | Pre-commit hook: lint + format sebelum commit |
| `vite-plugin-svgr` | Import SVG sebagai React component |

### Shared / DevOps

| Tool | Fungsi |
|:---|:---|
| `docker` + `docker compose` | Jalankan PostgreSQL + Redis lokal saat development |
| `GitHub Actions` | CI/CD pipeline dengan path filter |
| `git` dengan Conventional Commits | Format: `feat:`, `fix:`, `chore:`, `docs:` |

---

## 8. CLOUDFLARE SEBAGAI INFRASTRUKTUR SEMENTARA

### Mengapa Cloudflare Dulu?

| Alasan | Penjelasan |
|:---|:---|
| **Gratis & Cepat** | Cloudflare R2 gratis 10GB/bulan + **GRATIS egress** (tidak bayar bandwidth keluar) |
| **Setup Menit** | Daftar → buat bucket → dapat Access Key → langsung integrasikan ke Go |
| **Cloudflare Pages** | Deploy React SPA gratis, CDN global 300+ PoP, tanpa konfigurasi server |
| **CDN + WAF Gratis** | Perlindungan DDoS Layer 3/4/7 langsung aktif setelah domain dikonfigurasi |
| **SSL Otomatis** | Cloudflare Full Strict mode → SSL end-to-end tanpa konfigurasi manual |

### Layanan Cloudflare yang Dipakai

| Layanan | Tier | Fungsi untuk KIPAN |
|:---|:---:|:---|
| **Cloudflare DNS** | Free | DNS management utama — tetap dipakai bahkan di produksi |
| **Cloudflare CDN/Proxy** | Free | Cache aset statis, lindungi IP VPS dari publik |
| **Cloudflare WAF** | Free | 5 custom rule gratis, perlindungan dasar SQL injection, XSS |
| **Cloudflare R2** | Free 10GB | Object storage S3-compatible. **Gratis egress!** |
| **Cloudflare Pages** | Free | Host React SPA statik (500 build/bulan gratis) |
| **Cloudflare Tunnel** | Free | Opsional: expose VPS lokal ke internet tanpa port forwarding |

### Konfigurasi Cloudflare R2 di Go

```go
// pkg/storage/s3.go — Hanya ganti endpoint untuk pindah provider
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewS3Client(cfg *config.Config) *s3.Client {
    r2Resolver := aws.EndpointResolverWithOptionsFunc(
        func(service, region string, options ...interface{}) (aws.Endpoint, error) {
            return aws.Endpoint{
                // Cloudflare R2:
                URL: "https://<ACCOUNT_ID>.r2.cloudflarestorage.com",
                // IDCloudHost IS3 (produksi):
                // URL: "https://is3.cloudhost.id",
            }, nil
        },
    )
    
    awsCfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithEndpointResolverWithOptions(r2Resolver),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            cfg.StorageAccessKey,
            cfg.StorageSecretKey,
            "",
        )),
        config.WithRegion("auto"), // R2 pakai "auto", IS3 pakai "id-jkt-1" atau sesuai region
    )
    
    return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        o.UsePathStyle = true
    })
}
```

### Struktur Bucket Cloudflare R2

```
Cloudflare R2 Account
├── Bucket: kipan-public
│   ├── Access: Public (R2.dev domain atau custom domain)
│   └── Isi: foto profil anggota, avatar admin
│
└── Bucket: kipan-private
    ├── Access: Private (Presigned URL only, TTL 5 menit)
    └── Isi: scan KTP, surat sehat, SK PDF, KTA PDF
```

### ⚠️ Catatan Penting Cloudflare R2 untuk Fase Dev/Staging

> **Data Residency**: Cloudflare R2 menyimpan data di jaringan global Cloudflare, **bukan di Indonesia**. Ini **AMAN untuk development dan staging**, tetapi untuk produksi dengan data NIK, KTP, dan surat kesehatan anggota KIPAN, data wajib berada di Indonesia sesuai **UU PDP Pasal 55**. Migration ke IDCloudHost IS3 sebelum go-live wajib dilakukan untuk berkas private (KTP, surat sehat).
>
> **Pengecualian**: Berkas non-PII seperti foto publik dan aset frontend bisa tetap di Cloudflare R2 / Cloudflare Pages bahkan di produksi.

---

## 9. JALUR MIGRASI: CLOUDFLARE → IDCLOUDHOST (PRODUKSI)

```
SAAT INI (Development & Staging):
  Object Storage: Cloudflare R2 (hemat, gratis egress)
  Frontend: Cloudflare Pages (gratis)
  DNS + CDN + WAF: Cloudflare (tetap)

SEBELUM GO-LIVE (Produksi):
  ┌──────────────────────────────────────────────────────┐
  │ Yang DIPINDAH ke IDCloudHost IS3:                    │
  │   ✅ Bucket kipan-private (KTP, surat sehat, SK PDF) │
  │      → Wajib: Data PII di Indonesia (UU PDP)         │
  │                                                      │
  │ Yang TETAP di Cloudflare:                            │
  │   ✅ Bucket kipan-public (foto, thumbnail)            │
  │      → Opsional: bisa tetap di R2 (bukan PII)       │
  │   ✅ DNS + CDN + WAF                                  │
  │   ✅ Cloudflare Pages untuk React SPA                │
  └──────────────────────────────────────────────────────┘

LANGKAH MIGRASI STORAGE (otomatis via rclone):
  rclone copy r2:kipan-private is3:kipan-private \
    --progress --checksum

TIDAK perlu ganti kode Go — hanya ganti ENV variable:
  # .env.dev
  STORAGE_ENDPOINT=https://<ID>.r2.cloudflarestorage.com
  STORAGE_BUCKET_PRIVATE=kipan-private

  # .env.prod (setelah migrasi)
  STORAGE_ENDPOINT=https://is3.cloudhost.id
  STORAGE_BUCKET_PRIVATE=kipan-private
```

---

## RINGKASAN KEPUTUSAN TECH STACK

```
┌───────────────────────────────────────────────────────────────────┐
│  BACKEND    : Go 1.23 + Fiber v2 + sqlx + pgx v5                  │
│  FRONTEND   : React 19 + Vite 6 + TypeScript 5 + TanStack Query  │
│  UI         : shadcn/ui + Tailwind CSS v4                         │
│  DATABASE   : PostgreSQL 16                                       │
│  CACHE      : Redis 7 (satu VPS dengan Go Fiber)                  │
│  STORAGE    : Cloudflare R2 (dev) → IDCloudHost IS3 (prod PII)   │
│  CDN/DNS    : Cloudflare (gratis, tetap dipakai di produksi)      │
│  PROXY/SSL  : Caddy v2 (SSL auto Let's Encrypt)                  │
│  AUTH       : JWT HS256 + Refresh Token Rotation + Argon2id       │
│  CI/CD      : GitHub Actions (path filter per modul)              │
│  REPO       : Monorepo (backend/ + frontend/)                     │
│  KMS        : Env Variable → HashiCorp Vault (fase produksi)     │
└───────────────────────────────────────────────────────────────────┘
```
