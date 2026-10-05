package service

// backup_service.go — backup database server-side (pg_dump → bucket privat).
// Super Admin saja (ditegakkan di route). Fail-closed bila pg_dump/storage tak
// tersedia. DSN TIDAK pernah dicatat ke log.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// BackupStore adalah kapabilitas storage privat yang dibutuhkan backup.
type BackupStore interface {
	Configured() bool
	PutPrivateObject(ctx context.Context, key string, data []byte, contentType string) error
	PresignPrivateObject(ctx context.Context, key string) (string, error)
	DeletePrivateObject(ctx context.Context, key string) error
}

// BackupService mengelola backup database.
type BackupService interface {
	Create(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext) (*domain.Backup, error)
	List(ctx context.Context, page, limit int) ([]domain.Backup, int, error)
	DownloadURL(ctx context.Context, id int) (string, error)
	Delete(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) error
}

type backupService struct {
	cfg       *config.Config
	repo      repository.BackupRepository
	store     BackupStore
	auditRepo repository.AuditLogRepository
}

// NewBackupService membangun service backup.
func NewBackupService(cfg *config.Config, repo repository.BackupRepository, store BackupStore, auditRepo repository.AuditLogRepository) BackupService {
	return &backupService{cfg: cfg, repo: repo, store: store, auditRepo: auditRepo}
}

// Create menjalankan pg_dump, mengunggah arsip ke bucket privat, lalu mencatat
// riwayat. Tidak ada backup bersamaan (dijalankan serial oleh request admin).
func (s *backupService) Create(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext) (*domain.Backup, error) {
	if s.cfg == nil || !s.cfg.Backup.Enabled {
		return nil, svcutil.Unavailable("backup (nonaktif)")
	}
	if s.repo == nil {
		return nil, svcutil.Unavailable("backup")
	}
	if s.store == nil || !s.store.Configured() {
		return nil, svcutil.Unavailable("storage")
	}
	dsn := strings.TrimSpace(s.cfg.Database.DSN)
	if dsn == "" {
		return nil, svcutil.Unavailable("database DSN")
	}
	bin := strings.TrimSpace(s.cfg.Backup.PgDumpPath)
	if bin == "" {
		bin = "pg_dump"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, svcutil.Unavailable("pg_dump tidak tersedia di server (set BACKUP_PG_DUMP_PATH)")
	}

	tmp, err := os.CreateTemp("", "kipan-backup-*.dump")
	if err != nil {
		return nil, fmt.Errorf("gagal membuat berkas sementara: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	timeout := s.cfg.Backup.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, path, "--format=custom", "--no-owner", "--no-privileges", "--file", tmpPath, dsn)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Jangan bocorkan DSN; catat hanya status akhir.
		return nil, fmt.Errorf("pg_dump gagal: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca hasil backup: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("hasil backup kosong")
	}

	stamp := time.Now().Format("20060102-150405")
	filename := "kipan-" + stamp + ".dump"
	key := "backups/" + filename
	if err := s.store.PutPrivateObject(ctx, key, data, "application/octet-stream"); err != nil {
		return nil, err
	}

	by := actor.UserID
	out, err := s.repo.Create(ctx, &domain.Backup{
		Filename: filename, ObjectKey: key, SizeBytes: int64(len(data)),
		Status: "ready", CreatedBy: &by,
	})
	if err != nil {
		_ = s.store.DeletePrivateObject(ctx, key)
		return nil, err
	}
	meta := `{"event":"backup_create","filename":"` + filename + `"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"backups", fmt.Sprintf("%d", out.ID), "CREATE", &meta)
	log.Info().Str("filename", filename).Int64("size", int64(len(data))).Msg("Backup database dibuat")
	return out, nil
}

// List mengembalikan riwayat backup.
func (s *backupService) List(ctx context.Context, page, limit int) ([]domain.Backup, int, error) {
	if s.repo == nil {
		return nil, 0, svcutil.Unavailable("backup")
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	return s.repo.List(ctx, limit, (page-1)*limit)
}

// DownloadURL menerbitkan tiket baca arsip backup.
func (s *backupService) DownloadURL(ctx context.Context, id int) (string, error) {
	if s.repo == nil || s.store == nil {
		return "", svcutil.Unavailable("backup")
	}
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return s.store.PresignPrivateObject(ctx, b.ObjectKey)
}

// Delete menghapus arsip + riwayatnya.
func (s *backupService) Delete(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) error {
	if s.repo == nil {
		return svcutil.Unavailable("backup")
	}
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if s.store != nil && s.store.Configured() {
		if err := s.store.DeletePrivateObject(ctx, b.ObjectKey); err != nil {
			log.Warn().Err(err).Int("id", id).Msg("Gagal menghapus objek backup di storage")
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	meta := `{"event":"backup_delete","filename":"` + b.Filename + `"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"backups", fmt.Sprintf("%d", id), "DELETE", &meta)
	return nil
}
