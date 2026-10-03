package repository

// Integration test repository pendaftaran terhadap PostgreSQL asli.
//
// Dijalankan hanya bila TEST_DATABASE_URL diset (dev/CI dengan DB):
//   $env:TEST_DATABASE_URL="postgres://postgres@127.0.0.1:5432/kipan_core?sslmode=disable"
//   go test ./internal/repository/ -race -count=1 -v
//
// Tanpa env tersebut seluruh test SKIP (unit gate tetap hijau).
// Data uji memakai NIK acak + email @example.com dan dibersihkan defer.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL kosong — integration test dilewati")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("koneksi test DB gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testWilayah(t *testing.T, ctx context.Context, db *sqlx.DB) (provID, kabID int) {
	t.Helper()
	if err := db.GetContext(ctx, &provID,
		`SELECT id FROM wilayah_provinsi WHERE is_active = TRUE ORDER BY id LIMIT 1`); err != nil {
		t.Fatalf("tidak ada provinsi aktif: %v", err)
	}
	if err := db.GetContext(ctx, &kabID,
		`SELECT id FROM wilayah_kabupaten WHERE provinsi_id = $1 AND is_active = TRUE ORDER BY id LIMIT 1`, provID); err != nil {
		t.Fatalf("tidak ada kabupaten aktif: %v", err)
	}
	return provID, kabID
}

func uniqueNIK() string {
	return fmt.Sprintf("32010101%08d", time.Now().UnixNano()%100000000)
}

func testEntity(nomor, nikHash string, provID, kabID int) *domain.Pendaftaran {
	return &domain.Pendaftaran{
		NomorPendaftaran:     nomor,
		NamaLengkap:          "Uji Integrasi",
		NIKHash:              nikHash,
		NIKEncrypted:         "enc:stub",
		TempatLahir:          "Bandung",
		TanggalLahir:         time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC),
		JenisKelamin:         "L",
		Alamat:               "Jl. Merdeka No. 10, Bandung Kota",
		ProvinsiID:           provID,
		KabupatenID:          kabID,
		Email:                fmt.Sprintf("integ-%d@example.com", time.Now().UnixNano()%1000000000),
		Whatsapp:             "081234567890",
		FotoKey:              "uploads/pendaftaran/foto.jpg",
		KTPKey:               "uploads/pendaftaran/ktp.jpg",
		Motivasi:             "Motivasi uji integrasi",
		PersyaratanChecklist: "[]",
		Tipe:                 domain.TipePendaftaranKader,
		Status:               domain.PendaftaranStatusDraft,
	}
}

func cleanupPendaftaran(t *testing.T, ctx context.Context, db *sqlx.DB, ids ...int) {
	t.Helper()
	for _, id := range ids {
		db.ExecContext(ctx, `DELETE FROM pendaftaran_riwayat WHERE pendaftaran_id = $1`, id)
		db.ExecContext(ctx, `DELETE FROM pendaftaran WHERE id = $1`, id)
	}
}

func TestCreateWithHistoryAndDuplicateNIK(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	provID, kabID := testWilayah(t, ctx, db)
	repo := NewPendaftaranRepository(db)

	nikHash := "integ-hash-" + uniqueNIK()
	p1 := testEntity("REG-TEST-00001", nikHash, provID, kabID)
	if err := repo.CreateWithHistory(ctx, p1, "SUBMIT", "uji"); err != nil {
		t.Fatalf("CreateWithHistory gagal: %v", err)
	}
	if p1.ID == 0 {
		t.Fatal("ID tidak terisi dari RETURNING")
	}
	t.Cleanup(func() { cleanupPendaftaran(t, ctx, db, p1.ID) })

	got, err := repo.GetByNikHash(ctx, nikHash)
	if err != nil || got.ID != p1.ID {
		t.Fatalf("GetByNikHash gagal: %v", err)
	}

	// Duplikat NIK → 409, bukan 500.
	p2 := testEntity("REG-TEST-00002", nikHash, provID, kabID)
	err = repo.CreateWithHistory(ctx, p2, "SUBMIT", "uji")
	if err == nil {
		cleanupPendaftaran(t, ctx, db, p2.ID)
		t.Fatal("duplikat NIK DITERIMA")
	}
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != 409 {
		t.Fatalf("duplikat harus 409, dapat: %v", err)
	}
}

func TestNextRegistrationSequenceIncrements(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewPendaftaranRepository(db)

	s1, err := repo.NextRegistrationSequence(ctx, 2099, 1)
	if err != nil {
		t.Fatalf("sequence gagal: %v", err)
	}
	s2, err := repo.NextRegistrationSequence(ctx, 2099, 1)
	if err != nil {
		t.Fatalf("sequence gagal: %v", err)
	}
	if s2 != s1+1 {
		t.Fatalf("sequence tidak naik: %d -> %d", s1, s2)
	}
}

func TestNextRegistrationSequenceConcurrentUnique(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewPendaftaranRepository(db)

	const n = 10
	results := make([]int, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.NextRegistrationSequence(ctx, 2099, 2)
		}(i)
	}
	wg.Wait()
	seen := map[int]bool{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d gagal: %v", i, errs[i])
		}
		if seen[results[i]] {
			t.Fatalf("sequence duplikat: %d", results[i])
		}
		seen[results[i]] = true
	}
}

func TestListQueueFiltersWilayah(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	provID, kabID := testWilayah(t, ctx, db)
	repo := NewPendaftaranRepository(db)

	p := testEntity("REG-TEST-00003", "integ-hash-"+uniqueNIK(), provID, kabID)
	if err := repo.CreateWithHistory(ctx, p, "SUBMIT", "uji"); err != nil {
		t.Fatalf("CreateWithHistory gagal: %v", err)
	}
	t.Cleanup(func() { cleanupPendaftaran(t, ctx, db, p.ID) })

	items, err := repo.ListQueue(ctx, &provID, &kabID, "", 25, 0)
	if err != nil {
		t.Fatalf("ListQueue gagal: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ProvinsiID != provID || it.KabupatenID != kabID {
			t.Fatalf("filter bocor: %+v", it)
		}
		if it.ID == p.ID {
			found = true
		}
		// Proyeksi queue tidak boleh memuat kontak/key.
	}
	if !found {
		t.Fatal("baris uji tidak muncul di antrean wilayahnya")
	}
	total, err := repo.CountQueue(ctx, &provID, &kabID, "")
	if err != nil || total < 1 {
		t.Fatalf("CountQueue gagal: total=%d err=%v", total, err)
	}

	// Wilayah fiktif → kosong.
	negProv, negKab := -1, -1
	empty, err := repo.ListQueue(ctx, &negProv, &negKab, "", 25, 0)
	if err != nil || len(empty) != 0 {
		t.Fatalf("filter fiktif harus kosong: %v %+v", err, empty)
	}
}

func TestRevisionTokenFlow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	provID, kabID := testWilayah(t, ctx, db)
	repo := NewPendaftaranRepository(db)

	p := testEntity("REG-TEST-00004", "integ-hash-"+uniqueNIK(), provID, kabID)
	if err := repo.CreateWithHistory(ctx, p, "SUBMIT", "uji"); err != nil {
		t.Fatalf("CreateWithHistory gagal: %v", err)
	}
	t.Cleanup(func() { cleanupPendaftaran(t, ctx, db, p.ID) })

	if err := repo.UpdateStatus(ctx, p.ID, domain.PendaftaranStatusPerbaikan, "buram"); err != nil {
		t.Fatalf("UpdateStatus gagal: %v", err)
	}
	if err := repo.SetRevisiToken(ctx, p.ID, "hash-token-benar", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SetRevisiToken gagal: %v", err)
	}

	keys := map[string]string{
		"foto_key": "uploads/pendaftaran/foto2.jpg", "ktp_key": "uploads/pendaftaran/ktp2.jpg",
		"cv_key": "", "sk_key": "", "surat_pernyataan_key": "", "surat_sehat_key": "",
	}
	// Token salah → error generik.
	if err := repo.SubmitRevisionTx(ctx, p.ID, "hash-token-salah", keys, "x"); err == nil {
		t.Fatal("token salah DITERIMA")
	}
	// Token benar → sukses, status DRAFT, token hangus.
	if err := repo.SubmitRevisionTx(ctx, p.ID, "hash-token-benar", keys, "revisi"); err != nil {
		t.Fatalf("SubmitRevisionTx gagal: %v", err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.PendaftaranStatusDraft {
		t.Fatalf("status harus DRAFT: %+v %v", got, err)
	}
	if got.RevisiTokenHash != nil {
		t.Fatal("token tidak hangus setelah dipakai")
	}
	// Pakai ulang → error.
	if err := repo.SubmitRevisionTx(ctx, p.ID, "hash-token-benar", keys, "x"); err == nil {
		t.Fatal("token pakai-ulang DITERIMA")
	}
}
