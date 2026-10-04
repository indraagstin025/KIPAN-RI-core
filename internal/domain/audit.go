package domain

import "time"

// AuditFilter menyaring penelusuran jejak audit (TDD §7.2). Semua field
// opsional; kosong = tanpa filter.
type AuditFilter struct {
	Action    string     // aksi (mis. SETUJU_SK, LOGIN)
	Entity    string     // entitas (mis. pengurus, surat_keputusan)
	EntityID  string     // id entitas spesifik (mis. id anggota)
	Actor     string     // pencarian nama aktor (ILIKE)
	From      *time.Time // created_at >=
	To        *time.Time // created_at <=
	Limit     int
	Offset    int
	WithTotal bool
}
