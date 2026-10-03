package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestPengurusSingleActiveIndex memverifikasi indeks unik parsial
// uq_pengurus_single_active (TDD D10): keberadaan + perilaku menolak baris
// 'Aktif' kedua untuk anggota yang sama lintas SK.
func TestPengurusSingleActiveIndex(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	// 1) Index ada dengan predikat status = 'Aktif'.
	var def string
	if err := db.GetContext(ctx, &def,
		`SELECT indexdef FROM pg_indexes WHERE indexname = 'uq_pengurus_single_active'`); err != nil {
		t.Fatalf("index uq_pengurus_single_active tidak ditemukan: %v", err)
	}
	if !strings.Contains(def, "UNIQUE") || !strings.Contains(def, "'Aktif'") {
		t.Fatalf("definisi index tidak sesuai: %s", def)
	}

	// 2) Perilaku: dua baris Aktif untuk anggota sama harus ditolak.
	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("psa-%d", time.Now().UnixNano())

	var jabatanID int
	if err := db.GetContext(ctx, &jabatanID, `SELECT id FROM jabatan ORDER BY id LIMIT 1`); err != nil {
		t.Fatalf("tidak ada master jabatan: %v", err)
	}

	var anggotaID int
	if err := db.GetContext(ctx, &anggotaID, `
		INSERT INTO anggota (nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
			jenis_kelamin, alamat, provinsi_id, kabupaten_id, email, whatsapp, angkatan, tanggal_daftar, tanggal_angkat)
		VALUES ($1,$2,$3,'enc','Uji','2000-01-01','L','Alamat uji',$4,$5,$6,'0800000000','2026',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
		RETURNING id`,
		"KIPAN-IND-"+marker, "Anggota Uji "+marker, fmt.Sprintf("%-64s", marker), provID, kabID, marker+"@example.com"); err != nil {
		t.Fatalf("insert anggota gagal: %v", err)
	}

	mkSK := func(suffix string) int {
		var id int
		if err := db.GetContext(ctx, &id, `
			INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id,
				tanggal_terbit, tanggal_berakhir, file_sk_key)
			VALUES ($1,'Uji','KABUPATEN',$2,$3,CURRENT_DATE,CURRENT_DATE + INTERVAL '1 year','uploads/sk/uji.pdf')
			RETURNING id`, marker+"-"+suffix, provID, kabID); err != nil {
			t.Fatalf("insert SK gagal: %v", err)
		}
		return id
	}
	sk1, sk2 := mkSK("1"), mkSK("2")

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pengurus WHERE anggota_id = $1`, anggotaID)
		db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE nomor_sk LIKE $1`, marker+"%")
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id = $1`, anggotaID)
	})

	ins := func(skID int) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO pengurus (anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
			VALUES ($1,$2,'KABUPATEN',$3,$4,$5,'Aktif',CURRENT_DATE)`,
			anggotaID, skID, provID, kabID, jabatanID)
		return err
	}

	if err := ins(sk1); err != nil {
		t.Fatalf("insert Aktif pertama harus sukses: %v", err)
	}
	err := ins(sk2)
	if err == nil {
		t.Fatal("insert Aktif kedua untuk anggota sama harus ditolak")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("harus unique_violation (23505), dapat: %v", err)
	}
}
