package domain

import "time"

// ============================================================
// EMAIL OUTBOX (ANTRIAN PENGIRIMAN EMAIL)
// ============================================================

// EmailOutboxKind jenis email dalam antrian.
type EmailOutboxKind string

const (
	EmailOutboxStatusDisetujui EmailOutboxKind = "STATUS_DISETUJUI"
	EmailOutboxStatusDitolak   EmailOutboxKind = "STATUS_DITOLAK"
	EmailOutboxStatusPerbaikan EmailOutboxKind = "STATUS_PERBAIKAN"
	EmailOutboxSetPassword     EmailOutboxKind = "SET_PASSWORD"
	EmailOutboxAkunTerhubung   EmailOutboxKind = "AKUN_TERHUBUNG"
	EmailOutboxPengangkatan    EmailOutboxKind = "PENGANGKATAN"
)

// EmailOutboxStatus kondisi baris antrian.
type EmailOutboxStatus string

const (
	EmailOutboxPending EmailOutboxStatus = "pending"
	EmailOutboxSent    EmailOutboxStatus = "sent"
	EmailOutboxFailed  EmailOutboxStatus = "failed"
)

// EmailOutbox satu baris antrian pengiriman email.
type EmailOutbox struct {
	ID            int64             `db:"id" json:"id"`
	Jenis         EmailOutboxKind   `db:"jenis" json:"jenis"`
	PendaftaranID *int              `db:"pendaftaran_id" json:"pendaftaran_id,omitempty"`
	UserID        *string           `db:"user_id" json:"user_id,omitempty"`
	ProvinsiID    *int              `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID   *int              `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	ToEmail       string            `db:"to_email" json:"to_email"`
	Subject       string            `db:"subject" json:"subject"`
	TextBody      string            `db:"text_body" json:"text_body"`
	HTMLBody      *string           `db:"html_body" json:"html_body,omitempty"`
	Status        EmailOutboxStatus `db:"status" json:"status"`
	Attempts      int               `db:"attempts" json:"attempts"`
	NextRetryAt   time.Time         `db:"next_retry_at" json:"next_retry_at"`
	SentAt        *time.Time        `db:"sent_at" json:"sent_at,omitempty"`
	LastError     *string           `db:"last_error" json:"last_error,omitempty"`
	CreatedAt     time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at" json:"updated_at"`
}
