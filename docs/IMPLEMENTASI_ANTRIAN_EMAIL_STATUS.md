# Implementasi Antrian Email Status & Distribusi Kredensial Awal Anggota

> Status: **Rancangan implementasi (belum dieksekusi)**
> Tanggal: 02 Oktober 2026
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_ANTRIAN_EMAIL_STATUS.md`

---

## 1. Latar Belakang & Masalah

Saat ini pengiriman email status pendaftaran dilakukan **segera dan fire-and-forget** di dalam aksi approval:

- `verification_service.go` → `sendStatusEmail` memakai `go func()` (`internal/service/verification_service.go:173`).
- Gagal SMTP hanya di-`log.Warn`, email **hilang** (tanpa retry).
- Proses restart tepat setelah `SETUJUI` → email **hilang**.
- Tidak ada jejak terpusat mana yang terkirim / gagal / tertunda.

Selain itu, distribusi kredensial awal anggota masih **melewati admin**:

- `ensureMemberAccount` membuat password acak 16 karakter, menyimpan hash Argon2id, lalu mengembalikan plaintext sekali ke `ApprovalResult.OneTimePassword` (`verification_service.go:223`).
- Handler meneruskannya ke respons API admin (`pendaftaran_handler.go:229-233`), dan admin harus meneruskan manual "lewat kanal resmi" yang tidak pernah didefinisikan.
- Pola serupa di reset password anggota oleh admin (`anggota_handler.go:87-103`).

Tujuan: memindahkan pengiriman email ke **antrian (outbox) yang persisten**, dan mengirim kredensial awal **langsung ke anggota** (tanpa melewati UI admin).

---

## 2. Keputusan Desain

1. **Transactional Outbox berbasis PostgreSQL** (bukan Redis / broker eksternal). Alasan: transaksional dengan perubahan status, tanpa infra baru, mudah diaudit.
2. **Worker otomatis** (poller goroutine) yang mengirim `pending`/retry — **bukan** gerbang manual. Email status bersifat transaksional dan harus tetap terkirim otomatis.
3. **Filter status (disetujui/ditolak/perbaikan) = layar pemantauan admin**, lengkap dengan tombol "kirim ulang" untuk item gagal (DLQ). Bukan gerbang persetujuan kirim.
4. **Distribusi kredensial awal via tautan set-password (Opsi A)** — tidak ada password statis yang disimpan di DB/email. (Lihat §3.1 dan §9 untuk Opsi B.)

---

## 3. Keputusan Distribusi Kredensial Awal

### 3.1 Opsi A — Tautan set-password (REKOMENDASI)

Saat `SETUJUI` dan akun USER **baru** dibuat:

1. Akun USER dibuat dengan **password acak yang tidak pernah ditampilkan** (hash Argon2id disimpan; plaintext dibuang).
2. Didaftarkan satu baris outbox `jenis=SET_PASSWORD` (berisi `user_id`, **tanpa** token).
3. Saat worker memproses baris itu, worker **menerbitkan token setup** (256-bit), menyimpan `pwsetup:<hash(token)>` → `user_id` di Redis (TTL 7 hari), lalu merakit tautan `/set-password?token=...` dan mengirim email.
4. Anggota klik tautan → set password sendiri → login.

Keunggulan: **tidak ada plaintext password maupun token yang tersimpan di tabel outbox** (token hanya di Redis + email terkirim). Sekaligus menghapus masalah "password muncul di UI admin".

> Token setup memakai mekanisme yang sama dengan reset password (`password_reset_service.go`), tapi dengan prefix `pwsetup:` dan TTL lebih panjang (7 hari, bukan 30 menit).

### 3.2 Kasus email sudah terdaftar (akun USER existing)

`ensureMemberAccount` yang menghubungkan anggota ke akun existing (bukan buat baru) **tidak menerbitkan password**. Outbox dikirim sebagai `STATUS_DISETUJUI` dengan narasi "akun Anda terhubung, login dengan kredensial yang sudah Anda miliki".

### 3.3 Kasus email dipakai akun non-USER

Sudah ditolak `409` (`verification_service.go:236-239`). Tidak ada notifikasi. Admin memperbaiki email pendaftar lalu menyetujui ulang.

---

## 4. Skema Database (migrasi baru)

Nomor berikutnya: `000014`, `000015` (saat ini tertinggi `000013_document_key_indexes`).

### 4.1 `000014_email_outbox.up.sql`

```sql
-- ============================================================
-- EMAIL OUTBOX (ANTRIAN PENGIRIMAN EMAIL STATUS)
-- Version: 000014_email_outbox.up.sql
-- ============================================================
-- Antrian persisten pengiriman email (status pendaftaran & kredensial).
-- Ditulis pada transaksi yang SAMA dengan perubahan status (transactional
-- outbox); worker membaca pending & mengirim, lalu menandai sent/failed.
-- ============================================================

CREATE TABLE IF NOT EXISTS email_outbox (
    id              BIGSERIAL PRIMARY KEY,
    jenis           VARCHAR(40) NOT NULL,
    pendaftaran_id  INTEGER,
    user_id         UUID,
    to_email        VARCHAR(255) NOT NULL,
    subject         VARCHAR(255) NOT NULL,
    text_body       TEXT NOT NULL,
    html_body       TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at         TIMESTAMP WITH TIME ZONE,
    last_error      TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_email_outbox_jenis CHECK (jenis IN (
        'STATUS_DISETUJUI', 'STATUS_DITOLAK', 'STATUS_PERBAIKAN',
        'SET_PASSWORD', 'AKUN_TERHUBUNG'
    )),
    CONSTRAINT chk_email_outbox_status CHECK (status IN ('pending', 'sent', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_email_outbox_pending
    ON email_outbox (status, next_retry_at)
    WHERE status IN ('pending');

CREATE INDEX IF NOT EXISTS idx_email_outbox_pendaftaran
    ON email_outbox (pendaftaran_id);
```

Catatan:
- `jenis` adalah enum status/kredensial; `status` adalah kondisi antrian (pending/sent/failed).
- `pendaftaran_id`/`user_id` opsional dan hanya untuk filter/link balik (tanpa FK ketat agar baris tetap utuh untuk audit walau entitas asal berubah).

### 4.2 `000014_email_outbox.down.sql`

```sql
DROP TABLE IF EXISTS email_outbox;
```

### 4.3 Tidak butuh tabel token baru

Token setup disimpan di Redis (prefix `pwsetup:`), bukan tabel — konsisten dengan alur reset password yang sudah ada.

---

## 5. Perubahan Backend

### 5.1 Domain — `internal/domain/schema.go` (atau file baru `email_outbox.go`)

```go
type EmailOutboxKind string

const (
    EmailOutboxStatusDisetujui  EmailOutboxKind = "STATUS_DISETUJUI"
    EmailOutboxStatusDitolak    EmailOutboxKind = "STATUS_DITOLAK"
    EmailOutboxStatusPerbaikan  EmailOutboxKind = "STATUS_PERBAIKAN"
    EmailOutboxSetPassword      EmailOutboxKind = "SET_PASSWORD"
    EmailOutboxAkunTerhubung    EmailOutboxKind = "AKUN_TERHUBUNG"
)

type EmailOutboxStatus string

const (
    EmailOutboxPending EmailOutboxStatus = "pending"
    EmailOutboxSent    EmailOutboxStatus = "sent"
    EmailOutboxFailed  EmailOutboxStatus = "failed"
)

type EmailOutbox struct {
    ID            int64            `db:"id" json:"id"`
    Jenis         EmailOutboxKind  `db:"jenis" json:"jenis"`
    PendaftaranID *int             `db:"pendaftaran_id" json:"pendaftaran_id,omitempty"`
    UserID        *string          `db:"user_id" json:"user_id,omitempty"`
    ToEmail       string           `db:"to_email" json:"to_email"`
    Subject       string           `db:"subject" json:"subject"`
    TextBody      string           `db:"text_body" json:"text_body"`
    HTMLBody      *string          `db:"html_body" json:"html_body,omitempty"`
    Status        EmailOutboxStatus `db:"status" json:"status"`
    Attempts      int              `db:"attempts" json:"attempts"`
    NextRetryAt   time.Time        `db:"next_retry_at" json:"next_retry_at"`
    SentAt        *time.Time       `db:"sent_at" json:"sent_at,omitempty"`
    LastError     *string          `db:"last_error" json:"last_error,omitempty"`
    CreatedAt     time.Time        `db:"created_at" json:"created_at"`
    UpdatedAt     time.Time        `db:"updated_at" json:"updated_at"`
}
```

### 5.2 Repository — `internal/repository/email_outbox_repository.go`

```go
type EmailOutboxRepository interface {
    // Enqueue menambah baris antrian (dalam transaksi caller).
    Enqueue(ctx context.Context, item *domain.EmailOutbox) error
    // ClaimNext mengambil & mengunci N baris pending siap kirim (SKIP LOCKED).
    ClaimNext(ctx context.Context, limit int) ([]domain.EmailOutbox, error)
    // MarkSent / MarkFailed memperbarui status + attempts + next_retry_at.
    MarkSent(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, errMsg string, nextRetry time.Time, dead bool) error
    // List untuk layar admin (filter jenis/status + paginasi).
    List(ctx context.Context, filter OutboxFilter) ([]domain.EmailOutbox, int, error)
    // RetryNow mengembalikan item failed ke pending untuk kirim ulang manual.
    RetryNow(ctx context.Context, id int64) error
}
```

Implementasi `ClaimNext` memakai `SELECT ... FOR UPDATE SKIP LOCKED` agar aman dipanggil banyak instance worker.

### 5.3 Enqueue pada transaksi status

`verification_service.go`:

- Ganti `sendStatusEmail` (fire-and-forget) menjadi `enqueueStatusEmail(...)` yang menulis baris outbox **di dalam transaksi yang sama** dengan `UpdateStatusWithHistory` / `IssueMember`. Untuk `SETUJUI` yang melibatkan `IssueMember` (transaksi DB sendiri), enqueue dilakukan segera setelah commit member (best-effort; bila gagal, worker tidak jalan — lihat §10 untuk retry manual).

Sketsa:

```go
// Setelah UpdateStatusWithHistory berhasil:
s.enqueueStatusEmail(ctx, item, targetStatus, note, memberNIA, userID, setupRequired)
```

- Pada `SETUJUI` dengan akun baru → `jenis=SET_PASSWORD` (tanpa token; `user_id` diisi).
- Pada `SETUJUI` dengan akun terhubung → `jenis=AKUN_TERHUBUNG`.
- Pada `PERBAIKAN`/`DITOLAK` → `jenis=STATUS_PERBAIKAN`/`STATUS_DITOLAK`.

### 5.4 Worker — `internal/worker/email_worker.go` (baru)

```go
type EmailWorker struct {
    outbox repository.EmailOutboxRepository
    mail   gateway.MailSender
    rdb    *redis.Client
    cfg    *config.Config
    interval time.Duration
    maxAttempts int
    batch  int
}

func (w *EmailWorker) Run(ctx context.Context) { /* loop: ClaimNext → process → sleep */ }
func (w *EmailWorker) processOne(item domain.EmailOutbox) error { ... }
```

Alur `processOne`:
1. Jika `jenis == SET_PASSWORD`: terbitkan token setup (reuse `crypto.GenerateSecureToken`), simpan `pwsetup:<hash>` → `user_id` (TTL 7 hari), rakit `text_body`/`html_body` dari template + tautan.
2. `mail.Send(...)`.
3. Sukses → `MarkSent`. Gagal → `MarkFailed` dengan backoff eksponensial; bila `attempts >= maxAttempts` → `status=failed` (DLQ, tanpa auto-retry).

### 5.5 Handler admin — `internal/handler/email_outbox_handler.go` (baru)

```go
// GET  /admin/email-outbox?jenis=&status=&page=&limit=   (list + filter)
// POST /admin/email-outbox/:id/retry                       (kirim ulang failed)
```

Wajib role admin (guard role ≥ `ADMIN_KABUPATEN`; list difilter wilayah bila relevan). Didaftarkan di `routes.go` pada grup `admin`.

### 5.6 Hapus `one_time_password` dari respons

- `pendaftaran_handler.go:227-234` → hapus `out["one_time_password"]`.
- `anggota_handler.go:87-103` (`ResetPassword`) → ubah jadi enqueue `SET_PASSWORD` ke outbox alih-alih mengembalikan password; respons cukup `{"success": true}`.
- `verification_service.go` → `ApprovalResult.OneTimePassword` dihapus; `ensureMemberAccount` tidak lagi mengembalikan plaintext (membuang password yang digenerate).

### 5.7 Template email — `internal/service/mail_templates.go`

Tambahkan:
- `AccountSetupEmail(nama, link, publicURL string) EmailContent` — untuk `SET_PASSWORD`.
- Ubah `StatusEmail` DISETUJUI: bagi dua narasi (akun baru vs akun terhubung) atau dikelola oleh jenis outbox terpisah (`SET_PASSWORD`/`AKUN_TERHUBUNG` memuat NIA + instruksi login; email status murni `STATUS_DISETUJUI` hanya NIA bila tidak ada akun).

### 5.8 Config — `config/config.go` + `.env.example`

Tambahkan (dengan default aman):

```env
EMAIL_WORKER_INTERVAL=10s
EMAIL_WORKER_BATCH=25
EMAIL_WORKER_MAX_ATTEMPTS=5
SETUP_TOKEN_TTL=168h
```

### 5.9 Wiring — `internal/handler/routes.go` + `cmd/api`

- Instansiasi `EmailOutboxRepository`, `EmailWorker`.
- Mulai worker di `main` (goroutine + `signal.NotifyContext` untuk graceful shutdown).
- Injeksi `EmailOutboxRepository` + `rdb` ke `VerificationService` (perluas `VerificationDeps`).

---

## 6. Perubahan Frontend

### 6.1 Hapus tampilan `one_time_password`

- `frontend/src/features/verification/types.ts:51` — hapus `one_time_password?: string`.
- `frontend/src/features/verification/pages/DetailPage.tsx:99` — `setHasil({ nia: res.nia })` (tanpa `otp`).
- `frontend/src/features/anggota/pages/AnggotaDetailPage.tsx:69-77` — ubah pesan & hilangkan penampilan password; tampilkan "Tautan set-password telah dikirim ke email anggota".
- `frontend/src/features/anggota/api/anggotaService.ts:26-28` — ubah tipe kembalian jadi `{ success: boolean }` atau `null`.

### 6.2 Halaman set-password

Reuse `frontend/src/features/auth/pages/ResetPasswordPage.tsx`:
- Tambah route `/set-password` (alias `/reset-password`) + endpoint `POST /auth/set-password { token, new_password }` di backend (mirror `reset-password`, memakai prefix `pwsetup:` dan TTL 7 hari).

### 6.3 Layar admin "Antrian Email" (baru)

Feature baru `frontend/src/features/outbox/`:
- `pages/OutboxListPage.tsx` — tabel antrian email, filter `jenis` (dropdown: Disetujui/Ditolak/Perbaikan/Set-Password/Akun-Terhubung) + `status` (Pending/Terkirim/Gagal), tombol "Kirim ulang" untuk `failed`.
- `api/outboxService.ts` — `listOutbox(...)`, `retryOutbox(id)`.
- Route di `App.tsx` (dalam `AppLayout`, guard admin).

---

## 7. Alur End-to-End (SETUJUI)

1. Admin klik "Setujui" → `POST /admin/pendaftaran/:id/setujui`.
2. `ProcessApproval`: `IssueMember` → KTA PDF → `ensureMemberAccount` (akun baru, password acak dibuang) → **enqueue outbox `SET_PASSWORD`** → audit + notifikasi in-app admin.
3. Respons ke admin **tanpa** password.
4. Worker mengambil `SET_PASSWORD` → terbitkan token setup (Redis `pwsetup:*`, 7 hari) → kirim email tautan.
5. Anggota klik tautan → halaman `/set-password` → set password → login.

Untuk `PERBAIKAN`/`DITOLAK`: enqueue `STATUS_PERBAIKAN`/`STATUS_DITOLAK` pada transaksi yang sama dengan `UpdateStatusWithHistory`.

---

## 8. Keamanan & Kepatuhan

- **Tanpa plaintext rahasia di DB**: outbox hanya menyimpan konten email non-rahasia; token setup hanya di Redis + email.
- **Masking di log**: worker tidak boleh men-log `to_email` penuh maupun token; pakai `gateway.MaskEmail`/`MaskWA` bila perlu.
- **Anti-enumeration**: endpoint list outbox hanya admin terautentikasi; retry hanya item `failed`.
- **Audit**: enqueue/sent/failed dicatat (opsional) ke `activity_logs`; pembukaan token setup dicatat (mirror reset password).
- **Rate-limit**: worker dibatasi `batch`/interval agar tidak membanjiri SMTP.
- **Fail-closed pada produksi**: `MAIL_HOST` kosong tetap ditolak saat startup (sudah ada `wireMailSender`).

---

## 9. Alternatif (Opsi B) — Password statis di antrian

Bila Anda tetap memilih password statis (bukan tautan):
- Kolom `credential` (encrypted) di `email_outbox`, dihapus/dikosongkan setelah `sent`.
- Konsekuensi: ada window plaintext (atau ciphertext dengan kunci) di DB + password statis di email. **Tidak direkomendasikan.**

---

## 10. Testing

1. **Unit**:
   - `email_outbox_repository_test.go`: enqueue/claim/sent/failed/retry (dengan `sqlmock`/test DB).
   - `email_worker_test.go`: proses sukses, gagal+backoff, dead-letter, `SET_PASSWORD` menerbitkan token.
   - `mail_templates_test.go`: template baru `AccountSetupEmail` + escape HTML.
   - `verification_service_test.go`: SETUJUI menghasilkan outbox `SET_PASSWORD`/`AKUN_TERHUBUNG`, dan **tidak** mengembalikan password.
2. **Integrasi** (CDP/pentest suite):
   - SETUJUI → email muncul di outbox → worker kirim (dev `LogMailSender` menulis log) → tidak ada `one_time_password` di respons.
   - Filter list admin per jenis/status.
   - Retry item failed berhasil.
3. **Regression**: pentest suite `scripts/pentest_suite.ps1` masih hijau.

---

## 11. Tahapan Eksekusi (urutan)

1. Migrasi `000014_email_outbox` (up/down).
2. Domain `EmailOutbox` + enum.
3. Repository `EmailOutboxRepository` + test.
4. Config keys + `.env.example`.
5. Template `AccountSetupEmail` + revisi `StatusEmail` + test.
6. Worker `EmailWorker` + test.
7. Ubah `verification_service` (enqueue, hapus plaintext return) + `anggota_handler` (reset password → enqueue) + `pendaftaran_handler` (hapus one_time_password).
8. Handler `email_outbox_handler` + routes + wiring worker di `main`.
9. Frontend: hapus `one_time_password`, halaman `/set-password`, feature `outbox` (list + filter + retry).
10. `go test ./...` + `go vet ./...` + `gofmt`; `npm run build` + `npm run lint`.
11. Update dokumentasi (AUTH_GUIDE.md, KOMPATIBILITAS_FRONTEND_LAMA.md bila relevan).

---

## 12. Keputusan Final (disetujui pemilik — 02 Okt 2026)

1. **Kirim otomatis (worker)**; tombol manual hanya untuk retry item `failed`.
2. **Distribusi kredensial: Opsi A** (tautan set-password, tanpa plaintext rahasia).
3. **TTL token setup: 7 hari** (`168h`).
4. **Worker**: interval `10s`, batch `25`, maks `5` percobaan (lalu `failed`/DLQ).
5. **Cakupan outbox**: **email status pendaftaran + kredensial awal** saja.
   - Token revisi (`revision_service.go`) dan reset password (`password_reset_service.go`) **tetap dikirim langsung** (tidak masuk outbox pada iterasi ini).

> Status dokumen: rancangan final untuk dieksekusi nanti (belum ada perubahan kode).
