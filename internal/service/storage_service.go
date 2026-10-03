package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/storage"
)

// ObjectVerifier adalah kontrak verifikasi dokumen yang dipakai
// pendaftaranService agar bisa di-fake di test. *StorageService
// mengimplementasikannya.
type ObjectVerifier interface {
	Configured() bool
	VerifySubmittedObject(ctx context.Context, key, category string) error
}

// CategoryPolicy mengatur batas tiap kategori dokumen (RULES 14).
type CategoryPolicy struct {
	MIMEs   []string
	MaxSize int64
}

// Kebijakan kategori dokumen pendaftaran. Ukuran selaras Task 1.1.3:
// foto & KTP maks 2MB, dokumen PDF lain maks 5MB.
var categoryPolicies = map[string]CategoryPolicy{
	"foto":             {MIMEs: []string{"image/jpeg", "image/png"}, MaxSize: 2 << 20},
	"ktp":              {MIMEs: []string{"image/jpeg", "image/png", "application/pdf"}, MaxSize: 2 << 20},
	"cv":               {MIMEs: []string{"application/pdf"}, MaxSize: 2 << 20},
	"sk":               {MIMEs: []string{"application/pdf"}, MaxSize: 5 << 20},
	"surat_pernyataan": {MIMEs: []string{"application/pdf"}, MaxSize: 5 << 20},
	"surat_sehat":      {MIMEs: []string{"application/pdf"}, MaxSize: 5 << 20},
}

// mimeToExt menurunkan ekstensi dari MIME (bukan dari nama file client)
// agar double-extension (mis. ktp.jpg.php) tidak mungkin lolos.
var mimeToExt = map[string]string{
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"application/pdf": "pdf",
}

// PresignUploadResult adalah tiket upload langsung ke S3 (POST policy).
// Fields WAJIB dikirim apa adanya sebagai form-data bersama berkas (field
// "file"), dengan urutan fields lebih dulu, berkas terakhir.
type PresignUploadResult struct {
	UploadURL string            `json:"upload_url"`
	Fields    map[string]string `json:"fields"`
	ObjectKey string            `json:"object_key"`
	Bucket    string            `json:"bucket"`
	ExpiresIn int64             `json:"expires_in"`
	MIMEType  string            `json:"mime_type"`
	MaxSize   int64             `json:"max_size_bytes"`
}

// PresignViewResult adalah tiket unduh sementara dokumen privat.
type PresignViewResult struct {
	ViewURL   string `json:"view_url"`
	ObjectKey string `json:"object_key"`
	ExpiresIn int64  `json:"expires_in"`
}

// StorageService mengorkestrasi presigned upload/view (RULES 13, 14, 15).
// Client boleh nil (storage tidak dikonfigurasi): semua operasi publik
// gagal fail-closed 503, kecuali verifikasi submit yang degraded eksplisit
// hanya di non-production (pola yang sama dengan Redis).
type StorageService struct {
	cfg           *config.Config
	client        *storage.Client
	auditRepo     repository.AuditLogRepository
	ownerResolver DocumentOwnerResolver
}

// DocumentOwnerResolver menentukan pemilik (jurisdiksi) sebuah object key
// dokumen. Wajib ada agar akses baca dapat DIOTORISASI (SEC-STORE-BOLA),
// bukan sekadar diautentikasi. Nil = fail-closed (tiket tidak diterbitkan).
type DocumentOwnerResolver interface {
	ResolveOwner(ctx context.Context, key string) (*repository.DocumentOwner, error)
}

func NewStorageService(
	cfg *config.Config,
	client *storage.Client,
	auditRepo repository.AuditLogRepository,
	ownerResolver DocumentOwnerResolver,
) *StorageService {
	return &StorageService{cfg: cfg, client: client, auditRepo: auditRepo, ownerResolver: ownerResolver}
}

// Configured mengembalikan true bila S3 client tersedia.
func (s *StorageService) Configured() bool { return s.client != nil }

func (s *StorageService) uploadsBucket() string {
	if s.cfg != nil && s.cfg.Storage.BucketUploads != "" {
		return s.cfg.Storage.BucketUploads
	}
	return "kipan-uploads"
}

func (s *StorageService) putTTL() time.Duration {
	if s.cfg != nil && s.cfg.Storage.PresignedTTL > 0 {
		return s.cfg.Storage.PresignedTTL
	}
	return 10 * time.Minute
}

func policyFor(category string) (CategoryPolicy, error) {
	p, ok := categoryPolicies[strings.ToLower(strings.TrimSpace(category))]
	if !ok {
		return CategoryPolicy{}, domain.NewValidationError("Kategori dokumen tidak dikenal")
	}
	return p, nil
}

// RequestUploadPresign menerbitkan tiket upload langsung (browser → S3)
// berupa POST policy dengan batas ukuran KERAS (content-length-range) —
// S3/MinIO menolak body di luar batas, menutup DoS storage yang mungkin
// pada presigned PUT. Validasi awal: kategori dikenal, MIME allowlist,
// ukuran dalam batas (pertahanan berlapis; penegakan sebenarnya di storage).
func (s *StorageService) RequestUploadPresign(ctx context.Context, category, fileName, mimeType string, fileSize int64) (*PresignUploadResult, error) {
	policy, err := policyFor(category)
	if err != nil {
		return nil, err
	}
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	allowed := false
	for _, m := range policy.MIMEs {
		if mime == m {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, domain.NewValidationError("Tipe file tidak diizinkan untuk kategori " + category)
	}
	if fileSize <= 0 || fileSize > policy.MaxSize {
		return nil, domain.NewValidationError(
			fmt.Sprintf("Ukuran file harus 1-%d byte untuk kategori %s", policy.MaxSize, category))
	}
	if s.client == nil {
		return nil, unavailable("storage")
	}

	ext := mimeToExt[mime]
	key := fmt.Sprintf("uploads/pendaftaran/%s/%s.%s",
		time.Now().Format("200601"), uuid.NewString(), ext)

	ttl := s.putTTL()
	url, fields, err := s.client.PresignPostUpload(ctx, s.uploadsBucket(), key, policy.MaxSize, ttl)
	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan tiket upload: %w", err)
	}
	// Objek yatim (upload tanpa submit) dibersihkan lifecycle rule bucket:
	// hapus objek uploads/ berumur > 7 hari tanpa referensi DB (infra,
	// mis. `mc ilm rule add --expire-days 7`). ClamAV pra-approval Fase 6.
	log.Info().Str("category", category).Str("bucket", s.uploadsBucket()).
		Msg("Tiket upload (POST policy, ukuran dibatasi storage) diterbitkan")
	_ = fileName // nama file client tidak dipakai (key server-generated)
	return &PresignUploadResult{
		UploadURL: url,
		Fields:    fields,
		ObjectKey: key,
		Bucket:    s.uploadsBucket(),
		ExpiresIn: int64(ttl.Seconds()),
		MIMEType:  mime,
		MaxSize:   policy.MaxSize,
	}, nil
}

// VerifySubmittedObject memverifikasi object yang diklaim sudah di-upload
// SEBELUM referensinya disimpan ke database (RULES 14): pola key aman,
// object ada (HeadObject), ukuran dalam batas, content-type allowlist, dan
// magic bytes cocok (Range GET 512 byte pertama, tanpa memuat file utuh).
func (s *StorageService) VerifySubmittedObject(ctx context.Context, key, category string) error {
	policy, err := policyFor(category)
	if err != nil {
		return err
	}
	k := strings.TrimSpace(key)
	if err := storage.ValidateObjectKey(k); err != nil {
		return domain.NewValidationError("Object key dokumen tidak valid")
	}
	if s.client == nil {
		return unavailable("storage")
	}

	info, err := s.client.Stat(ctx, s.uploadsBucket(), k)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return domain.NewValidationError("Dokumen belum di-upload (object tidak ditemukan)")
	}
	if err != nil {
		return fmt.Errorf("gagal memverifikasi dokumen: %w", err)
	}
	if info.Size <= 0 || info.Size > policy.MaxSize {
		return domain.NewValidationError("Ukuran dokumen tidak sesuai ketentuan kategori " + category)
	}
	ctype := strings.ToLower(strings.TrimSpace(info.ContentType))
	// S3 menggemakan content-type; pecah parameter (mis. "; charset=") bila ada.
	if i := strings.Index(ctype, ";"); i >= 0 {
		ctype = strings.TrimSpace(ctype[:i])
	}
	allowed := false
	for _, m := range policy.MIMEs {
		if ctype == m {
			allowed = true
			break
		}
	}
	if !allowed {
		return domain.NewValidationError("Tipe dokumen tersimpan tidak sesuai kategori " + category)
	}

	head, err := s.client.SniffHead(ctx, s.uploadsBucket(), k, 512)
	if err != nil {
		return fmt.Errorf("gagal membaca isi dokumen: %w", err)
	}
	if err := storage.ValidateMagicBytes(head, ctype); err != nil {
		return domain.NewValidationError("Isi dokumen tidak cocok dengan tipenya (file rusak atau dimanipulasi)")
	}
	// L8: PDF terkunci password ditolak sejak submit — admin tidak boleh
	// menerima berkas yang tak bisa dibuka. Kebijakan: tolak (bukan minta
	// password: kredensial dokumen tidak pernah dikumpulkan/disimpan).
	if ctype == "application/pdf" {
		tail, err := s.client.SniffTail(ctx, s.uploadsBucket(), k, 4096)
		if err != nil {
			return fmt.Errorf("gagal membaca ekor dokumen: %w", err)
		}
		if storage.LooksEncryptedPDF(tail) {
			return domain.NewValidationError("Dokumen " + category + " terkunci password (kata sandi). Unggah versi tanpa password agar dapat diverifikasi")
		}
	}
	return nil
}

// RequestViewPresign menerbitkan tiket baca sementara untuk dokumen privat
// (TTL 5 menit) sekaligus mencatat siapa yang melihat (RULES 12, 21).
func (s *StorageService) RequestViewPresign(ctx context.Context, key string, actor domain.ActorContext, audit domain.AuditContext) (*PresignViewResult, error) {
	k := strings.TrimSpace(key)
	if err := storage.ValidateObjectKey(k); err != nil {
		return nil, domain.NewValidationError("Object key dokumen tidak valid")
	}
	if s.client == nil {
		return nil, unavailable("storage")
	}

	// OTORISASI (SEC-STORE-BOLA): tiket baca HANYA untuk dokumen milik entitas
	// dalam yurisdiksi aktor. Autentikasi saja tidak cukup (BOLA/IDOR).
	// Resolver nil = fail-closed: jangan pernah menerbitkan tiket tanpa otorisasi.
	if s.ownerResolver == nil {
		return nil, unavailable("otorisasi dokumen")
	}
	owner, err := s.ownerResolver.ResolveOwner(ctx, k)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("Dokumen")
		}
		return nil, err
	}
	if !actor.CanAccessWilayah(owner.ProvinsiID, owner.KabupatenID) {
		return nil, domain.NewForbiddenError("Dokumen di luar wilayah kerja Anda")
	}

	const ttl = 5 * time.Minute
	url, err := s.client.PresignGet(ctx, s.uploadsBucket(), k, ttl)
	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan tiket baca: %w", err)
	}
	if s.auditRepo != nil {
		actorID := actor.UserID
		meta := `{"object_key":"` + k + `"}`
		e := &domain.ActivityLog{
			ActorID:    &actorID,
			ActorName:  actor.Name,
			ActorRole:  string(actor.Role),
			IPAddress:  audit.IP,
			UserAgent:  audit.UserAgent,
			EntityName: "dokumen",
			EntityID:   k,
			Action:     "VIEW",
			Metadata:   &meta,
			RequestID:  audit.RequestID,
		}
		if err := s.auditRepo.Create(ctx, e); err != nil {
			log.Warn().Err(err).Str("object_key", k).Msg("Gagal mencatat audit view dokumen")
		}
	}
	return &PresignViewResult{ViewURL: url, ObjectKey: k, ExpiresIn: int64(ttl.Seconds())}, nil
}

// PutKTADocument mengunggah PDF KTA server-generated ke bucket private.
// Key server-generated: kta/{NIA}.pdf. Bukan jalur upload user.
func (s *StorageService) PutKTADocument(ctx context.Context, nia string, pdf []byte) (string, error) {
	if s.client == nil {
		return "", unavailable("storage")
	}
	key := "kta/" + strings.TrimSpace(nia) + ".pdf"
	if err := storage.ValidateObjectKey(key); err != nil {
		return "", domain.NewValidationError("NIA tidak valid untuk key KTA")
	}
	bucket := s.privateBucket()
	if err := s.client.Put(ctx, bucket, key, pdf, "application/pdf"); err != nil {
		return "", fmt.Errorf("gagal menyimpan PDF KTA: %w", err)
	}
	return key, nil
}

// PresignKTADocument menerbitkan tiket baca sementara PDF KTA (5 menit).
func (s *StorageService) PresignKTADocument(ctx context.Context, key string) (string, error) {
	k := strings.TrimSpace(key)
	if err := storage.ValidateObjectKey(k); err != nil {
		return "", domain.NewValidationError("Object key KTA tidak valid")
	}
	if s.client == nil {
		return "", unavailable("storage")
	}
	const ttl = 5 * time.Minute
	url, err := s.client.PresignGet(ctx, s.privateBucket(), k, ttl)
	if err != nil {
		return "", fmt.Errorf("gagal menerbitkan tiket baca KTA: %w", err)
	}
	return url, nil
}

func (s *StorageService) privateBucket() string {
	if s.cfg != nil && s.cfg.Storage.BucketPrivate != "" {
		return s.cfg.Storage.BucketPrivate
	}
	return "kipan-private"
}

// EnsureBuckets membuat bucket yang belum ada. Dipanggil sekali saat
// startup non-production; kegagalan hanya warning (dev ergonomics).
func (s *StorageService) EnsureBuckets(ctx context.Context) {
	if s.client == nil {
		return
	}
	buckets := map[string]string{
		"uploads": s.uploadsBucket(),
		"public":  s.cfg.Storage.BucketPublic,
		"private": s.cfg.Storage.BucketPrivate,
	}
	for label, b := range buckets {
		if strings.TrimSpace(b) == "" {
			continue
		}
		if err := s.client.EnsureBucket(ctx, b); err != nil {
			log.Warn().Err(err).Str("bucket", b).Str("label", label).
				Msg("Gagal menyiapkan bucket storage (lanjut tanpa bucket)")
		}
	}
}
