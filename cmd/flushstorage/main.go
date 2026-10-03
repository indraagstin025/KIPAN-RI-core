// Package main — Utility DEV untuk mengosongkan bucket MinIO/S3.
//
// Menghapus SELURUH object di bucket public/private/uploads (bucket itu
// sendiri dipertahankan; dibuat ulang otomatis oleh EnsureBuckets saat
// server start di non-production).
//
// ⚠️  HANYA UNTUK DEV/STAGING — fail-closed di luar APP_ENV lokal.
//
//	cd backend
//	go run ./cmd/flushstorage
package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var allowedEnvs = map[string]bool{
	"development": true,
	"dev":         true,
	"test":        true,
	"local":       true,
}

func main() {
	setupLogger()

	v := loadEnv()
	appEnv := strings.ToLower(strings.TrimSpace(v.GetString("APP_ENV")))
	if appEnv == "" {
		appEnv = "development"
	}
	if !allowedEnvs[appEnv] {
		log.Fatal().Str("app_env", appEnv).
			Msg("❌ flushstorage ditolak: hanya boleh dijalankan di development/test/local")
	}

	endpoint := strings.TrimSpace(v.GetString("STORAGE_ENDPOINT"))
	access := strings.TrimSpace(v.GetString("STORAGE_ACCESS_KEY_ID"))
	secret := strings.TrimSpace(v.GetString("STORAGE_SECRET_ACCESS_KEY"))
	if endpoint == "" || access == "" || secret == "" {
		log.Warn().Msg("Konfigurasi storage kosong — tidak ada yang dibersihkan")
		return
	}
	region := v.GetString("STORAGE_REGION")
	if region == "" {
		region = "auto"
	}

	buckets := []string{
		v.GetString("STORAGE_BUCKET_PUBLIC"),
		v.GetString("STORAGE_BUCKET_PRIVATE"),
		v.GetString("STORAGE_BUCKET_UPLOADS"),
	}

	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal membangun konfigurasi AWS")
	}
	client := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	total := 0
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		n, err := purgeBucket(ctx, client, bucket)
		if err != nil {
			log.Warn().Err(err).Str("bucket", bucket).Msg("Gagal mengosongkan bucket")
			continue
		}
		log.Info().Str("bucket", bucket).Int("objects", n).Msg("✅ Bucket dikosongkan")
		total += n
	}
	log.Info().Int("total_objects", total).Msg("🎉 Storage dibersihkan")
}

func purgeBucket(ctx context.Context, client *s3.Client, bucket string) (int, error) {
	deleted := 0
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deleted, err
		}
		for _, o := range page.Contents {
			// Hapus satu-per-satu: MinIO menolak DeleteObjects batch tanpa
			// header Content-MD5 yang tidak disisipkan SDK secara otomatis.
			if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    o.Key,
			}); err != nil {
				return deleted, err
			}
			deleted++
		}
	}
	return deleted, nil
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

func loadEnv() *viper.Viper {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.ReadInConfig()
	return v
}
