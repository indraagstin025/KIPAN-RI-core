package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestCloseExpiredAppointments memverifikasi materialisasi kedaluwarsa (TDD §5.4):
// pengurus Aktif dengan masa bakti SK lewat ditutup otomatis; status efektif
// (view) sudah Demisioner sebelum job; idempoten setelah dijalankan.
func TestCloseExpiredAppointments(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewPengurusRepository(db)

	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("exp-%d", time.Now().UnixNano())

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
		"KIPAN-IND-"+marker, "Uji Kedaluwarsa "+marker, fmt.Sprintf("%-64s", marker), provID, kabID, marker+"@example.com"); err != nil {
		t.Fatalf("insert anggota gagal: %v", err)
	}

	var skID int
	if err := db.GetContext(ctx, &skID, `
		INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id,
			tanggal_terbit, tanggal_berakhir, file_sk_key, status, approval_status)
		VALUES ($1,'Uji Kedaluwarsa','KABUPATEN',$2,$3, CURRENT_DATE - INTERVAL '400 days',
			CURRENT_DATE - INTERVAL '1 day','uploads/sk/uji.pdf','Aktif','DISETUJUI')
		RETURNING id`, marker, provID, kabID); err != nil {
		t.Fatalf("insert SK gagal: %v", err)
	}

	var pengurusID int
	if err := db.GetContext(ctx, &pengurusID, `
		INSERT INTO pengurus (anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
		VALUES ($1,$2,'KABUPATEN',$3,$4,$5,'Aktif', CURRENT_DATE - INTERVAL '400 days')
		RETURNING id`, anggotaID, skID, provID, kabID, jabatanID); err != nil {
		t.Fatalf("insert pengurus gagal: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pengurus WHERE id = $1`, pengurusID)
		db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, skID)
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id = $1`, anggotaID)
	})

	// Sebelum job: status efektif (view, baca live) sudah Demisioner.
	var efektif string
	if err := db.GetContext(ctx, &efektif,
		`SELECT status_efektif FROM v_pengurus_efektif WHERE id = $1`, pengurusID); err != nil {
		t.Fatalf("query view gagal: %v", err)
	}
	if efektif != "Demisioner" {
		t.Fatalf("status efektif harus Demisioner, dapat %q", efektif)
	}

	// Jalankan materialisasi.
	closed, err := repo.CloseExpiredAppointments(ctx)
	if err != nil {
		t.Fatalf("CloseExpiredAppointments gagal: %v", err)
	}
	found := false
	for _, c := range closed {
		if c.ID == pengurusID {
			found = true
			if c.NomorSK != marker {
				t.Fatalf("nomor_sk tidak sesuai: %q", c.NomorSK)
			}
		}
	}
	if !found {
		t.Fatal("pengurus kedaluwarsa tidak ikut ditutup")
	}

	// Status tersimpan menjadi Demisioner.
	var stored string
	if err := db.GetContext(ctx, &stored, `SELECT status FROM pengurus WHERE id = $1`, pengurusID); err != nil {
		t.Fatalf("query status gagal: %v", err)
	}
	if stored != "Demisioner" {
		t.Fatalf("status tersimpan harus Demisioner, dapat %q", stored)
	}

	// Idempoten: run kedua tidak lagi memuat baris ini.
	closed2, err := repo.CloseExpiredAppointments(ctx)
	if err != nil {
		t.Fatalf("run kedua gagal: %v", err)
	}
	for _, c := range closed2 {
		if c.ID == pengurusID {
			t.Fatal("run kedua seharusnya tidak menutup ulang baris ini")
		}
	}
}

// TestListExpiringSoonDanMarkNotified memverifikasi sumber notifikasi H-30/H-7:
// hanya pengurus Aktif dengan SK Aktif yang berakhir dalam maxDays hari;
// penanda anti-duplikat per milestone.
func TestListExpiringSoonDanMarkNotified(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewPengurusRepository(db)

	provID, kabID := testWilayah(t, ctx, db)
	marker := fmt.Sprintf("expw-%d", time.Now().UnixNano())

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
		"KIPAN-IND-"+marker, "Uji Peringatan "+marker, fmt.Sprintf("%-64s", marker), provID, kabID, marker+"@example.com"); err != nil {
		t.Fatalf("insert anggota gagal: %v", err)
	}

	var skID int
	if err := db.GetContext(ctx, &skID, `
		INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id,
			tanggal_terbit, tanggal_berakhir, file_sk_key, status, approval_status)
		VALUES ($1,'Uji Peringatan','KABUPATEN',$2,$3, CURRENT_DATE - INTERVAL '300 days',
			CURRENT_DATE + INTERVAL '20 days','uploads/sk/uji.pdf','Aktif','DISETUJUI')
		RETURNING id`, marker, provID, kabID); err != nil {
		t.Fatalf("insert SK gagal: %v", err)
	}

	var pengurusID int
	if err := db.GetContext(ctx, &pengurusID, `
		INSERT INTO pengurus (anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
		VALUES ($1,$2,'KABUPATEN',$3,$4,$5,'Aktif', CURRENT_DATE - INTERVAL '300 days')
		RETURNING id`, anggotaID, skID, provID, kabID, jabatanID); err != nil {
		t.Fatalf("insert pengurus gagal: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM pengurus WHERE id = $1`, pengurusID)
		db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, skID)
		db.ExecContext(ctx, `DELETE FROM anggota WHERE id = $1`, anggotaID)
	})

	// H+20: masuk jendela 30 hari, di luar jendela 5 hari.
	rows, err := repo.ListExpiringSoon(ctx, 30)
	if err != nil {
		t.Fatalf("ListExpiringSoon gagal: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.ID == pengurusID {
			found = true
			if r.NomorSK != marker || r.TanggalBerakhir.IsZero() {
				t.Fatalf("kolom tidak lengkap: %+v", r)
			}
			if r.NotifiedH30At != nil || r.NotifiedH7At != nil {
				t.Fatal("flag notifikasi harus NULL di awal")
			}
		}
	}
	if !found {
		t.Fatal("pengurus H+20 harus masuk jendela 30 hari")
	}
	rows5, err := repo.ListExpiringSoon(ctx, 5)
	if err != nil {
		t.Fatalf("ListExpiringSoon(5) gagal: %v", err)
	}
	for _, r := range rows5 {
		if r.ID == pengurusID {
			t.Fatal("pengurus H+20 tidak boleh masuk jendela 5 hari")
		}
	}

	// Tandai H-30; milestone tak dikenal ditolak.
	if err := repo.MarkExpiryNotified(ctx, pengurusID, 30); err != nil {
		t.Fatalf("MarkExpiryNotified(30) gagal: %v", err)
	}
	if err := repo.MarkExpiryNotified(ctx, pengurusID, 99); err == nil {
		t.Fatal("milestone 99 harus ditolak")
	}
	var flagged bool
	if err := db.GetContext(ctx, &flagged, `SELECT notified_h30_at IS NOT NULL FROM pengurus WHERE id = $1`, pengurusID); err != nil {
		t.Fatalf("query flag gagal: %v", err)
	}
	if !flagged {
		t.Fatal("flag notified_h30_at harus terisi")
	}
}
