package service

// anggota_admin.go — CRUD anggota langsung oleh admin (TDD §6.5): tambah,
// sunting, ubah status (soft), dan ekspor CSV. NIK tetap terenkripsi; NIA
// digenerate server-side. Wewenang: admin dalam yurisdiksi wilayah.

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// CreateAnggota menambah anggota langsung: validasi → NIK enkripsi+blind index
// → cek duplikat (anggota & pendaftaran aktif) → alokasi NIA → simpan → audit.
func (s *anggotaService) CreateAnggota(ctx context.Context, in domain.AnggotaCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error) {
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	name := strings.TrimSpace(in.NamaLengkap)
	if len([]rune(name)) < 3 || len([]rune(name)) > svcutil.MaxNamaLen || containsAngleBracket(name) {
		return nil, domain.NewValidationError("Nama lengkap wajib 3-150 karakter & tanpa karakter < atau >")
	}
	nik := strings.TrimSpace(in.NIK)
	if !svcutil.NipPattern.MatchString(nik) || !isPlausibleNIKDate(nik) {
		return nil, domain.NewValidationError("NIK harus 16 digit dengan segmen tanggal lahir valid")
	}
	dob, err := time.Parse(time.RFC3339, strings.TrimSpace(in.TanggalLahir))
	if err != nil {
		return nil, domain.NewValidationError("Format tanggal lahir tidak valid")
	}
	tempat := strings.TrimSpace(in.TempatLahir)
	if tempat == "" || len([]rune(tempat)) > svcutil.MaxTempatLahirLen || containsAngleBracket(tempat) {
		return nil, domain.NewValidationError("Tempat lahir wajib diisi (maks 100 karakter)")
	}
	jk := strings.ToUpper(strings.TrimSpace(in.JenisKelamin))
	if jk != "L" && jk != "P" {
		return nil, domain.NewValidationError("Jenis kelamin harus L atau P")
	}
	if in.ProvinsiID <= 0 || in.KabupatenID <= 0 {
		return nil, domain.NewValidationError("Wilayah provinsi & kabupaten wajib dipilih")
	}
	if !actor.CanAccessWilayah(in.ProvinsiID, in.KabupatenID) {
		return nil, domain.NewForbiddenError("Di luar wilayah kerja Anda")
	}
	if s.wilayahRepo != nil {
		ok, err := s.wilayahRepo.KabupatenInProvinsi(ctx, in.KabupatenID, in.ProvinsiID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, domain.NewValidationError("Kabupaten/Kota tidak valid untuk provinsi tersebut")
		}
	}
	alamat := strings.TrimSpace(in.Alamat)
	if len([]rune(alamat)) < 5 || len([]rune(alamat)) > svcutil.MaxAlamatLen || containsAngleBracket(alamat) {
		return nil, domain.NewValidationError("Alamat wajib 5-2000 karakter & tanpa karakter < atau >")
	}
	email := strings.TrimSpace(in.Email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, domain.NewValidationError("Format email tidak valid")
	}
	wa := strings.TrimSpace(in.Whatsapp)
	if !svcutil.PhonePattern.MatchString(wa) {
		return nil, domain.NewValidationError("Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}
	status := domain.AnggotaStatus(strings.ToUpper(strings.TrimSpace(in.Status)))
	if status == "" {
		status = domain.AnggotaStatusAktif
	}
	if !status.IsValid() {
		return nil, domain.NewValidationError("Status anggota tidak valid")
	}

	// NIK: blind index (cek duplikat) + enkripsi (simpan).
	bKey, err := blindIndexKey(s.cfg)
	if err != nil {
		return nil, err
	}
	aKey, err := aesKey(s.cfg)
	if err != nil {
		return nil, err
	}
	nikHash, err := crypto.BlindIndex(nik, bKey)
	if err != nil {
		return nil, err
	}
	nikEnc, err := crypto.EncryptAESGCM(nik, aKey)
	if err != nil {
		return nil, err
	}

	exists, err := s.anggotaRepo.ExistsByNikHash(ctx, nikHash)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.NewConflictError("NIK sudah terdaftar sebagai anggota")
	}
	if s.pendaftaranRepo != nil {
		if _, err := s.pendaftaranRepo.GetByNikHash(ctx, nikHash); err == nil {
			return nil, domain.NewConflictError("NIK sudah terdaftar pada pendaftaran aktif")
		} else if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}

	niaCode, err := s.anggotaRepo.AllocateNIA(ctx, in.ProvinsiID, in.KabupatenID, time.Now().Year())
	if err != nil {
		return nil, err
	}

	entity := &domain.Anggota{
		NIA:          niaCode,
		NamaLengkap:  name,
		NIKHash:      nikHash,
		NIKEncrypted: nikEnc,
		TempatLahir:  tempat,
		TanggalLahir: dob,
		JenisKelamin: jk,
		Agama:        strings.TrimSpace(in.Agama),
		Pendidikan:   strings.TrimSpace(in.Pendidikan),
		Pekerjaan:    strings.TrimSpace(in.Pekerjaan),
		Alamat:       alamat,
		ProvinsiID:   in.ProvinsiID,
		KabupatenID:  in.KabupatenID,
		Kecamatan:    strings.TrimSpace(in.Kecamatan),
		Desa:         strings.TrimSpace(in.Desa),
		KodePos:      strings.TrimSpace(in.KodePos),
		Email:        email,
		Whatsapp:     wa,
		Status:       status,
		Tipe:         domain.TipePendaftaranKader,
		Angkatan:     strings.TrimSpace(in.Angkatan),
	}

	out, err := s.anggotaRepo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"anggota_create","nia":"` + out.NIA + `"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"anggota", strconv.Itoa(out.ID), "CREATE", &meta)
	return out, nil
}

// UpdateAnggota menyunting data anggota. Field nil = tidak diubah; NIK/NIA tetap.
func (s *anggotaService) UpdateAnggota(ctx context.Context, id int, in domain.AnggotaUpdateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	item, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}

	if in.NamaLengkap != nil {
		v := strings.TrimSpace(*in.NamaLengkap)
		if len([]rune(v)) < 3 || len([]rune(v)) > svcutil.MaxNamaLen || containsAngleBracket(v) {
			return nil, domain.NewValidationError("Nama lengkap wajib 3-150 karakter & tanpa karakter < atau >")
		}
		item.NamaLengkap = v
	}
	if in.TempatLahir != nil {
		v := strings.TrimSpace(*in.TempatLahir)
		if v == "" || len([]rune(v)) > svcutil.MaxTempatLahirLen || containsAngleBracket(v) {
			return nil, domain.NewValidationError("Tempat lahir wajib diisi (maks 100 karakter)")
		}
		item.TempatLahir = v
	}
	if in.TanggalLahir != nil {
		dob, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.TanggalLahir))
		if err != nil {
			return nil, domain.NewValidationError("Format tanggal lahir tidak valid")
		}
		item.TanggalLahir = dob
	}
	if in.JenisKelamin != nil {
		jk := strings.ToUpper(strings.TrimSpace(*in.JenisKelamin))
		if jk != "L" && jk != "P" {
			return nil, domain.NewValidationError("Jenis kelamin harus L atau P")
		}
		item.JenisKelamin = jk
	}
	if in.Agama != nil {
		item.Agama = strings.TrimSpace(*in.Agama)
	}
	if in.Pendidikan != nil {
		item.Pendidikan = strings.TrimSpace(*in.Pendidikan)
	}
	if in.Pekerjaan != nil {
		item.Pekerjaan = strings.TrimSpace(*in.Pekerjaan)
	}
	if in.Alamat != nil {
		v := strings.TrimSpace(*in.Alamat)
		if len([]rune(v)) < 5 || len([]rune(v)) > svcutil.MaxAlamatLen || containsAngleBracket(v) {
			return nil, domain.NewValidationError("Alamat wajib 5-2000 karakter & tanpa karakter < atau >")
		}
		item.Alamat = v
	}
	// Perubahan wilayah (opsional) + validasi relasi & scope.
	newProv, newKab := item.ProvinsiID, item.KabupatenID
	if in.ProvinsiID != nil {
		newProv = *in.ProvinsiID
	}
	if in.KabupatenID != nil {
		newKab = *in.KabupatenID
	}
	if newProv != item.ProvinsiID || newKab != item.KabupatenID {
		if newProv <= 0 || newKab <= 0 {
			return nil, domain.NewValidationError("Wilayah tidak valid")
		}
		if !actor.CanAccessWilayah(newProv, newKab) {
			return nil, domain.NewForbiddenError("Wilayah tujuan di luar kewenangan Anda")
		}
		if s.wilayahRepo != nil {
			ok, err := s.wilayahRepo.KabupatenInProvinsi(ctx, newKab, newProv)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, domain.NewValidationError("Kabupaten/Kota tidak valid untuk provinsi tersebut")
			}
		}
		item.ProvinsiID, item.KabupatenID = newProv, newKab
	}
	if in.Kecamatan != nil {
		item.Kecamatan = strings.TrimSpace(*in.Kecamatan)
	}
	if in.Desa != nil {
		item.Desa = strings.TrimSpace(*in.Desa)
	}
	if in.KodePos != nil {
		kp := strings.TrimSpace(*in.KodePos)
		if kp != "" && !svcutil.KodePosPattern.MatchString(kp) {
			return nil, domain.NewValidationError("Kode pos wajib 5 digit angka")
		}
		item.KodePos = kp
	}
	if in.Email != nil {
		e := strings.TrimSpace(*in.Email)
		if _, err := mail.ParseAddress(e); err != nil {
			return nil, domain.NewValidationError("Format email tidak valid")
		}
		item.Email = e
	}
	if in.Whatsapp != nil {
		w := strings.TrimSpace(*in.Whatsapp)
		if !svcutil.PhonePattern.MatchString(w) {
			return nil, domain.NewValidationError("Nomor WhatsApp tidak valid")
		}
		item.Whatsapp = w
	}
	if in.Angkatan != nil {
		item.Angkatan = strings.TrimSpace(*in.Angkatan)
	}
	if in.Status != nil {
		st := domain.AnggotaStatus(strings.ToUpper(strings.TrimSpace(*in.Status)))
		if !st.IsValid() {
			return nil, domain.NewValidationError("Status anggota tidak valid")
		}
		item.Status = st
	}

	if err := s.anggotaRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	meta := `{"event":"anggota_update"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"anggota", strconv.Itoa(id), "UPDATE", &meta)
	return item, nil
}

// SetAnggotaStatus mengubah status keanggotaan (soft delete = NONAKTIF).
func (s *anggotaService) SetAnggotaStatus(ctx context.Context, id int, in domain.AnggotaStatusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	st := domain.AnggotaStatus(strings.ToUpper(strings.TrimSpace(in.Status)))
	if !st.IsValid() {
		return nil, domain.NewValidationError("Status anggota tidak valid")
	}
	item, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}
	if err := s.anggotaRepo.SetStatus(ctx, id, st); err != nil {
		return nil, err
	}
	item.Status = st
	meta := `{"event":"anggota_status","to":"` + string(st) + `"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"anggota", strconv.Itoa(id), "UPDATE", &meta)
	return item, nil
}

// ExportCSV mengekspor daftar anggota ter-scope (dibatasi agar respons wajar).
func (s *anggotaService) ExportCSV(ctx context.Context, actor domain.ActorContext, status, search string) ([]byte, error) {
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, err
	}
	q := strings.TrimSpace(search)
	if q != "" && len([]rune(q)) > 100 {
		return nil, domain.NewValidationError("Kata kunci pencarian maksimal 100 karakter")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"NIA", "Nama", "Pekerjaan", "Riwayat", "Provinsi", "Kabupaten", "Status"}); err != nil {
		return nil, err
	}
	const pageSize = 100
	const maxRows = 5000
	offset := 0
	for {
		items, err := s.anggotaRepo.ListAnggota(ctx, prov, kab, status, q, pageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		s.attachRiwayat(ctx, items)
		for i := range items {
			it := items[i]
			if err := w.Write([]string{it.NIA, it.NamaLengkap, it.Pekerjaan, it.Riwayat, it.ProvinsiNama, it.KabupatenNama, it.Status}); err != nil {
				return nil, err
			}
		}
		offset += len(items)
		if len(items) < pageSize || offset >= maxRows {
			break
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
