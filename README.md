# SIM-KIPAN Core — Sistem Informasi Manajemen KIPAN Republik Indonesia

Monorepo resmi Core Sistem Informasi Manajemen Kader Inti Pemuda Anti Narkoba (KIPAN) Republik Indonesia.

---

## 🏛️ Arsitektur Monorepo

```
PROJECT_KIPAN_RI/
├── backend/                  # Go Fiber v2 REST API (Clean Architecture)
│   ├── cmd/api/              # Entrypoint server
│   ├── config/               # Viper configuration loader & validation
│   ├── internal/             # Domain, Handlers, Services, Repositories, Middleware
│   ├── pkg/                  # Reusable utilities (crypto, response, nia, storage, pagination)
│   ├── migrations/           # Database migration files (PostgreSQL 16)
│   └── Makefile              # Task runner backend
│
├── frontend/                 # React 19 + Vite + TypeScript (SPA)
│   ├── src/
│   │   ├── pages/            # Public pages & Admin pages (Lazy-loaded)
│   │   ├── components/       # UI & Domain components
│   │   ├── services/         # API clients (TanStack Query + Axios)
│   │   ├── store/            # Zustand client state (Auth & UI)
│   │   └── routes/           # React Router v7 routes & Protected guards
│   └── package.json
│
├── docs/                     # Dokumentasi Arsitektur, ERD, Security, Tech Stack
├── docker-compose.dev.yml    # PostgreSQL 16 + Redis 7 + MinIO + Mailpit
└── README.md
```

---

## 🚀 Quick Start (Development)

### 1. Jalankan Infrastruktur Lokal (Docker)
Pastikan Docker Desktop aktif, lalu jalankan:
```bash
docker compose -f docker-compose.dev.yml up -d
```
Layanan yang aktif:
- **PostgreSQL**: `localhost:5432` (db: `sim_kipan`, user: `kipan_admin`)
- **Redis**: `localhost:6379`
- **MinIO S3 Console**: `http://localhost:9001` (user: `minio_admin`)
- **Mailpit Web UI**: `http://localhost:8025`

### 2. Jalankan Backend (Go Fiber)
```bash
cd backend
cp .env.example .env
go run ./cmd/api        # API HTTP
go run ./cmd/worker     # worker terpisah: antrian email + tugas terjadwal
```
Server berjalan di `http://localhost:8080`  
Health check: `http://localhost:8080/health`

> **Worker terpisah:** pengiriman email (outbox) dan tugas terjadwal **tidak**
> lagi berjalan di proses API. Jalankan `cmd/worker` (boleh >1 instance; tiap
> tugas dilindungi kunci singleton PostgreSQL advisory lock).

### 3. Jalankan Frontend (React SPA)
```bash
cd frontend
npm install
npm run dev
```
Frontend berjalan di `http://localhost:5173`

---

## 🔒 Standar Keamanan & PII
- **NIK & Nomor HP**: Dienkripsi di database dengan AES-256-GCM.
- **Pencarian Data Sensitif**: Menggunakan HMAC-SHA256 Blind Index (Deterministic searchable index).
- **Password Admin**: Dihash menggunakan Argon2id.
- **Autentikasi**: JWT Access Token (15 menit) + Refresh Token Rotation di Redis/Postgres.
- **Kepatuhan Regulasi**: UU Perlindungan Data Pribadi (UU PDP No. 27/2022).

---

## 📚 Dokumen

| Dokumen | Isi |
|---|---|
| [`AUTH_GUIDE.md`](AUTH_GUIDE.md) | Panduan autentikasi, RBAC, scope wilayah, endpoint |
| [`docs/PERBANDINGAN_ADMIN_LAMA_BARU.md`](docs/PERBANDINGAN_ADMIN_LAMA_BARU.md) | Perbandingan fitur admin project lama vs baru (non-CMS) |
| [`docs/IMPLEMENTASI_PENGANGKATAN.md`](docs/IMPLEMENTASI_PENGANGKATAN.md) | Pengangkatan kader→pengurus via SK berjenjang |
| [`docs/IMPLEMENTASI_SK_MULTILEVEL.md`](docs/IMPLEMENTASI_SK_MULTILEVEL.md) | Lanjutan SK: multi-level, Single Active SK, ganti jabatan pengurus |
| [`docs/Document/IMPLEMENTASI_SUPER_ADMIN_LANJUTAN.md`](docs/Document/IMPLEMENTASI_SUPER_ADMIN_LANJUTAN.md) | **SELESAI (B0–B9)**: SK Draft→Ajukan, jabatan berlevel, pengurus, pendaftaran draft/kedaluwarsa, master wilayah, admin nasional, manajemen pengguna, optimisasi skala |
| [`docs/RENCANA_TDD_ALIGNMENT.md`](docs/RENCANA_TDD_ALIGNMENT.md) | Rencana adopsi aturan bisnis TDD ke Go (batch per batch) + Batch 0 worker/scheduler |
| [`docs/IMPLEMENTASI_WILAYAH_NASIONAL.md`](docs/IMPLEMENTASI_WILAYAH_NASIONAL.md) | Master wilayah 38 provinsi + 514 kab/kota |
| [`docs/IMPLEMENTASI_ANTRIAN_EMAIL_STATUS.md`](docs/IMPLEMENTASI_ANTRIAN_EMAIL_STATUS.md) | **SELESAI**: antrean email (outbox) + kredensial via tautan set-password |
| [`docs/IMPLEMENTASI_KTA_CARD_DESIGN_LAMA.md`](docs/IMPLEMENTASI_KTA_CARD_DESIGN_LAMA.md) | Desain & output KTA (PDF + QR) |
| [`docs/audit-security/`](docs/audit-security/) | Laporan audit keamanan OWASP (S1–S9) |
| [`docs/EVALUASI_ARSITEKTUR_BACKEND.md`](docs/EVALUASI_ARSITEKTUR_BACKEND.md) | Evaluasi SOLID/DRY backend + rekomendasi & struktur routes |

