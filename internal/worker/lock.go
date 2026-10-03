// Package worker menyediakan biner worker terpisah: penjadwal (scheduler) tugas
// berkala dengan kunci singleton berbasis PostgreSQL advisory lock, sehingga
// aman dijalankan pada lebih dari satu instance (hanya satu yang mengeksekusi).
package worker

import (
	"context"
	"hash/fnv"

	"github.com/jmoiron/sqlx"
)

// Locker adalah kontrak kunci singleton per nama tugas. Implementasi nyata
// memakai pg_try_advisory_lock; test memakai fake.
type Locker interface {
	// TryAcquire mencoba mengambil kunci bernama. Bila ok=true, pemanggil WAJIB
	// memanggil release() setelah tugas selesai (idempotent).
	TryAcquire(ctx context.Context, name string) (release func(), ok bool, err error)
}

// AdvisoryLocker mengimplementasikan Locker dengan PostgreSQL advisory lock.
// Kunci bersesi terikat pada satu koneksi, maka koneksi ditahan (tidak kembali
// ke pool) selama kunci dipegang agar pg_advisory_unlock berjalan di koneksi
// yang sama.
type AdvisoryLocker struct {
	db *sqlx.DB
}

// NewAdvisoryLocker membangun locker di atas pool sqlx.
func NewAdvisoryLocker(db *sqlx.DB) *AdvisoryLocker {
	return &AdvisoryLocker{db: db}
}

// lockKey menurunkan kunci int64 stabil dari nama tugas (FNV-1a 64-bit).
func lockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("kipan-worker:" + name))
	return int64(h.Sum64())
}

// TryAcquire mengambil kunci advisory untuk nama tugas. Bila DB tidak
// tersedia, mengembalikan ok=true tanpa lock agar scheduler tetap jalan di
// lingkungan tanpa PostgreSQL (tidak fail).
func (l *AdvisoryLocker) TryAcquire(ctx context.Context, name string) (func(), bool, error) {
	if l == nil || l.db == nil {
		return func() {}, true, nil
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	key := lockKey(name)

	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !acquired {
		_ = conn.Close()
		return func() {}, false, nil
	}

	release := func() {
		// Pakai ctx terpisah agar unlock tetap berjalan saat ctx tugas dibatalkan.
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, key)
		_ = conn.Close()
	}
	return release, true, nil
}
