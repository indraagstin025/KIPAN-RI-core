package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestRiwayatByAnggotaIDs memverifikasi query kolom RIWAYAT (TDD §5.5):
// anggota tanpa pengurus = "-"; anggota dengan pengurus aktif = "Pengurus Aktif …".
func TestRiwayatByAnggotaIDs(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewAnggotaRepository(db)

	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("riw-%d", time.Now().UnixNano())

	var jabatanID int
	if err := db.GetContext(ctx, &jabatanID, `SELECT id FROM jabatan ORDER BY id LIMIT 1`); err != nil {
		t.Fatalf("tidak ada master jabatan: %v", err)
	}

	insAnggota := func(suffix string) int {
		var id int
		if err := db.GetContext(ctx, &id, `
			INSERT INTO anggota (nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
				jenis_kelamin, alamat, provinsi_id, kabupaten_id, email, whatsapp, angkatan, tanggal_daftar, tanggal_angkat)
			VALUES ($1,$2,$3,'enc','Uji','2000-01-01','L','Alamat uji',$4,$5,$6,'0800000000','2026',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
			RETURNING id`,
			"KIPAN-IND-"+marker+suffix, "Uji Riwayat "+marker+suffix, fmt.Sprintf("%-64s", marker+suffix),
			provID, kabID, marker+suffix+"@example.com"); err != nil {
			t.Fatalf("insert anggota gagal: %v", err)
		}
		return id
	}

	idBiasa := insAnggota("A") // tanpa pengurus
	idAktif := insAnggota("B") // punya pengurus aktif

	var skID int
	if err := db.GetContext(ctx, &skID, `
		INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id,
			tanggal_terbit, tanggal_berakhir, file_sk_key, status, approval_status)
		VALUES ($1,'Uji Riwayat','KABUPATEN',$2,$3,CURRENT_DATE,CURRENT_DATE + INTERVAL '1 year','uploads/sk/uji.pdf','Aktif','DISETUJUI')
		RETURNING id`, marker, provID, kabID); err != nil {
		t.Fatalf("insert SK gagal: %v", err)
	}
	var pengurusID int
	if err := db.GetContext(ctx, &pengurusID, `
		INSERT INTO pengurus (anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
		VALUES ($1,$2,'KABUPATEN',$3,$4,$5,'Aktif',CURRENT_DATE)
		RETURNING id`, idAktif, skID, provID, kabID, jabatanID); err != nil {
		t.Fatalf("insert pengurus gagal: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pengurus WHERE id = $1`, pengurusID)
		db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, skID)
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id IN ($1,$2)`, idBiasa, idAktif)
	})

	got, err := repo.RiwayatByAnggotaIDs(ctx, []int{idBiasa, idAktif})
	if err != nil {
		t.Fatalf("RiwayatByAnggotaIDs gagal: %v", err)
	}
	if got[idBiasa] != "-" {
		t.Fatalf("anggota tanpa pengurus harus '-', dapat %q", got[idBiasa])
	}
	if !strings.HasPrefix(got[idAktif], "Pengurus Aktif ") {
		t.Fatalf("anggota aktif harus 'Pengurus Aktif …', dapat %q", got[idAktif])
	}
}
