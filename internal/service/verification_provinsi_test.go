package service

import (
	"context"
	"strings"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// Admin Provinsi boleh MELIHAT antrean provinsinya, tetapi TIDAK BOLEH aksi
// verifikasi (verifikasi/perbaikan/tolak/setujui) — wewenang eksklusif
// Kab/Kota di wilayahnya (selaras proyek lama + URD Opsi A).
func TestProvinsiDitolakSemuaAksiVerifikasi(t *testing.T) {
	for _, action := range []domain.PendaftaranApprovalAction{
		domain.PendaftaranActionVerifikasi,
		domain.PendaftaranActionPerbaikan,
		domain.PendaftaranActionTolak,
		domain.PendaftaranActionSetujui,
	} {
		item, member := approveFixture()
		repo := &fakeApproveRepo{item: item, member: member}
		svc := approveSvc(repo, &fakeMemberUserRepo{}, &fakeAnggotaRepo{}, &fakeOutboxRepo{})
		prov := 32
		actor := domain.ActorContext{
			UserID: "admin-prov", Name: "Admin Prov",
			Role:       domain.RoleAdminProvinsi,
			ProvinsiID: &prov,
		}
		_, err := svc.ProcessApproval(context.Background(), item.ID, action, "catatan", actor, domain.AuditContext{})
		if err == nil {
			t.Fatalf("aksi %q oleh ADMIN_PROVINSI DITERIMA", action)
		}
		if !strings.Contains(err.Error(), "Kabupaten/Kota") {
			t.Fatalf("aksi %q: pesan harus menyebut wewenang Kab/Kota, dapat %q", action, err.Error())
		}
	}
}
