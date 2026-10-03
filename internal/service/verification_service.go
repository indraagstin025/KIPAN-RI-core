package service

// VerificationService menangani verifikasi admin + KTA + NIK reveal (R3:
// pecahan dari god-service pendaftaran). Dependensi minimal: repo +
// anggota + audit + KTA.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

type VerificationService interface {
	ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) (*ApprovalResult, error)
	VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error)
	RevealNIK(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) (string, error)
}

// ApprovalResult adalah hasil proses approval. Kredensial awal TIDAK lagi
// dikembalikan ke admin — dikirim ke email anggota via antrian (outbox).
type ApprovalResult struct {
	NIA string `json:"nia,omitempty"`
}

// VerificationDeps adalah dependensi service verifikasi (R1: pola deps).
type VerificationDeps struct {
	Repo        repository.PendaftaranVerificationRepository
	AnggotaRepo repository.AnggotaRepository
	UserRepo    repository.UserAccountRepository
	AuditRepo   repository.AuditLogRepository
	KTASvc      KTAService
	NotifRepo   repository.NotificationRepository
	Mail        gateway.MailSender
	OutboxRepo  repository.EmailOutboxRepository
}

type verificationSvc struct {
	cfg         *config.Config
	repo        repository.PendaftaranVerificationRepository
	anggotaRepo repository.AnggotaRepository
	userRepo    repository.UserAccountRepository
	auditRepo   repository.AuditLogRepository
	ktaSvc      KTAService
	notifRepo   repository.NotificationRepository
	mail        gateway.MailSender
	outboxRepo  repository.EmailOutboxRepository
}

func NewVerificationService(cfg *config.Config, deps VerificationDeps) VerificationService {
	return &verificationSvc{
		cfg: cfg, repo: deps.Repo, anggotaRepo: deps.AnggotaRepo,
		userRepo:  deps.UserRepo,
		auditRepo: deps.AuditRepo, ktaSvc: deps.KTASvc, notifRepo: deps.NotifRepo,
		mail: deps.Mail, outboxRepo: deps.OutboxRepo,
	}
}

// ProcessApproval memvalidasi otorisasi + jurisdiction + transisi status,
// lalu mengeksekusi secara atomik beserta riwayat beraktor dan audit trail.
// actor WAJIB berasal dari JWT terverifikasi (RULES 6), bukan dari client.
//
// Aksi SETUJI sekaligus menerbitkan akun USER anggota (Batch 2): password
// awal acak dikembalikan SEKALI di ApprovalResult untuk diteruskan ke
// anggota — tidak pernah ditulis ke audit/log.
func (s *verificationSvc) ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) (*ApprovalResult, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, unavailable("pendaftaran")
	}
	if action == "" {
		return nil, domain.NewValidationError("Aksi verifikasi wajib dipilih")
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}
	// Batasan peran (selaras proyek lama + URD): Admin Provinsi boleh
	// MELIHAT antrean provinsinya, tetapi TIDAK BOLEH aksi verifikasi
	// (verifikasi/perbaikan/tolak/setujui = wewenang eksklusif
	// Kab/Kota di wilayahnya, plus Nasional/Super).
	if actor.Role == domain.RoleAdminProvinsi {
		return nil, domain.NewValidationError("Admin Provinsi tidak berwenang memverifikasi pendaftaran. Verifikasi adalah wewenang Admin Kabupaten/Kota.")
	}

	targetStatus, ok := mapStatusForAction(action)
	if !ok {
		return nil, domain.NewValidationError("Aksi tidak valid untuk proses pendaftaran")
	}
	if !domain.IsAllowedTransition(item.Status, targetStatus, action) {
		return nil, domain.NewValidationError("Transisi status tidak sah untuk aksi yang diminta")
	}

	note := strings.TrimSpace(catatan)
	// T6: perbaikan/penolakan WAJIB beralasan agar pendaftar tahu yang harus
	// diperbaiki. Ditegakkan di server, bukan hanya di frontend.
	if (action == domain.PendaftaranActionPerbaikan || action == domain.PendaftaranActionTolak) && note == "" {
		return nil, domain.NewValidationError("Catatan wajib diisi untuk permintaan perbaikan atau penolakan")
	}
	actorID, actorName, actorRole := actor.UserID, actor.Name, string(actor.Role)
	meta := fmt.Sprintf(`{"from":%q,"to":%q}`, string(item.Status), string(targetStatus))

	if action == domain.PendaftaranActionSetujui {
		if s.cfg == nil || strings.TrimSpace(s.cfg.Crypto.KTASigningKey) == "" {
			return nil, domain.NewValidationError("KTA_SIGNING_KEY belum dikonfigurasi")
		}
		member, err := s.repo.IssueMember(ctx, id, time.Now().Year(), s.cfg.Crypto.KTASigningKey)
		if err != nil {
			// Jalur heal: approve sebelumnya berhasil terbitkan anggota
			// tetapi gagal di PDF (fail-closed) — coba selesaikan PDF-nya
			// alih-alih gagal dengan "sudah memiliki anggota".
			var appErr *domain.AppError
			if errors.As(err, &appErr) && appErr.Code == 409 && s.anggotaRepo != nil {
				return s.healKTADocument(ctx, id, actor, audit, meta)
			}
			return nil, err
		}
		// PDF KTA server-side, fail-closed: gagal render/upload = approve
		// gagal, admin retry (idempoten via jalur heal di atas).
		if s.ktaSvc != nil {
			if _, err := s.ktaSvc.IssueKTADocument(ctx, member, member.KTAQRHashValue(), audit); err != nil {
				return nil, err
			}
		}
		// Terbitkan akun USER anggota (idempoten: email sudah ada = link).
		// Gagal di sini = admin retry (jalur heal melengkapi sisanya).
		userID, isNew, err := s.ensureMemberAccount(ctx, member, actor, audit)
		if err != nil {
			return nil, err
		}
		// Kredensial via antrian email (Opsi A): tautan set-password untuk
		// akun baru; notifikasi akun tertaut untuk akun yang sudah ada.
		if isNew {
			s.enqueueContent(ctx, domain.EmailOutboxSetPassword, item, &userID, item.Email,
				EmailContent{Subject: "Buat Kata Sandi Akun KIPAN", TextBody: "Buat kata sandi akun Anda melalui tautan pada email ini."})
		} else {
			s.enqueueContent(ctx, domain.EmailOutboxAkunTerhubung, item, nil, item.Email,
				AccountLinkedEmail(item.NamaLengkap, member.NIA, publicURLFrom(s.cfg)))
		}
		s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
			"pendaftaran", strconv.Itoa(id), string(action), &meta)
		s.notifyAdmins(ctx, "Pembaruan Status Verifikasi",
			"Pendaftaran "+item.NamaLengkap+" disetujui menjadi Anggota.",
			domain.NotifTypeVerifikasi, "#admin?page=verifikasi",
			item.ProvinsiID, item.KabupatenID)
		return &ApprovalResult{NIA: member.NIA}, nil
	}

	if err := s.repo.UpdateStatusWithHistory(ctx, id, targetStatus, string(action), &actorID, &actorName, &actorRole, note); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), string(action), &meta)
	s.notifyAdmins(ctx, "Pembaruan Status Verifikasi",
		"Pendaftaran "+item.NamaLengkap+" diperbarui menjadi "+string(targetStatus)+".",
		domain.NotifTypeVerifikasi, "#admin?page=verifikasi",
		item.ProvinsiID, item.KabupatenID)
	// Batch 3: PERBAIKAN & DITOLAK via antrian email (outbox).
	if targetStatus == domain.PendaftaranStatusPerbaikan || targetStatus == domain.PendaftaranStatusDitolak {
		jenis := domain.EmailOutboxStatusPerbaikan
		if targetStatus == domain.PendaftaranStatusDitolak {
			jenis = domain.EmailOutboxStatusDitolak
		}
		s.enqueueContent(ctx, jenis, item, nil, item.Email,
			StatusEmail(string(targetStatus), item.NamaLengkap, item.NomorPendaftaran, note, "", publicURLFrom(s.cfg)))
	}
	return &ApprovalResult{}, nil
}

// enqueueContent menulis satu baris antrian email (best-effort, non-fatal).
func (s *verificationSvc) enqueueContent(ctx context.Context, jenis domain.EmailOutboxKind, item *domain.Pendaftaran, userID *string, toEmail string, c EmailContent) {
	if s.outboxRepo == nil || strings.TrimSpace(toEmail) == "" {
		return
	}
	html := c.HTMLBody
	entry := &domain.EmailOutbox{
		Jenis: jenis, UserID: userID, ToEmail: toEmail,
		Subject: c.Subject, TextBody: c.TextBody, HTMLBody: &html,
	}
	if item != nil {
		id := item.ID
		entry.PendaftaranID = &id
		p, k := item.ProvinsiID, item.KabupatenID
		entry.ProvinsiID = &p
		entry.KabupatenID = &k
	}
	enqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.outboxRepo.Enqueue(enqCtx, entry); err != nil {
		log.Warn().Err(err).Str("jenis", string(jenis)).Msg("Gagal enqueue email outbox")
	}
}

// healKTADocument menyelesaikan PDF KTA untuk approve yang sebelumnya gagal
// di tengah jalan (anggota sudah terbit, PDF belum). Idempoten: bila PDF
// sudah ada, IssueKTADocument mengembalikan key lama. Akun USER ikut
// dilengkapi bila belum terhubung (percobaan pertama gagal setelah IssueMember).
func (s *verificationSvc) healKTADocument(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext, meta string) (*ApprovalResult, error) {
	if s.anggotaRepo == nil || s.ktaSvc == nil {
		return nil, domain.NewConflictError("Pendaftaran sudah memiliki anggota")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.AnggotaID == nil {
		return nil, domain.NewConflictError("Pendaftaran sudah memiliki anggota")
	}
	member, err := s.anggotaRepo.GetByID(ctx, *item.AnggotaID)
	if err != nil {
		return nil, err
	}
	if _, err := s.ktaSvc.IssueKTADocument(ctx, member, member.KTAQRHashValue(), audit); err != nil {
		return nil, err
	}
	userID, isNew, err := s.ensureMemberAccount(ctx, member, actor, audit)
	if err != nil {
		return nil, err
	}
	// Kredensial via antrian email (Opsi A).
	if isNew {
		s.enqueueContent(ctx, domain.EmailOutboxSetPassword, item, &userID, member.Email,
			EmailContent{Subject: "Buat Kata Sandi Akun KIPAN", TextBody: "Buat kata sandi akun Anda melalui tautan pada email ini."})
	} else {
		s.enqueueContent(ctx, domain.EmailOutboxAkunTerhubung, item, nil, member.Email,
			AccountLinkedEmail(member.NamaLengkap, member.NIA, publicURLFrom(s.cfg)))
	}
	actorID, actorName, actorRole := actor.UserID, actor.Name, string(actor.Role)
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), string(domain.PendaftaranActionSetujui), &meta)
	return &ApprovalResult{NIA: member.NIA}, nil
}

// ensureMemberAccount menerbitkan akun USER untuk anggota (Batch 2).
// Idempoten: email sudah terdaftar = hubungkan anggota ke akun existing.
// Mengembalikan (userID, akunBaru). Password acak dibuat HANYA untuk hash
// (Opsi A): plaintext TIDAK dikembalikan — login via tautan set-password.
func (s *verificationSvc) ensureMemberAccount(ctx context.Context, member *domain.Anggota, actor domain.ActorContext, audit domain.AuditContext) (string, bool, error) {
	if s.userRepo == nil || s.anggotaRepo == nil {
		return "", false, unavailable("akun user")
	}
	if member == nil || strings.TrimSpace(member.Email) == "" {
		return "", false, domain.NewValidationError("Email anggota tidak valid untuk penerbitan akun")
	}

	if existing, err := s.userRepo.GetByEmail(ctx, member.Email); err == nil && existing != nil {
		// #3(a): hanya tautkan ke akun ber-role USER. Email milik akun
		// admin/verifikator TIDAK boleh diam-diam menjadi pemilik data
		// anggota (pendaftar bisa menulis email orang lain) — minta admin
		// menyelesaikan manual (mis. perbaiki email pendaftar).
		if existing.Role != domain.RoleUser {
			return "", false, domain.NewConflictError(
				"Email pendaftar sudah dipakai akun non-anggota (" + string(existing.Role) +
					"). Perbaiki email pendaftaran sebelum menyetujui")
		}
		if err := s.anggotaRepo.SetUserID(ctx, member.ID, existing.ID); err != nil {
			return "", false, err
		}
		actorID := actor.UserID
		linkMeta := `{"event":"member_account_linked"}`
		s.auditEvent(ctx, audit, &actorID, actor.Name, string(actor.Role),
			"anggota", strconv.Itoa(member.ID), "LINK_USER", &linkMeta)
		return existing.ID, false, nil
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return "", false, err
	}

	password, err := generateMemberPassword(16)
	if err != nil {
		return "", false, fmt.Errorf("gagal membuat password awal: %w", err)
	}
	hash, err := argon2id.CreateHash(password, argon2Params)
	if err != nil {
		return "", false, fmt.Errorf("gagal hash password awal: %w", err)
	}
	// Tipe akun mengikuti jalur pendaftaran (KADER/PENGURUS); legacy
	// tanpa tipe dianggap KADER (selaras DEFAULT migrasi 000011).
	tipeUser := domain.UserTipe(member.Tipe)
	if tipeUser != domain.UserTipeKader && tipeUser != domain.UserTipePengurus {
		tipeUser = domain.UserTipeKader
	}
	user := &domain.User{
		ID:           uuid.NewString(),
		Email:        strings.TrimSpace(member.Email),
		PasswordHash: hash,
		Name:         member.NamaLengkap,
		Role:         domain.RoleUser,
		TipeUser:     tipeUser,
		Status:       domain.UserStatusAktif,
		ProvinsiID:   &member.ProvinsiID,
		KabupatenID:  &member.KabupatenID,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return "", false, err
	}
	if err := s.anggotaRepo.SetUserID(ctx, member.ID, user.ID); err != nil {
		return "", false, err
	}
	actorID := actor.UserID
	createMeta := `{"event":"member_account_created","role":"USER","tipe":"` + string(tipeUser) + `"}`
	s.auditEvent(ctx, audit, &actorID, actor.Name, string(actor.Role),
		"users", user.ID, "CREATE", &createMeta)
	return user.ID, true, nil
}

// memberPasswordAlphabet aman untuk shell/.env (tanpa kutip, backslash,
// dolar, atau spasi) — selaras generator password seeder.
const memberPasswordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#%^*-_=+?"

// generateMemberPassword membuat password awal acak (crypto/rand) dengan
// tiap kelas karakter (kecil, besar, digit, simbol) minimal satu.
func generateMemberPassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}
	classes := []string{
		"abcdefghijkmnopqrstuvwxyz",
		"ABCDEFGHJKLMNPQRSTUVWXYZ",
		"23456789",
		"!@#%^*-_=+?",
	}
	out := make([]byte, 0, length)
	for _, class := range classes {
		idx, err := randIntN(len(class))
		if err != nil {
			return "", err
		}
		out = append(out, class[idx])
	}
	for len(out) < length {
		idx, err := randIntN(len(memberPasswordAlphabet))
		if err != nil {
			return "", err
		}
		out = append(out, memberPasswordAlphabet[idx])
	}
	for i := len(out) - 1; i > 0; i-- {
		j, err := randIntN(i + 1)
		if err != nil {
			return "", err
		}
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

// randIntN mengembalikan angka acak [0, max) dari crypto/rand.
func randIntN(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// VerifyKTA memverifikasi keaslian KTA secara kriptografis (RULES 20).
// NIA tak dikenal dan signature salah menghasilkan verdict valid=false yang
// sama (tanpa oracle). Hanya input kosong yang ditolak sebagai 422.
func (s *verificationSvc) VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error) {
	code := strings.TrimSpace(nia)
	signature := strings.TrimSpace(sig)
	if code == "" || len(code) > 50 || len(signature) > 128 {
		return nil, domain.NewValidationError("Parameter verifikasi KTA tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	member, err := s.anggotaRepo.GetByNIA(ctx, code)
	if err != nil {
		// NIA tak dikenal = verdict tidak valid (bukan 404, anti oracle).
		return &domain.KTAVerificationResponse{NIA: code, Valid: false}, nil
	}
	keys := ktaVerifyKeys(s.cfg)
	if len(keys) == 0 {
		return nil, domain.NewValidationError("KTA_SIGNING_KEY belum dikonfigurasi")
	}
	// Rotasi: coba kunci aktif dulu, lalu kunci sebelumnya. Kartu lama yang
	// ditandatangani kunci prev tetap valid tanpa migrasi ulang.
	valid := false
	for _, k := range keys {
		if err := crypto.VerifyKTASignature(
			member.NIA,
			member.TanggalAngkat.Format("2006-01-02"),
			member.ID,
			signature,
			k,
		); err == nil {
			valid = true
			break
		}
	}
	if !valid {
		return &domain.KTAVerificationResponse{NIA: member.NIA, Valid: false}, nil
	}
	tgl := member.TanggalAngkat
	return &domain.KTAVerificationResponse{
		NIA:           member.NIA,
		Valid:         true,
		NamaLengkap:   member.NamaLengkap,
		Status:        string(member.Status),
		TanggalAngkat: &tgl,
	}, nil
}

// RevealNIK mendekripsi NIK khusus untuk admin verifikator dalam yurisdiksinya.
// Setiap pembukaan dicatat (actor, IP, request-ID) agar dapat diaudit (RULES 12, 21).
func (s *verificationSvc) RevealNIK(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) (string, error) {
	if id <= 0 {
		return "", domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return "", unavailable("pendaftaran")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return "", domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}
	key, err := aesKey(s.cfg)
	if err != nil {
		return "", err
	}
	nik, err := crypto.DecryptAESGCM(item.NIKEncrypted, key)
	if err != nil {
		return "", domain.NewValidationError("Data NIK tidak dapat dibuka")
	}
	actorID, actorName, actorRole := actor.UserID, actor.Name, string(actor.Role)
	meta := `{"event":"nik_reveal"}`
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), "NIK_REVEAL", &meta)
	return nik, nil
}

func mapStatusForAction(action domain.PendaftaranApprovalAction) (domain.PendaftaranStatus, bool) {
	switch action {
	case domain.PendaftaranActionVerifikasi:
		return domain.PendaftaranStatusDiverifikasi, true
	case domain.PendaftaranActionPerbaikan:
		return domain.PendaftaranStatusPerbaikan, true
	case domain.PendaftaranActionTolak:
		return domain.PendaftaranStatusDitolak, true
	case domain.PendaftaranActionSetujui:
		return domain.PendaftaranStatusDisetujui, true
	default:
		return "", false
	}
}

// ktaVerifyKeys mengembalikan kunci verifikasi KTA: aktif dulu, lalu kunci
// rotasi sebelumnya (verify-only). Urutan penting untuk short-circuit.
func ktaVerifyKeys(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if k := strings.TrimSpace(cfg.Crypto.KTASigningKey); k != "" {
		keys = append(keys, k)
	}
	if k := strings.TrimSpace(cfg.Crypto.KTASigningKeyPrev); k != "" {
		keys = append(keys, k)
	}
	return keys
}

// notifyAdmins menyebar notifikasi ke admin berhak secara best-effort.
func (s *verificationSvc) notifyAdmins(ctx context.Context, title, message string, notifType domain.NotificationType, link string, provID, kabID int) {
	if s.notifRepo == nil {
		return
	}
	if err := s.notifRepo.NotifyAdmins(ctx, title, message, notifType, link, provID, kabID); err != nil {
		log.Warn().Err(err).Msg("gagal menyebar notifikasi verifikasi")
	}
}

// auditEvent mendelegasikan ke writeAudit terpusat (R2).
func (s *verificationSvc) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	writeAudit(ctx, s.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}
