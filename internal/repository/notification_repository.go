package repository

import (
	"context"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// NotificationRepository menangani notifikasi in-app minimal untuk admin
// verifikator (pengganti aman notifyAdmins proyek lama).
type NotificationRepository interface {
	// NotifyAdmins menyebar notifikasi ke admin aktif yang berhak:
	// SUPER_ADMIN + ADMIN_NASIONAL (semua), ADMIN_PROVINSI (seprovinsi),
	// ADMIN_KABUPATEN (sekabupaten). Satu statement, tanpa N+1.
	NotifyAdmins(ctx context.Context, title, message string, notifType domain.NotificationType, link string, provinsiID, kabupatenID int) error
	// NotifyUser mengirim satu notifikasi in-app ke user tertentu (mis. anggota
	// yang didemosikan otomatis dan punya akun). Best-effort: FK users
	// memastikan user ada; user terhapus → error (pemanggil mencatat saja).
	NotifyUser(ctx context.Context, userID, title, message string, notifType domain.NotificationType, link string) error
	// ListMine mengambil notifikasi milik satu user (klaim JWT), terbaru dulu.
	ListMine(ctx context.Context, userID string, limit int) ([]domain.Notification, error)
	// MarkRead menandai satu notifikasi milik user sebagai dibaca.
	// rows==0 berarti ID asing/milik orang lain (tanpa oracle bedakan).
	MarkRead(ctx context.Context, id int64, userID string) error
}

type notificationRepo struct {
	db *sqlx.DB
}

func NewNotificationRepository(db *sqlx.DB) NotificationRepository {
	return &notificationRepo{db: db}
}

func (r *notificationRepo) NotifyAdmins(ctx context.Context, title, message string, notifType domain.NotificationType, link string, provinsiID, kabupatenID int) error {
	query := `
		INSERT INTO notifications (user_id, title, message, type, link, is_read, created_at)
		SELECT u.id, $1, $2, $3, $4, FALSE, CURRENT_TIMESTAMP
		FROM users u
		WHERE u.deleted_at IS NULL AND u.status = 'Aktif' AND (
			u.role IN ('SUPER_ADMIN', 'ADMIN_NASIONAL')
			OR (u.role = 'ADMIN_PROVINSI' AND u.provinsi_id = $5)
			OR (u.role = 'ADMIN_KABUPATEN' AND u.kabupaten_id = $6)
		)`
	var linkArg interface{}
	if strings.TrimSpace(link) == "" {
		linkArg = nil
	} else {
		linkArg = strings.TrimSpace(link)
	}
	_, err := r.db.ExecContext(ctx, query,
		strings.TrimSpace(title), strings.TrimSpace(message), string(notifType), linkArg,
		provinsiID, kabupatenID)
	return err
}

func (r *notificationRepo) NotifyUser(ctx context.Context, userID, title, message string, notifType domain.NotificationType, link string) error {
	var linkArg interface{}
	if strings.TrimSpace(link) == "" {
		linkArg = nil
	} else {
		linkArg = strings.TrimSpace(link)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications (user_id, title, message, type, link, is_read, created_at)
		 VALUES ($1, $2, $3, $4, $5, FALSE, CURRENT_TIMESTAMP)`,
		strings.TrimSpace(userID), strings.TrimSpace(title), strings.TrimSpace(message),
		string(notifType), linkArg)
	return err
}

func (r *notificationRepo) ListMine(ctx context.Context, userID string, limit int) ([]domain.Notification, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	items := make([]domain.Notification, 0)
	query := `SELECT id, user_id, title, message, type, link, is_read, created_at
		FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	if err := r.db.SelectContext(ctx, &items, query, strings.TrimSpace(userID), limit); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, id int64, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`,
		id, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
