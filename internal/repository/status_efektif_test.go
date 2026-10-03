package repository

import (
	"context"
	"testing"
)

// TestPengurusStatusEfektifFunction menguji fungsi SQL status efektif (TDD §5.4)
// secara langsung dengan literal tanggal (deterministik, tanpa fixture).
func TestPengurusStatusEfektifFunction(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	cases := []struct {
		name     string
		pengurus string
		skStatus string
		skEnd    interface{} // string tanggal atau nil
		want     string
	}{
		{"aktif-dalam-masa", "Aktif", "Aktif", "2030-01-01", "Aktif"},
		{"aktif-terlewat", "Aktif", "Aktif", "2020-01-01", "Demisioner"},
		{"aktif-tepat-berakhir", "Aktif", "Aktif", "2026-10-04", "Aktif"},
		{"aktif-sk-nonaktif", "Aktif", "TidakAktif", "2030-01-01", "Demisioner"},
		{"aktif-tanpa-tanggal-berakhir", "Aktif", "Aktif", nil, "Aktif"},
		{"diberhentikan-dikembalikan", "Diberhentikan", "Aktif", "2030-01-01", "Diberhentikan"},
		{"mengundurkan-diri", "Mengundurkan Diri", "TidakAktif", "2020-01-01", "Mengundurkan Diri"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			err := db.GetContext(ctx, &got,
				`SELECT pengurus_status_efektif($1::text, $2::text, $3::date, $4::date)`,
				tc.pengurus, tc.skStatus, tc.skEnd, "2026-10-04")
			if err != nil {
				t.Fatalf("panggilan fungsi gagal: %v", err)
			}
			if got != tc.want {
				t.Fatalf("status efektif: mau %q, dapat %q", tc.want, got)
			}
		})
	}
}

// TestViewPengurusEfektifQueryable memastikan view valid & mengekspos kolom
// status_efektif (dipakai tampilan/daftar pengurus).
func TestViewPengurusEfektifQueryable(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	rows := []struct {
		StatusEfektif string `db:"status_efektif"`
	}{}
	if err := db.SelectContext(ctx, &rows, `SELECT status_efektif FROM v_pengurus_efektif LIMIT 5`); err != nil {
		t.Fatalf("view v_pengurus_efektif tidak dapat dikueri: %v", err)
	}
}
