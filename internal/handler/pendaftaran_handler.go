package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// PendaftaranHandler menangani HTTP transport untuk pendaftaran membership.
// Thin-handler (RULES 4): hanya parse request, ambil identitas server-side,
// panggil service, format response. Seluruh akses data lewat service.
type PendaftaranHandler struct {
	service         service.PendaftaranService
	revisionSvc     service.RevisionService
	verificationSvc service.VerificationService
	validator       *validator.CustomValidator
}

func NewPendaftaranHandler(
	service service.PendaftaranService,
	revisionSvc service.RevisionService,
	verificationSvc service.VerificationService,
	validator *validator.CustomValidator,
) *PendaftaranHandler {
	return &PendaftaranHandler{
		service: service, revisionSvc: revisionSvc,
		verificationSvc: verificationSvc, validator: validator,
	}
}

// actorOf membangun identitas server-side dari JWT terverifikasi (RULES 6).
// Kembalikan false bila tidak ada claims (route admin wajib Authenticate).
// Name memakai klaim nama; token terbitan lama (tanpa klaim nama) fallback
// ke email agar audit tetap terisi identifier (SEC-AUDIT-NAME).
func actorOf(c *fiber.Ctx) (domain.ActorContext, bool) {
	claims := middleware.GetUser(c)
	if claims == nil {
		return domain.ActorContext{}, false
	}
	name := claims.Name
	if name == "" {
		name = claims.Email
	}
	return domain.ActorContext{
		UserID:      claims.UserID,
		Name:        name,
		Role:        claims.Role,
		ProvinsiID:  claims.ProvinsiID,
		KabupatenID: claims.KabupatenID,
	}, true
}

func (h *PendaftaranHandler) Submit(c *fiber.Ctx) error {
	var req domain.PendaftaranSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	if err := h.service.ValidateSubmitRequest(req); err != nil {
		return response.FromError(c, err)
	}
	result, err := h.service.CreateRegistration(c.Context(), req, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Pendaftaran berhasil dikirim", result)
}

func (h *PendaftaranHandler) TrackStatus(c *fiber.Ctx) error {
	nomor := strings.TrimSpace(c.Params("nomor"))
	if nomor == "" {
		return response.BadRequest(c, "Nomor pendaftaran wajib diisi")
	}
	item, err := h.service.GetTracking(c.Context(), nomor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status pendaftaran ditemukan", item)
}

func (h *PendaftaranHandler) Detail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	item, err := h.service.GetDetail(c.Context(), id, actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Detail pendaftaran", item)
}

// RevealNIK membuka NIK terdekripsi untuk verifikator (1.4.2).
// Akses dicatat di audit trail oleh service.
func (h *PendaftaranHandler) RevealNIK(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	nik, err := h.verificationSvc.RevealNIK(c.Context(), id, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "NIK berhasil dibuka (tercatat di audit)", fiber.Map{"nik": nik})
}

func (h *PendaftaranHandler) ListQueue(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if c.Query("paginate") == "cursor" {
		items, next, err := h.service.ListQueueCursor(c.Context(), actor, c.Query("status"), c.Query("cursor"), c.QueryInt("limit", 25))
		if err != nil {
			return response.FromError(c, err)
		}
		return response.Paginated(c, "Daftar antrean pendaftaran", items, fiber.Map{
			"per_page":    c.QueryInt("limit", 25),
			"with_total":  false,
			"next_cursor": next,
		})
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	status := c.Query("status")
	items, total, err := h.service.ListQueue(c.Context(), actor, status, page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return response.Paginated(c, "Daftar antrean pendaftaran", items, fiber.Map{
		"page":        page,
		"per_page":    limit,
		"total":       total,
		"total_pages": pages,
	})
}

// RequestRevisionToken menerbitkan token revisi untuk status PERBAIKAN.
// Wajib bukti pemilik (email DAN whatsapp terdaftar — BE-001).
// Batch 3 (Opsi B): token dikirim ke EMAIL terdaftar dan TIDAK lagi
// dikembalikan di respons API.
func (h *PendaftaranHandler) RequestRevisionToken(c *fiber.Ctx) error {
	var req domain.RevisionTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	res, err := h.revisionSvc.RequestRevisionToken(c.Context(), req, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c,
		"Token revisi dikirim ke email terdaftar (berlaku 24 jam, sekali pakai)",
		fiber.Map{"expires_at": res.ExpiresAt})
}

// SubmitRevision memproses revisi mandiri applicant bertoken.
func (h *PendaftaranHandler) SubmitRevision(c *fiber.Ctx) error {
	nomor := strings.TrimSpace(c.Params("nomor"))
	if nomor == "" {
		return response.BadRequest(c, "Nomor pendaftaran wajib diisi")
	}
	var req domain.RevisionSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if err := h.revisionSvc.SubmitRevision(c.Context(), nomor, req, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Revisi berhasil dikirim, status kembali DRAFT", nil)
}

func (h *PendaftaranHandler) Verify(c *fiber.Ctx) error {
	return h.processApproval(c, domain.PendaftaranActionVerifikasi, "Verifikasi berhasil diproses")
}

func (h *PendaftaranHandler) RequestRevision(c *fiber.Ctx) error {
	return h.processApproval(c, domain.PendaftaranActionPerbaikan, "Permintaan revisi dikirim")
}

func (h *PendaftaranHandler) Reject(c *fiber.Ctx) error {
	return h.processApproval(c, domain.PendaftaranActionTolak, "Pendaftaran ditolak")
}

func (h *PendaftaranHandler) Approve(c *fiber.Ctx) error {
	return h.processApproval(c, domain.PendaftaranActionSetujui, "Pendaftaran disetujui")
}

func (h *PendaftaranHandler) processApproval(c *fiber.Ctx, action domain.PendaftaranApprovalAction, successMsg string) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var payload struct {
		Catatan string `json:"catatan"`
	}
	_ = c.BodyParser(&payload)
	res, err := h.verificationSvc.ProcessApproval(c.Context(), id, action, payload.Catatan, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	// Kredensial dikirim ke email anggota via antrian (tautan set-password);
	// tidak ada password di respons admin.
	var data interface{}
	if action == domain.PendaftaranActionSetujui && res != nil {
		data = fiber.Map{"nia": res.NIA}
	}
	return response.Success(c, successMsg, data)
}

func (h *PendaftaranHandler) VerifyKTA(c *fiber.Ctx) error {
	nia := strings.TrimSpace(c.Params("nia"))
	sig := strings.TrimSpace(c.Query("sig"))
	if nia == "" {
		return response.BadRequest(c, "NIA wajib diisi")
	}
	result, err := h.verificationSvc.VerifyKTA(c.Context(), nia, sig)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Hasil verifikasi KTA", result)
}
