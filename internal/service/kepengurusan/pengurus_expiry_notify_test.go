package kepengurusan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// fakeExpiryPengurusRepo menyediakan baris kedaluwarsa/peringatan canned.
type fakeExpiryPengurusRepo struct {
	repository.PengurusRepository
	expiring []domain.ExpiringAppointment
	closed   []domain.ExpiredAppointment
	marked   map[int]int
}

func (f *fakeExpiryPengurusRepo) ListExpiringSoon(context.Context, int) ([]domain.ExpiringAppointment, error) {
	return f.expiring, nil
}

func (f *fakeExpiryPengurusRepo) MarkExpiryNotified(_ context.Context, id, milestone int) error {
	if f.marked == nil {
		f.marked = map[int]int{}
	}
	f.marked[id] = milestone
	return nil
}

func (f *fakeExpiryPengurusRepo) CloseExpiredAppointments(context.Context) ([]domain.ExpiredAppointment, error) {
	return f.closed, nil
}

type notifCall struct {
	title, message, link string
	typ                  domain.NotificationType
	prov, kab            int
	userID               string
}

// fakeExpiryNotifRepo merekam notifikasi keluar.
type fakeExpiryNotifRepo struct {
	repository.NotificationRepository
	admins []notifCall
	users  []notifCall
}

func (f *fakeExpiryNotifRepo) NotifyAdmins(_ context.Context, title, message string, typ domain.NotificationType, link string, prov, kab int) error {
	f.admins = append(f.admins, notifCall{title: title, message: message, typ: typ, link: link, prov: prov, kab: kab})
	return nil
}

func (f *fakeExpiryNotifRepo) NotifyUser(_ context.Context, userID, title, message string, typ domain.NotificationType, link string) error {
	f.users = append(f.users, notifCall{userID: userID, title: title, message: message, typ: typ, link: link})
	return nil
}

func expiringRow(id int, daysLeft int, h30, h7 bool) domain.ExpiringAppointment {
	prov, kab := 32, 3273
	r := domain.ExpiringAppointment{
		ID: 1, AnggotaID: 7, NamaLengkap: "Budi", Jabatan: "Ketua",
		NomorSK: "001/SK/2026", TanggalBerakhir: time.Now().AddDate(0, 0, daysLeft),
		Level: "KABUPATEN", ProvinsiID: &prov, KabupatenID: &kab,
	}
	r.ID = id
	if h30 {
		t := time.Now()
		r.NotifiedH30At = &t
	}
	if h7 {
		t := time.Now()
		r.NotifiedH7At = &t
	}
	return r
}

// TestWarnExpiringSoonMilestoneDanDedup mengunci: H-29 → peringatan 30;
// H-6 → peringatan 7; yang sudah ditandai / di luar jendela dilewati;
// isi pesan memuat nomor SK.
func TestWarnExpiringSoonMilestoneDanDedup(t *testing.T) {
	pgr := &fakeExpiryPengurusRepo{expiring: []domain.ExpiringAppointment{
		expiringRow(1, 29, false, false),
		expiringRow(2, 6, false, false),
		expiringRow(3, 5, false, true),
		expiringRow(4, 40, false, false),
	}}
	notif := &fakeExpiryNotifRepo{}
	svc := NewPengurusExpiryService(pgr, nil, notif)

	n, err := svc.WarnExpiringSoon(context.Background())
	if err != nil {
		t.Fatalf("WarnExpiringSoon gagal: %v", err)
	}
	if n != 2 {
		t.Fatalf("harap 2 peringatan, dapat %d", n)
	}
	if pgr.marked[1] != 30 || pgr.marked[2] != 7 {
		t.Fatalf("flag salah: %+v", pgr.marked)
	}
	if _, ok := pgr.marked[3]; ok {
		t.Fatal("baris yang sudah ditandai tidak boleh ditandai ulang")
	}
	if len(notif.admins) != 2 {
		t.Fatalf("harap 2 notifikasi admin, dapat %d", len(notif.admins))
	}
	for _, c := range notif.admins {
		if !strings.Contains(c.message, "001/SK/2026") {
			t.Fatalf("pesan harus memuat nomor SK: %q", c.message)
		}
		if c.prov != 32 || c.kab != 3273 {
			t.Fatalf("scope salah: %+v", c)
		}
		if c.typ != domain.NotifTypeSK {
			t.Fatalf("tipe = %q, harap SK", c.typ)
		}
	}
}

// TestRunOnceNotifikasiDemosi mengunci: demosi memicu notifikasi admin
// sewilayah + anggota yang punya akun saja.
func TestRunOnceNotifikasiDemosi(t *testing.T) {
	prov, kab := 32, 3273
	uid := "user-1"
	pgr := &fakeExpiryPengurusRepo{closed: []domain.ExpiredAppointment{
		{ID: 5, AnggotaID: 7, NomorSK: "001/SK/2026", Jabatan: "Ketua",
			NamaLengkap: "Budi", Level: "KABUPATEN",
			ProvinsiID: &prov, KabupatenID: &kab, UserID: &uid},
		{ID: 6, AnggotaID: 8, NomorSK: "002/SK/2026", Jabatan: "Sekretaris",
			NamaLengkap: "Ani", Level: "KABUPATEN",
			ProvinsiID: &prov, KabupatenID: &kab},
	}}
	notif := &fakeExpiryNotifRepo{}
	svc := NewPengurusExpiryService(pgr, nil, notif)

	n, err := svc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce gagal: %v", err)
	}
	if n != 2 {
		t.Fatalf("harap 2 ditutup, dapat %d", n)
	}
	if len(notif.admins) != 2 {
		t.Fatalf("harap 2 notifikasi admin, dapat %d", len(notif.admins))
	}
	if len(notif.users) != 1 || notif.users[0].userID != "user-1" {
		t.Fatalf("harap 1 notifikasi anggota berakun: %+v", notif.users)
	}
}
