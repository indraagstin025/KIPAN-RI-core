package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestExistsInSKIgnoresNonAktif memverifikasi anggota hanya dianggap
// tercantum bila memegang jabatan AKTIF secara efektif (selaras
// CountJabatanInSK): Demisioner tidak memblokir pengangkatan ulang di SK
// yang sama.
func TestExistsInSKIgnoresNonAktif(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewPengurusRepository(db)

	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("exs-%d", time.Now().UnixNano())

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

	var skID int
	if err := db.GetContext(ctx, &skID, `
		INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id,
			tanggal_terbit, tanggal_berakhir, file_sk_key, status)
		VALUES ($1,'Uji','KABUPATEN',$2,$3,CURRENT_DATE,CURRENT_DATE + INTERVAL '1 year','uploads/sk/uji.pdf','Aktif')
		RETURNING id`, marker, provID, kabID); err != nil {
		t.Fatalf("insert SK gagal: %v", err)
	}

	var pengurusID int
	if err := db.GetContext(ctx, &pengurusID, `
		INSERT INTO pengurus (anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
		VALUES ($1,$2,'KABUPATEN',$3,$4,$5,'Demisioner',CURRENT_DATE)
		RETURNING id`, anggotaID, skID, provID, kabID, jabatanID); err != nil {
		t.Fatalf("insert pengurus gagal: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pengurus WHERE id = $1`, pengurusID)
		db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, skID)
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id = $1`, anggotaID)
	})

	if exists, err := repo.ExistsInSK(ctx, skID, anggotaID); err != nil {
		t.Fatalf("ExistsInSK gagal: %v", err)
	} else if exists {
		t.Fatal("anggota Demisioner seharusnya tidak dianggap tercantum")
	}

	if _, err := db.ExecContext(ctx, `UPDATE pengurus SET status='Aktif' WHERE id=$1`, pengurusID); err != nil {
		t.Fatalf("update status gagal: %v", err)
	}
	if exists, err := repo.ExistsInSK(ctx, skID, anggotaID); err != nil {
		t.Fatalf("ExistsInSK gagal: %v", err)
	} else if !exists {
		t.Fatal("anggota Aktif seharusnya dianggap tercantum")
	}
}
