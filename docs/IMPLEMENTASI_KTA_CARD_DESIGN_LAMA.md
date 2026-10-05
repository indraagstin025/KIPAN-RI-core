# Implementasi KTA Card — Desain Lama (PDF, Server-Side Render)

> Status: **Rancangan implementasi (belum dieksekusi)**
> Tanggal: 02 Oktober 2026
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_KTA_CARD_DESIGN_LAMA.md`

---

## 1. Latar Belakang & Tujuan

KTA saat ini di-render server-side sebagai **PDF minimal** (`pkg/kta/kta.go`, gofpdf) berisi judul + NIA + Nama + Status + Tanggal Angkat + QR. Desain ini jauh lebih sederhana dari desain kartu di proyek lama (`KIPAN_INDONESIA/src/components/shared/KtaCardRenderer.tsx`) yang dua sisi, dekoratif, dan memuat foto + data lengkap.

Tujuan: **mengganti PDF minimal dengan desain kartu lama (dua sisi, dekorasi penuh, foto + QR), tetap sebagai PDF, dan tetap dirender server-side** agar QR tetap bertanda tangan dan foto tetap privat.

---

## 2. Keputusan Final (disetujui — 02 Okt 2026)

1. **Desain**: pakai desain lama `KtaCardRenderer.tsx` (depan + belakang, logo, gelombang warna, watermark peta, slogan).
2. **Output**: **PDF** (bukan PNG), via headless Chrome `printToPDF`.
3. **Data kartu**: NAMA, NIA (apa adanya), Wilayah (nama), TTL, Jenis Kelamin, Agama, Alamat Lengkap + **Foto** dan **QR bertanda tangan** (keduanya wajib).
4. **NIA**: **Opsi A** — kartu mencetak `anggota.nia` apa adanya (tanpa re-format). Format NIA: `KIPAN-IND-{kab}-{tahun}-{6 digit}` (contoh `KIPAN-IND-3204-2026-000001`), digenerate di `pkg/nia/generator.go`.
5. **Waktu render**: **pre-generate async** (sekali saat approve, lewat worker), unduh = presigned URL (ringan).
6. **Beban infrastruktur**: di-tuning belakangan (terisolasi di balik interface `CardRenderer`).

---

## 3. Output & Alur Unduh

- Artefak tetap disimpan di bucket **private** dengan key `kta/{nia}.pdf` (`storage_service.go:301-314`), contentType `application/pdf` — **tidak berubah**.
- Unduh tetap lewat presigned URL (TTL 5 menit):
  - Admin: `GET /admin/anggota/:id/kta` → `DownloadKTA`
  - USER mandiri: `GET /user/kta` → `DownloadMyKTA`
- Hanya **isi PDF** yang berubah (dari gofpdf → chromedp printToPDF).

> Catatan: presigned URL (5 menit) **tidak** dikirim ke email. Email (fase terpisah, lihat dokumen antrian email) mengirim tautan stabil ke halaman `/akun/kta`.

---

## 4. Sumber Data Kartu

| Field kartu | Sumber |
|---|---|
| NAMA | `Anggota.NamaLengkap` |
| NIA | `Anggota.NIA` (apa adanya) |
| Wilayah | `WilayahRepository.GetNames(ProvinsiID, KabupatenID)` → provinsi/kabupaten (nama) |
| TTL | `Anggota.TempatLahir` + `Anggota.TanggalLahir` |
| Jenis Kelamin | `Anggota.JenisKelamin` (L/P) |
| Agama | `Anggota.Agama` |
| Alamat Lengkap | `Anggota.Alamat` + `Kecamatan` + `Desa` + `KodePos` (digabung) |
| Foto | `Anggota.FotoKey` → fetch objek dari bucket uploads (lihat §6.2) |
| QR | `kta.QRBytes(verifyURL)` — verifyURL bertanda tangan (`kta_service.go:82`) |

Semua field tersedia di `domain.Anggota` (`schema.go:279`). `GetNames` sudah ada di `WilayahRepository` (`wilayah_repository.go:26`).

---

## 5. Arsitektur Render

```
Approve (SETUJUI)
  └─ IssueMember → KTAQRHash (signature) → akun USER
       └─ enqueue job "GENERATE_KTA" (bukan render langsung)

Worker (async, fase 2)
  └─ fetch foto + wilayah + QR → CardData → CardRenderer.Render → PDF
       └─ PutKTADocument (kta/{nia}.pdf) → tandai status "ready"

Unduh (admin / user)
  └─ presign PDF (tanpa render) → download_url
```

- **Fase 1 (inti):** render server-side via `CardRenderer`; bisa dijalankan sinkron dulu (di dalam `IssueKTADocument`) untuk kesederhanaan.
- **Fase 2 (opsional):** pindah render ke worker async + kolom status (lihat §10).

---

## 6. Perubahan Backend

### 6.1 `pkg/nia/generator.go` — TIDAK BERUBAH

Keputusan Opsi A. Tidak ada perubahan.

### 6.2 `pkg/storage` — tambah `Get` (baca objek penuh)

`pkg/storage/s3.go` saat ini hanya punya `PresignPostUpload`, `PresignGet`, `Stat`, `SniffHead`, `SniffTail`, `Put`, `EnsureBucket`. Tambah:

```go
// Get mengunduh seluruh isi objek dari bucket (dipakai embed foto ke PDF KTA).
func (c *Client) Get(ctx context.Context, bucket, key string) ([]byte, error)
```

`internal/service/dokumen/storage_service.go` — tambah pembaca foto dari bucket uploads:

```go
// GetUploadedObject mengembalikan isi berkas yang diunggah pendaftar
// (bucket uploads). Dipakai KTA untuk embed pas foto ke PDF.
func (s *StorageService) GetUploadedObject(ctx context.Context, key string) ([]byte, error)
```

### 6.3 `pkg/kta` — CardData + renderer baru

#### 6.3.1 `card.go` — CardData diperkaya

```go
type CardData struct {
    NIA           string
    NamaLengkap   string
    TempatLahir   string
    TanggalLahir  time.Time
    JenisKelamin  string // L | P
    Agama         string
    Alamat        string
    ProvinsiNama  string
    KabupatenNama string
    Status        string
    TanggalAngkat time.Time
    VerifyURL     string
    FotoBytes     []byte // boleh nil → placeholder ikon
    QRPng         []byte // hasil QRBytes(VerifyURL)
}
```

#### 6.3.2 `renderer.go` — interface (kunci agar beban infra bisa di-tuning nanti)

```go
// CardRenderer merender CardData menjadi artefak PDF. Dibungkus interface
// agar mesin render (chromedp/gofpdf/lainnya) dapat diganti tanpa menyentuh
// desain maupun pemanggil.
type CardRenderer interface {
    Render(ctx context.Context, card CardData) ([]byte, error)
}
```

#### 6.3.3 `html_renderer.go` — implementasi chromedp (printToPDF)

- Embed template + asset via `//go:embed`.
- `HTMLCardRenderer.Render`:
  1. Bangun HTML final (data + foto base64 + QR base64 di-inline).
  2. `chromedp` buka halaman, tunggu `document.fonts.ready`, `page.PrintToPDF` dengan ukuran kartu (A6 landscape atau ukuran kustom 480×302).
  3. Kembalikan `[]byte` PDF.

#### 6.3.4 `template.html` + `assets/`

- **`template.html`**: reproduksi desain `KtaCardRenderer.tsx` (depan + belakang) dengan **CSS inline** (bukan Tailwind), placeholder `{{ .NamaLengkap }}`, `{{ .FotoBase64 }}`, `{{ .QRBase64 }}`, dst.
- **`assets/kipan-logo.png`** dan **`assets/peta-indonesia-v3.png`**: disalin dari `KIPAN_INDONESIA/public/`, di-embed `go:embed`.
- Font: Poppins (embed woff2) atau fallback sistem; Brush Script MT untuk slogan (fallback cursive).

#### 6.3.5 `kta.go` — tetap

`QRBytes` dan `RenderPDF` (gofpdf) **dibiarkan ada** sebagai fallback/dokumentasi. `RenderPDF` bisa dipertahankan sebagai implementasi `CardRenderer` kedua (tanpa foto, untuk environment tanpa Chromium).

### 6.4 `internal/service/dokumen/kta_service.go`

#### 6.4.1 `KTADeps` — tambah dependensi

```go
type KTADeps struct {
    AnggotaRepo repository.AnggotaRepository
    WilayahRepo repository.WilayahRepository   // baru: lookup nama wilayah
    DocStore    KTADocumentStore
    AuditRepo   repository.AuditLogRepository
    Renderer    kta.CardRenderer              // baru: mesin render
}
```

#### 6.4.2 `KTADocumentStore` — tambah baca foto

```go
type KTADocumentStore interface {
    Configured() bool
    PutKTADocument(ctx context.Context, nia string, pdf []byte) (string, error)
    PresignKTADocument(ctx context.Context, key string) (string, error)
    GetUploadedObject(ctx context.Context, key string) ([]byte, error) // baru
}
```

#### 6.4.3 `IssueKTADocument` — bangun CardData + render

Alur baru (idempoten, jalur heal tetap dipertahankan di awal):

1. Cek `KTAPDFKey` sudah terisi → kembalikan key lama (jalur heal, `kta_service.go:68`).
2. `verifyURL := fmt.Sprintf("%s/v/%s?sig=%s", base, member.NIA, ktaSig)` (tidak berubah).
3. `qrPNG, _ := kta.QRBytes(verifyURL)`.
4. `prov, kab, _ := s.wilayahRepo.GetNames(ctx, member.ProvinsiID, member.KabupatenID)`.
5. `foto, _ := s.docStore.GetUploadedObject(ctx, member.FotoKey)` (best-effort; nil = placeholder).
6. Bangun `kta.CardData{...}`.
7. `pdf, err := s.renderer.Render(ctx, card)` (gagal → return error, approve gagal, admin retry — konsisten fail-closed).
8. `s.docStore.PutKTADocument(ctx, member.NIA, pdf)` + `SetKTAPDFKey` (tidak berubah).

### 6.5 Config — `config/config.go` + `.env.example`

Tambahkan (default aman, tuning infra belakangan):

```env
KTA_RENDER_ENGINE=chromedp        # chromedp | gofpdf
KTA_RENDER_TIMEOUT=20s
KTA_RENDER_CHROME_PATH=           # kosong = gunakan default headless shell
KTA_VERIFY_BASE_URL=https://kipan.id
```

### 6.6 Wiring — `internal/handler/routes.go`

- Instansiasi `kta.CardRenderer` (pilih engine berdasar `KTA_RENDER_ENGINE`).
- Inject ke `KTADeps` (tambah `WilayahRepo: wilayahRepo`, `Renderer: renderer`).

---

## 7. Perubahan Frontend (minimal)

Frontend sudah format-agnostic — `adminKtaUrl` (`anggotaService.ts:22`) dan `getMyKta` (`ktaService.ts:11`) hanya menerima `download_url`. Yang perlu:

1. Tampilkan hasil sebagai PDF lewat **PDF viewer** (`<iframe src={download_url}>` / `<embed>`), bukan `window.open`/unduh langsung, di `AnggotaDetailPage` / `KtaSayaPage`.
2. (Fase 2) Tampilkan status "KTA sedang dibuat" bila render async belum selesai.

---

## 8. Keamanan

- QR tetap **bertanda tangan HMAC** (`crypto.KTASignature`, `RULES 20`); kunci `KTA_SIGNING_KEY` tidak pernah keluar server.
- Foto tetap di bucket uploads/privat; di-fetch server, tidak diekspos URL publik langsung.
- Render **fail-closed**: gagal render foto/QR/PDF → `IssueKTADocument` error → approve gagal → admin retry (jalur heal).
- Unduh tetap **teraudit** (`VIEW`, actor, IP, request-id) — `GetKTADocumentURL` / `GetMyKTADocumentURL` tidak berubah.
- Template HTML di-escape (anti injeksi) untuk semua field teks dari DB.

---

## 9. Testing

1. **Unit**:
   - `pkg/kta/card_test.go`: CardData validasi (field wajib, foto/QR opsional).
   - `pkg/kta/html_renderer_test.go`: render menghasilkan PDF valid (cek magic `%PDF-`), foto/QR ter-embed.
   - `pkg/storage/s3_test.go`: `Get` (berhasil / objek tidak ada).
   - `internal/service/dokumen/kta_service_test.go`: `IssueKTADocument` idempoten + fail-closed (renderer error → error).
2. **Integrasi** (CDP/pentest):
   - `SETUJUI` → `kta/{nia}.pdf` tersimpan → `GET /admin/anggota/:id/kta` & `/user/kta` mengembalikan `download_url` PDF valid.
   - QR hasil PDF tetap tervalidasi (`VerifyKTA` dengan sig dari DB).
3. **Regression**: `go test ./...`, `go vet ./...`, `gofmt`; pentest suite tetap hijau.

---

## 10. Fase 2 (opsional): Render Async + Status KTA

Bila render chromedp dirasa berat di request approve:

1. Migrasi `000015_kta_render_status.up.sql`:
   ```sql
   ALTER TABLE anggota ADD COLUMN IF NOT EXISTS kta_render_status VARCHAR(20) NOT NULL DEFAULT 'pending';
   -- pending | ready | failed
   ```
2. Ganti panggilan `IssueKTADocument` sinkron di `ProcessApproval` dengan enqueue job (tabel `kta_render_jobs` atau pola outbox serupa dokumen antrian email).
3. Worker memproses job → render → `PutKTADocument` → set `kta_render_status='ready'` (gagal → `failed` + retry/DLQ).
4. `GetKTADocumentURL`/`GetMyKTADocumentURL` cek status: `pending` → respons "KTA sedang dibuat", `failed` → error; `ready` → presign.

> Fase 2 menunda keharusan menyiapkan Chromium di path approve; tetap perlu retry + notifikasi admin bila gagal.

---

## 11. Tahapan Eksekusi (urutan)

1. `pkg/storage`: tambah `Get` + test.
2. `internal/service/dokumen/storage_service.go`: `GetUploadedObject` + test.
3. `pkg/kta`: `card.go` (CardData), `renderer.go` (interface), `template.html`, `assets/` (embed), `html_renderer.go` (chromedp), `go.mod` tambah `chromedp`.
4. `config`: `KTA_RENDER_*`.
5. `internal/service/dokumen/kta_service.go`: `KTADeps` + `KTADocumentStore` + `IssueKTADocument` baru + test.
6. `routes.go`: wiring renderer + wilayahRepo.
7. Frontend: buka `download_url` sebagai PDF (preview/download).
8. `go test ./...` + `go vet ./...` + `gofmt`; `npm run build` + `npm run lint`.
9. (Opsional) Fase 2 async + migrasi `000015`.

---

## 12. Keputusan Final (disetujui — 02 Okt 2026)

1. **Alamat Lengkap**: gabungkan `Alamat + Kecamatan + Desa + KodePos`.
2. **Preview PDF di frontend**: gunakan **PDF viewer** (`<iframe>`/`<embed>` inline), bukan unduh langsung.
3. **Renderer fallback**: setuju — bila Chromium belum tersedia, fallback ke `RenderPDF` (gofpdf).
4. **Fase 2 (async)**: menyusul setelah Fase 1 hijau (default).
