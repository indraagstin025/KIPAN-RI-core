package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestLatestPendaftaranAksiByAnggotaIDs memverifikasi fallback kolom RIWAYAT:
// aksi pendaftaran TERAKHIR per anggota; anggota tanpa pendaftaran absen.
func TestLatestPendaftaranAksiByAnggotaIDs(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewAnggotaRepository(db)

	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("lpa-%d", time.Now().UnixNano())

	var regID int
	if err := db.GetContext(ctx, &regID, `
		INSERT INTO pendaftaran (nomor_pendaftaran, nama_lengkap, nik_hash, nik_encrypted,
			tempat_lahir, tanggal_lahir, jenis_kelamin, alamat, provinsi_id, kabupaten_id,
			email, whatsapp, status)
		VALUES ($1,'Kader Uji',$5,'enc','Uji','2000-01-01','L','Alamat uji',$2,$3,$4,'0800000000','DISETUJUI')
		RETURNING id`,
		"REG-"+marker, provID, kabID, marker+"@example.com", fmt.Sprintf("%-64s", marker)); err != nil {
		t.Fatalf("insert pendaftaran gagal: %v", err)
	}
	for _, aksi := range []string{"SUBMIT", "VERIFIKASI", "SETUJUI"} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO pendaftaran_riwayat (pendaftaran_id, aksi) VALUES ($1, $2)`, regID, aksi); err != nil {
			t.Fatalf("insert riwayat %s gagal: %v", aksi, err)
		}
	}

	var memberID int
	if err := db.GetContext(ctx, &memberID, `
		INSERT INTO anggota (nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
			jenis_kelamin, alamat, provinsi_id, kabupaten_id, email, whatsapp, angkatan,
			tanggal_daftar, tanggal_angkat, pendaftaran_id)
		VALUES ($1,'Kader Uji',$5,'enc','Uji','2000-01-01','L','Alamat uji',$2,$3,$4,'0800000000','2026',
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $6)
		RETURNING id`,
		"KIPAN-IND-"+marker, provID, kabID, marker+"@example.com", fmt.Sprintf("%-64s", "m-"+marker), regID); err != nil {
		t.Fatalf("insert anggota gagal: %v", err)
	}
	var loneID int
	if err := db.GetContext(ctx, &loneID, `
		INSERT INTO anggota (nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
			jenis_kelamin, alamat, provinsi_id, kabupaten_id, email, whatsapp, angkatan,
			tanggal_daftar, tanggal_angkat)
		VALUES ($1,'Kader Lone',$5,'enc','Uji','2000-01-01','L','Alamat uji',$2,$3,$4,'0800000000','2026',
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`,
		"KIPAN-IND-LONE-"+marker, provID, kabID, "lone-"+marker+"@example.com", fmt.Sprintf("%-64s", "lone-"+marker)); err != nil {
		t.Fatalf("insert anggota lone gagal: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pendaftaran_riwayat WHERE pendaftaran_id = $1`, regID)
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id IN ($1, $2)`, memberID, loneID)
		db.ExecContext(ctx, `DELETE FROM pendaftaran WHERE id = $1`, regID)
	})

	got, err := repo.LatestPendaftaranAksiByAnggotaIDs(ctx, []int{memberID, loneID})
	if err != nil {
		t.Fatalf("LatestPendaftaranAksiByAnggotaIDs gagal: %v", err)
	}
	if got[memberID] != "SETUJUI" {
		t.Fatalf("aksi terakhir = %q, harap SETUJUI", got[memberID])
	}
	if _, ok := got[loneID]; ok {
		t.Fatal("anggota tanpa pendaftaran seharusnya absen dari map")
	}
	if len(got) != 1 {
		t.Fatalf("map harus berisi 1 entri, dapat %d", len(got))
	}
}
