package platform

// role_service.go — katalog read-only role & wewenang (TDD §3.3), sumber data
// dari registry domain (capabilities.go). Tidak ada mutasi.

import (
	"context"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// RoleService menyajikan katalog role + matriks wewenang.
type RoleService interface {
	Catalog(ctx context.Context) *domain.RoleCatalog
}

type roleService struct{}

// NewRoleService membangun service katalog role.
func NewRoleService() RoleService {
	return &roleService{}
}

// Catalog mengembalikan katalog role beserta capability matrix.
func (s *roleService) Catalog(_ context.Context) *domain.RoleCatalog {
	return &domain.RoleCatalog{
		Roles:        domain.AllRoles(),
		Capabilities: domain.AllCapabilities(),
	}
}
