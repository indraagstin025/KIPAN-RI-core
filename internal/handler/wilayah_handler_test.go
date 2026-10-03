package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

type fakeWilayahService struct {
	items []domain.WilayahKecamatan
	desa  []domain.WilayahDesa
	pos   []domain.WilayahKodepos
	err   error
}

func (f *fakeWilayahService) ListProvinsi(context.Context) ([]domain.WilayahProvinsi, error) {
	return nil, nil
}
func (f *fakeWilayahService) ListKabupaten(context.Context, int) ([]domain.WilayahKabupaten, error) {
	return nil, nil
}
func (f *fakeWilayahService) ListKecamatan(context.Context, string) ([]domain.WilayahKecamatan, error) {
	return f.items, f.err
}
func (f *fakeWilayahService) ListDesa(context.Context, string) ([]domain.WilayahDesa, error) {
	return f.desa, f.err
}
func (f *fakeWilayahService) ListKodepos(context.Context, string, string, string) ([]domain.WilayahKodepos, error) {
	return f.pos, f.err
}

func TestWilayahKecamatanTolakKodeInvalid(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{})
	app.Get("/kecamatan", h.ListKecamatan)

	for _, q := range []string{"", "32", "327", "32733", "32.73", "abcd", "../../etc/passwd", "https://evil.test/x"} {
		req := httptest.NewRequest(http.MethodGet, "/kecamatan?kabupaten_kode="+q, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("kode %q harus 400, dapat %d", q, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWilayahKecamatanSukses(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{
		items: []domain.WilayahKecamatan{{Kode: "327301", Nama: "Sukasari"}},
	})
	app.Get("/kecamatan", h.ListKecamatan)

	req := httptest.NewRequest(http.MethodGet, "/kecamatan?kabupaten_kode=3273", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("harap 200, dapat %d", resp.StatusCode)
	}
	var body struct {
		Data []domain.WilayahKecamatan `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].Kode != "327301" {
		t.Fatalf("data salah: %+v", body.Data)
	}
}

func TestWilayahDesaTolakKodeInvalid(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{})
	app.Get("/desa", h.ListDesa)

	for _, q := range []string{"", "3273", "3273081", "32.73.08", "abcdef", "../../etc/passwd"} {
		req := httptest.NewRequest(http.MethodGet, "/desa?kecamatan_kode="+q, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("kode %q harus 400, dapat %d", q, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWilayahDesaSukses(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{
		desa: []domain.WilayahDesa{{Kode: "3273081001", Nama: "Hegarmanah"}},
	})
	app.Get("/desa", h.ListDesa)

	req := httptest.NewRequest(http.MethodGet, "/desa?kecamatan_kode=327308", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("harap 200, dapat %d", resp.StatusCode)
	}
	var body struct {
		Data []domain.WilayahDesa `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].Kode != "3273081001" {
		t.Fatalf("data salah: %+v", body.Data)
	}
}

func TestWilayahKodeposTolakKosong(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{})
	app.Get("/kodepos", h.ListKodepos)

	for _, q := range []string{"", "desa=&kecamatan=", "desa=<x>&kecamatan=y"} {
		req := httptest.NewRequest(http.MethodGet, "/kodepos?"+q, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("q=%q harus 400, dapat %d", q, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWilayahKodeposSukses(t *testing.T) {
	app := fiber.New()
	h := NewWilayahHandler(&fakeWilayahService{
		pos: []domain.WilayahKodepos{{KodePos: "40559"}},
	})
	app.Get("/kodepos", h.ListKodepos)

	req := httptest.NewRequest(http.MethodGet, "/kodepos?desa=Karyawangi&kecamatan=Parongpong", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("harap 200, dapat %d", resp.StatusCode)
	}
	var body struct {
		Data []domain.WilayahKodepos `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].KodePos != "40559" {
		t.Fatalf("data salah: %+v", body.Data)
	}
}
