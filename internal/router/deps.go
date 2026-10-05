package router

// deps.go adalah composition root: membangun seluruh repository, service,
// handler, dan worker. Tujuannya memisahkan wiring dari layer transport
// (handler) sehingga handler tetap murni HTTP.

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/infra"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/wilayah"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// deps menampung seluruh handler dan middleware yang dibutuhkan registrasi rute.
// Diisi sekali oleh newDeps.
type deps struct {
	authMiddleware      *middleware.AuthMiddleware
	authHandler         *handler.AuthHandler
	pwResetHandler      *handler.PasswordResetHandler
	pendaftaranHandler  *handler.PendaftaranHandler
	otpHandler          *handler.OTPHandler
	storageHandler      *handler.StorageHandler
	wilayahHandler      *handler.WilayahHandler
	wilayahAdminHandler *handler.WilayahAdminHandler
	outboxHandler       *handler.OutboxHandler
	ktaHandler          *handler.KTAHandler
	anggotaHandler      *handler.AnggotaHandler
	notifHandler        *handler.NotificationHandler
	userAdminHandler    *handler.UserAdminHandler
	kepengurusanHandler *handler.KepengurusanHandler
	dashboardHandler    *handler.DashboardHandler
	laporanHandler      *handler.LaporanHandler
	auditHandler        *handler.AuditHandler
	roleHandler         *handler.RoleHandler
	organisasiHandler   *handler.OrganisasiHandler
	backupHandler       *handler.BackupHandler
}

// newDeps membangun semua dependensi aplikasi (repo → service → handler) dan
// menjalankan worker antrian email. Mengembalikan struct handler siap pakai.
func newDeps(cfg *config.Config, db *sqlx.DB, rdb *redis.Client, val *validator.CustomValidator) *deps {
	// --- Repository ---
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	pendaftaranRepo := repository.NewPendaftaranRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	emailOutboxRepo := repository.NewEmailOutboxRepository(db)
	anggotaRepo := repository.NewAnggotaRepository(db)
	wilayahRepo := repository.NewWilayahRepository(db)

	// --- Infrastruktur eksternal ---
	storageService := wireStorageService(cfg, auditRepo, repository.NewDocumentRepository(db))
	waGateway := infra.WAGateway(cfg)
	mailSender := infra.MailSender(cfg)

	// --- Service ---
	authService := service.NewAuthService(cfg, service.AuthDeps{
		UserRepo: userRepo, RDB: rdb, AuditRepo: auditRepo,
	})
	ktaSvc := service.NewKTAService(cfg, service.KTADeps{
		AnggotaRepo: anggotaRepo, DocStore: storageService, AuditRepo: auditRepo,
	})
	otpSvc := service.NewOTPService(cfg, service.OtpDeps{
		RDB: rdb, Gateway: waGateway,
	})
	pendaftaranService := service.NewPendaftaranService(cfg, service.PendaftaranDeps{
		Repo: pendaftaranRepo, AnggotaRepo: anggotaRepo, AuditRepo: auditRepo,
		StorageSvc: storageService, WilayahRepo: wilayahRepo, NotifRepo: notifRepo,
		OTPSvc: otpSvc, WAGateway: waGateway, ListRepo: repository.NewListKeysetRepository(db),
		OutboxRepo: emailOutboxRepo,
	})
	revisionSvc := service.NewRevisionService(cfg, service.RevisionDeps{
		Repo: pendaftaranRepo, StorageSvc: storageService, AuditRepo: auditRepo,
		Mail: mailSender,
	})
	verificationSvc := service.NewVerificationService(cfg, service.VerificationDeps{
		Repo: pendaftaranRepo, AnggotaRepo: anggotaRepo, UserRepo: userRepo,
		AuditRepo: auditRepo, KTASvc: ktaSvc,
		NotifRepo: notifRepo, Mail: mailSender, OutboxRepo: emailOutboxRepo,
	})
	pwResetSvc := service.NewPasswordResetService(cfg, service.PasswordResetDeps{
		UserRepo: userRepo, RDB: rdb, Mail: mailSender, AuditRepo: auditRepo,
	})
	anggotaService := service.NewAnggotaService(cfg, service.AnggotaDeps{
		AnggotaRepo: anggotaRepo, WilayahRepo: wilayahRepo,
		UserRepo: userRepo, AuditRepo: auditRepo,
		ListRepo:        repository.NewListKeysetRepository(db),
		OutboxRepo:      emailOutboxRepo,
		PendaftaranRepo: pendaftaranRepo,
		PengurusRepo:    repository.NewPengurusRepository(db),
	})
	wilayahService := wilayah.NewWilayahService(wilayahRepo)
	wilayahAdminService := wilayah.NewWilayahAdminService(repository.NewWilayahAdminRepository(db), auditRepo)
	userAdminService := service.NewUserAdminService(service.UserAdminDeps{
		UserRepo: userRepo, AdminRepo: repository.NewUserAdminRepository(db),
		WilayahRepo: wilayahRepo, AuditRepo: auditRepo,
	})
	// Worker email dipakai untuk pengiriman manual sinkron (tombol admin);
	// pengiriman otomatis tetap di biner cmd/worker.
	emailWorker := service.NewEmailWorker(cfg, emailOutboxRepo, mailSender, rdb, userRepo, anggotaRepo, notifRepo)
	outboxService := service.NewOutboxService(emailOutboxRepo, emailWorker)
	notifService := service.NewNotificationService(cfg, notifRepo)
	dashboardService := service.NewDashboardService(repository.NewDashboardRepository(db))
	laporanService := service.NewLaporanService(cfg, repository.NewLaporanRepository(db))
	auditService := service.NewAuditService(auditRepo)
	roleService := service.NewRoleService()
	organisasiService := service.NewOrganisasiService(repository.NewOrganisasiRepository(db), auditRepo)
	backupService := service.NewBackupService(cfg, repository.NewBackupRepository(db), storageService, auditRepo)
	kepengurusanService := service.NewKepengurusanService(cfg, service.KepengurusanDeps{
		JabatanRepo:  repository.NewJabatanRepository(db),
		SKRepo:       repository.NewSKRepository(db),
		PengurusRepo: repository.NewPengurusRepository(db),
		AnggotaRepo:  anggotaRepo, UserRepo: userRepo, AuditRepo: auditRepo,
		WilayahRepo: wilayahRepo, OutboxRepo: emailOutboxRepo,
	})

	// Catatan: worker antrian email TIDAK dijalankan di proses API. Worker
	// berjalan di biner terpisah (cmd/worker) agar tidak dobel saat API
	// di-scale >1 instance.

	// --- Handler ---
	secureCookie := cfg.App.Env != "development"
	return &deps{
		authMiddleware:      middleware.NewAuthMiddleware(cfg.Auth.AccessTokenSecret, rdb),
		authHandler:         handler.NewAuthHandler(authService, val, cfg.Auth.RefreshTokenTTL, secureCookie, cfg.Auth.CookieSameSite, cfg.Auth.CookiePath, cfg.Auth.CookieDomain),
		pwResetHandler:      handler.NewPasswordResetHandler(pwResetSvc, val),
		pendaftaranHandler:  handler.NewPendaftaranHandler(pendaftaranService, revisionSvc, verificationSvc, val),
		otpHandler:          handler.NewOTPHandler(otpSvc),
		storageHandler:      handler.NewStorageHandler(storageService, val),
		wilayahHandler:      handler.NewWilayahHandler(wilayahService),
		wilayahAdminHandler: handler.NewWilayahAdminHandler(wilayahAdminService),
		outboxHandler:       handler.NewOutboxHandler(outboxService),
		ktaHandler:          handler.NewKTAHandler(ktaSvc),
		anggotaHandler:      handler.NewAnggotaHandler(anggotaService),
		notifHandler:        handler.NewNotificationHandler(notifService),
		userAdminHandler:    handler.NewUserAdminHandler(userAdminService),
		kepengurusanHandler: handler.NewKepengurusanHandler(kepengurusanService),
		dashboardHandler:    handler.NewDashboardHandler(dashboardService),
		laporanHandler:      handler.NewLaporanHandler(laporanService),
		auditHandler:        handler.NewAuditHandler(auditService),
		roleHandler:         handler.NewRoleHandler(roleService),
		organisasiHandler:   handler.NewOrganisasiHandler(organisasiService),
		backupHandler:       handler.NewBackupHandler(backupService),
	}
}
