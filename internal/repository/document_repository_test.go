package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// Integration (gated TEST_DATABASE_URL): berkas SK harus dapat dipetakan ke
// pemiliknya agar presign-view tidak 404 (SEC-STORE-BOLA).
func TestResolveOwnerSuratKeputusan(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	provID, kabID := testWilayah(t, ctx, db)
	repo := NewDocumentRepository(db)

	key := fmt.Sprintf("uploads/pendaftaran/%d-sk.pdf", time.Now().UnixNano())
	nomor := fmt.Sprintf("SK-UJI-%d", time.Now().UnixNano())
	var id int
	if err := db.GetContext(ctx, &id,
		`INSERT INTO surat_keputusan (nomor_sk, judul, level, provinsi_id, kabupaten_id, tanggal_terbit, file_sk_key, status, approval_status)
		 VALUES ($1, 'Uji', 'KABUPATEN', $2, $3, CURRENT_DATE, $4, 'Aktif', 'DRAFT')
		 RETURNING id`, nomor, provID, kabID, key); err != nil {
		t.Fatalf("insert SK gagal: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, id) })

	owner, err := repo.ResolveOwner(ctx, key)
	if err != nil {
		t.Fatalf("ResolveOwner gagal: %v", err)
	}
	if owner.Entity != "surat_keputusan" || owner.EntityID != strconv.Itoa(id) {
		t.Fatalf("owner salah: %+v", owner)
	}
	if owner.ProvinsiID != provID || owner.KabupatenID != kabID {
		t.Fatalf("wilayah owner salah: %+v", owner)
	}

	// SK NASIONAL: wilayah NULL → COALESCE 0.
	keyNas := fmt.Sprintf("uploads/pendaftaran/%d-sk-nas.pdf", time.Now().UnixNano())
	nomorNas := fmt.Sprintf("SK-UJI-NAS-%d", time.Now().UnixNano())
	var idNas int
	if err := db.GetContext(ctx, &idNas,
		`INSERT INTO surat_keputusan (nomor_sk, judul, level, tanggal_terbit, file_sk_key, status, approval_status)
		 VALUES ($1, 'Uji Nas', 'NASIONAL', CURRENT_DATE, $2, 'Aktif', 'DRAFT')
		 RETURNING id`, nomorNas, keyNas); err != nil {
		t.Fatalf("insert SK nasional gagal: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM surat_keputusan WHERE id = $1`, idNas) })

	ownerNas, err := repo.ResolveOwner(ctx, keyNas)
	if err != nil {
		t.Fatalf("ResolveOwner nasional gagal: %v", err)
	}
	if ownerNas.ProvinsiID != 0 || ownerNas.KabupatenID != 0 {
		t.Fatalf("wilayah nasional harus 0/0: %+v", ownerNas)
	}

	// Key tak dikenal → ErrNotFound.
	if _, err := repo.ResolveOwner(ctx, "uploads/pendaftaran/tidak-ada.pdf"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("harap ErrNotFound, dapat %v", err)
	}
}
