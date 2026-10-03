package domain

import "time"

// Backup adalah satu entri riwayat backup database.
type Backup struct {
	ID        int       `db:"id" json:"id"`
	Filename  string    `db:"filename" json:"filename"`
	ObjectKey string    `db:"object_key" json:"object_key"`
	SizeBytes int64     `db:"size_bytes" json:"size_bytes"`
	Status    string    `db:"status" json:"status"`
	Error     *string   `db:"error" json:"error,omitempty"`
	CreatedBy *string   `db:"created_by" json:"created_by,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
