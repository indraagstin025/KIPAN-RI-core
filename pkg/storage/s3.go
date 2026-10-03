package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// ErrObjectNotFound dikembalikan saat object tidak ada di bucket.
var ErrObjectNotFound = errors.New("object tidak ditemukan di storage")

// ObjectInfo adalah metadata object dari HeadObject.
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// Client adalah wrapper tipis S3-compatible (MinIO dev, R2, IDCloudHost IS3).
// Semua operasi memakai path-style agar kompatibel MinIO.
type Client struct {
	s3       *s3.Client
	presign  *s3.PresignClient
	endpoint string
}

// NewClient membangun S3 client dari konfigurasi eksplisit. Tidak ada
// default kredensial: endpoint/key kosong berarti storage tidak
// dikonfigurasi (service layer wajib fail-closed, RULES 14).
func NewClient(endpoint, region, accessKey, secretKey string) (*Client, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("STORAGE_ENDPOINT belum dikonfigurasi")
	}
	if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" {
		return nil, errors.New("kredensial storage (access/secret key) belum dikonfigurasi")
	}
	if strings.TrimSpace(region) == "" {
		region = "auto"
	}

	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	// BaseEndpoint (bukan resolver legacy): mengisi resolved URI sehingga
	// presigned POST ikut bekerja, bukan hanya PUT/GET. Tetap path-style
	// untuk kompatibilitas MinIO.
	s3client := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})
	return &Client{
		s3:       s3client,
		presign:  s3.NewPresignClient(s3client),
		endpoint: endpoint,
	}, nil
}

// Endpoint mengembalikan endpoint yang dipakai (untuk log/debug).
func (c *Client) Endpoint() string { return c.endpoint }

// PresignPostUpload menerbitkan POST policy (browser → S3, bypass backend)
// dengan batas ukuran KERAS via kondisi content-length-range. Berbeda dari
// presigned PUT, S3/MinIO MENOLAK body di luar rentang ini di level storage,
// sehingga mencegah penyalahgunaan tiket untuk mengisi bucket (DoS storage).
//
// Key dibatasi persis oleh SDK (kondisi "key"); Content-Type tetap
// diverifikasi ulang di backend saat submit (HeadObject + magic bytes).
func (c *Client) PresignPostUpload(ctx context.Context, bucket, key string, maxSize int64, expiry time.Duration) (string, map[string]string, error) {
	if maxSize <= 0 {
		return "", nil, errors.New("batas ukuran upload tidak valid")
	}
	out, err := c.presign.PresignPostObject(ctx,
		&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
		func(o *s3.PresignPostOptions) {
			o.Expires = expiry
			o.Conditions = []any{
				// S3 menegakkan ukuran object di antara min..max saat POST.
				[]any{"content-length-range", 0, maxSize},
			}
		},
	)
	if err != nil {
		return "", nil, err
	}
	return out.URL, out.Values, nil
}

// PresignGet menerbitkan URL unduh sementara untuk dokumen privat.
func (c *Client) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	out, err := c.presign.PresignGetObject(ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(expiry),
	)
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// Stat mengambil metadata object (existence + size + content-type).
func (c *Client) Stat(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	var ctype string
	if out.ContentType != nil {
		ctype = *out.ContentType
	}
	return &ObjectInfo{Size: size, ContentType: ctype}, nil
}

// SniffHead mengunduh N byte pertama object (Range GET) untuk validasi
// magic bytes tanpa memuat seluruh file ke memori backend (RULES 13).
func (c *Client) SniffHead(ctx context.Context, bucket, key string, n int64) ([]byte, error) {
	if n <= 0 || n > 4096 {
		n = 512
	}
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=0-%d", n-1)),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(io.LimitReader(out.Body, n))
}

// SniffTail mengunduh N byte TERAKHIR object (suffix Range GET) untuk
// inspeksi trailer PDF (deteksi enkripsi) tanpa memuat file utuh.
// Batas 8 KB: trailer + startxref dokumen ≤5 MB selalu muat di ekor ini.
func (c *Client) SniffTail(ctx context.Context, bucket, key string, n int64) ([]byte, error) {
	if n <= 0 || n > 8192 {
		n = 4096
	}
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=-%d", n)),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(io.LimitReader(out.Body, n))
}

// LooksEncryptedPDF mendeteksi PDF terkunci (user/owner password) via
// penanda /Encrypt di ekor file. Kamus trailer (yang memuat /Encrypt)
// terletak tepat sebelum startxref akhir, sehingga ekor ≤8 KB selalu
// mencakupnya untuk dokumen ≤5 MB.
//
// Heuristik yang disengaja (bukan parser PDF penuh): false-negative mungkin
// pada PDF non-standar/linearized eksotis — verifikasi manual admin tetap
// backstop. False-positive praktis mustahil: isi content stream umumnya
// terkompresi sehingga literal "/Encrypt" tak terbaca, dan kemunculan di
// trailer selalu berarti kamus enkripsi (posisinya terikat spesifikasi).
func LooksEncryptedPDF(tail []byte) bool {
	if len(tail) == 0 {
		return false
	}
	idx := bytes.Index(tail, []byte("/Encrypt"))
	if idx < 0 {
		return false
	}
	after := idx + len("/Encrypt")
	if after >= len(tail) {
		return false
	}
	switch tail[after] {
	case ' ', '\t', '\n', '\r', '\x00', '\f', '/', '<', '[', '(':
		return true
	}
	return false
}

// Put mengunggah object kecil (mis. PDF KTA) langsung dari backend.
// Hanya untuk artefak server-generated; upload user tetap via presign.
func (c *Client) Put(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	if len(data) == 0 {
		return errors.New("data object kosong")
	}
	if err := ValidateObjectKey(key); err != nil {
		return err
	}
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

// EnsureBucket membuat bucket bila belum ada. Dipakai saat startup di
// non-production agar dev tidak perlu provisioning manual.
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	if _, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}); err == nil {
		return nil
	} else if !isNotFound(err) {
		return err
	}
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket", "NoSuchUpload":
			return true
		}
	}
	return false
}

// ============================================================
// Object key validation (pure, unit-testable)
// ============================================================

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_\.\-]{0,254}$`)

// ValidateObjectKey memastikan key aman: allowlist karakter, tanpa
// traversal (..), maks 255 char. dipakai service sebelum HeadObject.
func ValidateObjectKey(key string) error {
	k := strings.TrimSpace(key)
	if k == "" {
		return errors.New("object key kosong")
	}
	if len(k) > 255 || !keyPattern.MatchString(k) || strings.Contains(k, "..") {
		return errors.New("object key tidak valid: " + k)
	}
	return nil
}

// ============================================================
// Magic bytes validation (pure, unit-testable)
// ============================================================

var (
	magicJPEG = []byte{0xFF, 0xD8, 0xFF}
	magicPNG  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	magicPDF  = []byte("%PDF-")
)

// ValidateMagicBytes memastikan isi biner cocok dengan MIME yang
// dideklarasikan (RULES 15: jangan percaya metadata saja).
func ValidateMagicBytes(data []byte, mimeType string) error {
	var magic []byte
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		magic = magicJPEG
	case "image/png":
		magic = magicPNG
	case "application/pdf":
		magic = magicPDF
	default:
		return errors.New("tipe MIME tidak didukung untuk validasi magic bytes: " + mimeType)
	}
	if len(data) < len(magic) || !bytes.Equal(data[:len(magic)], magic) {
		return errors.New("isi file tidak cocok dengan tipe " + mimeType + " (magic bytes mismatch)")
	}
	return nil
}
