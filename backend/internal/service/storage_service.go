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

// PresignUploadResult adalah tiket upload langsung ke S3.
type PresignUploadResult struct {
	UploadURL  string `json:"upload_url"`
	ObjectKey  string `json:"object_key"`
	Bucket     string `json:"bucket"`
	ExpiresIn  int64  `json:"expires_in"`
	MIMEType   string `json:"mime_type"`
	MaxSize    int64  `json:"max_size_bytes"`
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
	cfg       *config.Config
	client    *storage.Client
	auditRepo repository.AuditLogRepository
}

func NewStorageService(cfg *config.Config, client *storage.Client, auditRepo repository.AuditLogRepository) *StorageService {
	return &StorageService{cfg: cfg, client: client, auditRepo: auditRepo}
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

// RequestUploadPresign menerbitkan tiket upload langsung (browser → S3).
// Validasi: kategori dikenal, MIME allowlist, ukuran dalam batas.
func (s *StorageService) RequestUploadPresign(_ context.Context, category, fileName, mimeType string, fileSize int64) (*PresignUploadResult, error) {
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
		return nil, domain.NewUnavailableError("Layanan storage belum dikonfigurasi")
	}

	ext := mimeToExt[mime]
	key := fmt.Sprintf("uploads/pendaftaran/%s/%s.%s",
		time.Now().Format("200601"), uuid.NewString(), ext)

	ttl := s.putTTL()
	url, err := s.client.PresignPut(context.Background(), s.uploadsBucket(), key, mime, ttl)
	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan tiket upload: %w", err)
	}
	_ = fileName // nama file client tidak dipakai (key server-generated)
	return &PresignUploadResult{
		UploadURL: url,
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
		return domain.NewUnavailableError("Layanan storage belum dikonfigurasi")
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
		return nil, domain.NewUnavailableError("Layanan storage belum dikonfigurasi")
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
