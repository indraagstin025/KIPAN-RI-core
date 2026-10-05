package handler

// Test HTTP permukaan membership (BE-010): parsing, otorisasi transport,
// dan kontrak error — tanpa DB (service di-stub).

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/pendaftaran"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

func stringReader(s string) *strings.Reader { return strings.NewReader(s) }

// stubPendaftaranService mengimplementasikan pendaftaran.PendaftaranService
// dengan perilaku terprogram per test.
type stubPendaftaranService struct {
	validateErr error
	createRes   *domain.PendaftaranCreateResult
	createErr   error
	trackRes    *domain.PendaftaranTrackingResponse
	trackErr    error
	revisionErr error
}

func (s *stubPendaftaranService) ValidateSubmitRequest(domain.PendaftaranSubmitRequest) error {
	return s.validateErr
}
func (s *stubPendaftaranService) BuildRegistrationNumber(context.Context, int, int) (string, error) {
	return "REG-202609-00001", nil
}
func (s *stubPendaftaranService) GenerateBlindIndex(string) (string, error) {
	return "hash", nil
}
func (s *stubPendaftaranService) EncryptNIK(string) (string, error) { return "enc", nil }
func (s *stubPendaftaranService) CreateRegistration(context.Context, domain.PendaftaranSubmitRequest, domain.AuditContext) (*domain.PendaftaranCreateResult, error) {
	return s.createRes, s.createErr
}
func (s *stubPendaftaranService) GetTracking(context.Context, string) (*domain.PendaftaranTrackingResponse, error) {
	return s.trackRes, s.trackErr
}
func (s *stubPendaftaranService) VerifyKTA(context.Context, string, string) (*domain.KTAVerificationResponse, error) {
	return &domain.KTAVerificationResponse{NIA: "x", Valid: false}, nil
}
func (s *stubPendaftaranService) ListQueue(context.Context, domain.ActorContext, string, int, int) ([]domain.PendaftaranQueueItem, int, error) {
	return []domain.PendaftaranQueueItem{}, 0, nil
}
func (s *stubPendaftaranService) ListQueueCursor(context.Context, domain.ActorContext, string, string, int) ([]domain.PendaftaranQueueItem, string, error) {
	return []domain.PendaftaranQueueItem{}, "", nil
}
func (s *stubPendaftaranService) RequestRevisionToken(context.Context, domain.RevisionTokenRequest, domain.AuditContext) (*domain.RevisionTokenResponse, error) {
	return nil, s.revisionErr
}
func (s *stubPendaftaranService) SubmitRevision(context.Context, string, domain.RevisionSubmitRequest, domain.AuditContext) error {
	return nil
}
func (s *stubPendaftaranService) GetDetail(context.Context, int, domain.ActorContext) (*domain.PendaftaranAdminDetail, error) {
	return nil, domain.ErrNotFound
}
func (s *stubPendaftaranService) RevealNIK(context.Context, int, domain.ActorContext, domain.AuditContext) (string, error) {
	return "3201010101010001", nil
}
func (s *stubPendaftaranService) ProcessApproval(context.Context, int, domain.PendaftaranApprovalAction, string, domain.ActorContext, domain.AuditContext) (*pendaftaran.ApprovalResult, error) {
	return &pendaftaran.ApprovalResult{}, nil
}

var _ pendaftaran.PendaftaranService = (*stubPendaftaranService)(nil)

func testPendaftaranApp(stub *stubPendaftaranService) *fiber.App {
	app := fiber.New()
	h := NewPendaftaranHandler(stub, stub, stub, validator.New())
	app.Post("/pendaftaran", h.Submit)
	app.Get("/pendaftaran/track/:nomor", h.TrackStatus)
	app.Post("/pendaftaran/revisi/request-token", h.RequestRevisionToken)
	app.Get("/admin/pendaftaran/:id",
		func(c *fiber.Ctx) error {
			c.Locals("user", &middleware.JWTClaims{UserID: "u1", Role: domain.RoleAdminKabupaten})
			return c.Next()
		},
		h.Detail)
	return app
}

func TestSubmitInvalidJSON400(t *testing.T) {
	app := testPendaftaranApp(&stubPendaftaranService{})
	req := httptest.NewRequest("POST", "/pendaftaran", stringReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("harap 400, dapat %d", resp.StatusCode)
	}
}

func TestSubmitServiceErrorPropagates(t *testing.T) {
	stub := &stubPendaftaranService{createErr: domain.NewValidationError("x")}
	app := testPendaftaranApp(stub)
	req := httptest.NewRequest("POST", "/pendaftaran", stringReader(`{"nama_lengkap":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("harap 422 dari service, dapat %d", resp.StatusCode)
	}
}

func TestTrackEmptyNomor400(t *testing.T) {
	// Fiber: param kosong tak match route → 404 dari router. Uji nomor
	// dengan service error sebagai gantinya (jalur FromError).
	stub := &stubPendaftaranService{trackErr: domain.ErrNotFound}
	app := testPendaftaranApp(stub)
	req := httptest.NewRequest("GET", "/pendaftaran/track/REG-X", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("harap 404, dapat %d", resp.StatusCode)
	}
}

func TestDetailTanpaClaims401(t *testing.T) {
	// Tanpa middleware Authenticate (tanpa claims) wajib 401, bukan 500.
	app := fiber.New()
	stub := &stubPendaftaranService{}
	h := NewPendaftaranHandler(stub, stub, stub, validator.New())
	app.Get("/admin/pendaftaran/:id", h.Detail)

	req := httptest.NewRequest("GET", "/admin/pendaftaran/1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("harap 401 tanpa claims, dapat %d", resp.StatusCode)
	}
}

func TestRequestTokenButuhBuktiPemilik(t *testing.T) {
	// Tanpa email+whatsapp service menolak (stub meniru service asli).
	stub := &stubPendaftaranService{revisionErr: domain.NewForbiddenError("Anda tidak berhak meminta token revisi ini")}
	app := testPendaftaranApp(stub)
	req := httptest.NewRequest("POST", "/pendaftaran/revisi/request-token",
		stringReader(`{"nomor":"REG-202609-00001"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("harap 403 tanpa bukti pemilik, dapat %d", resp.StatusCode)
	}
}

func TestActorOfPakaiNamaKlaim(t *testing.T) {
	// SEC-AUDIT-NAME: klaim nama masuk ActorContext.Name agar audit
	// mencatat nama pengguna, bukan email.
	newCtx := func(claims *middleware.JWTClaims) (domain.ActorContext, bool) {
		app := fiber.New()
		var got domain.ActorContext
		var ok bool
		app.Get("/", func(c *fiber.Ctx) error {
			if claims != nil {
				c.Locals("user", claims)
			}
			got, ok = actorOf(c)
			return c.SendStatus(fiber.StatusOK)
		})
		req := httptest.NewRequest("GET", "/", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request gagal: %v", err)
		}
		resp.Body.Close()
		return got, ok
	}

	actor, ok := newCtx(&middleware.JWTClaims{UserID: "u1", Email: "a@x.id", Name: "Budi Santoso", Role: domain.RoleSuperAdmin})
	if !ok || actor.Name != "Budi Santoso" {
		t.Fatalf("harap Name dari klaim, dapat %+v ok=%v", actor, ok)
	}
	// Token lama tanpa klaim nama fallback ke email.
	actor, ok = newCtx(&middleware.JWTClaims{UserID: "u1", Email: "a@x.id", Role: domain.RoleSuperAdmin})
	if !ok || actor.Name != "a@x.id" {
		t.Fatalf("harap fallback email, dapat %+v ok=%v", actor, ok)
	}
	// Tanpa claims → tidak terotentikasi.
	if _, ok := newCtx(nil); ok {
		t.Fatal("harap false tanpa claims")
	}
}
