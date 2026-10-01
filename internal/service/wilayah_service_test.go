package service

import (
	"context"
	"testing"
)

func TestWilayahNilRepoUnavailable(t *testing.T) {
	svc := NewWilayahService(nil)
	if _, err := svc.ListProvinsi(context.Background()); err == nil {
		t.Fatal("harap 503 tanpa repo")
	}
	if _, err := svc.ListKabupaten(context.Background(), 32); err == nil {
		t.Fatal("harap 503 tanpa repo")
	}
}

func TestWilayahInvalidProvinsi(t *testing.T) {
	svc := NewWilayahService(nil)
	for _, id := range []int{0, -1} {
		if _, err := svc.ListKabupaten(context.Background(), id); err == nil {
			t.Fatalf("provinsi %d harus ditolak", id)
		}
	}
}
