package domain

import "testing"

// TestCapabilityRegistryInvariants memastikan registry konsisten: key/label
// terisi, tidak ada duplikat, dan setiap capability punya role valid.
func TestCapabilityRegistryInvariants(t *testing.T) {
	seen := map[Capability]bool{}
	for _, info := range capabilityRegistry {
		if info.Key == "" || info.Label == "" || info.Description == "" {
			t.Fatalf("capability tidak lengkap: %+v", info)
		}
		if seen[info.Key] {
			t.Fatalf("capability duplikat: %s", info.Key)
		}
		seen[info.Key] = true
		if len(info.Roles) == 0 {
			t.Fatalf("capability %s tanpa role", info.Key)
		}
		for _, r := range info.Roles {
			switch r {
			case RoleSuperAdmin, RoleAdminNasional, RoleAdminProvinsi, RoleAdminKabupaten:
			default:
				t.Fatalf("role tidak valid pada %s: %q", info.Key, r)
			}
		}
	}
}

// TestSuperHasAllCapabilities memastikan Super Admin tidak pernah kehilangan
// wewenang (invariant penting).
func TestSuperHasAllCapabilities(t *testing.T) {
	for _, info := range capabilityRegistry {
		if !HasCapability(RoleSuperAdmin, info.Key) {
			t.Fatalf("Super Admin kurang capability %s", info.Key)
		}
	}
}

// TestUserHasNoCapability memastikan akun anggota tidak punya wewenang admin.
func TestUserHasNoCapability(t *testing.T) {
	for _, info := range capabilityRegistry {
		if HasCapability(RoleUser, info.Key) {
			t.Fatalf("USER tidak boleh memiliki capability %s", info.Key)
		}
	}
}

// TestCapabilitySpotChecks mengunci sel-sel matriks yang penting.
func TestCapabilitySpotChecks(t *testing.T) {
	if HasCapability(RoleAdminNasional, CapManageUsers) {
		t.Fatal("manage_users harus Super-only")
	}
	if !HasCapability(RoleAdminKabupaten, CapVerifyPendaftaran) {
		t.Fatal("Kabupaten harus bisa verifikasi pendaftaran")
	}
	if HasCapability(RoleAdminProvinsi, CapVerifyPendaftaran) {
		t.Fatal("Provinsi tidak boleh verifikasi pendaftaran")
	}
	if !HasCapability(RoleAdminNasional, CapManageWilayah) {
		t.Fatal("Nasional harus bisa kelola wilayah")
	}
	if HasCapability(RoleAdminProvinsi, CapManageWilayah) {
		t.Fatal("Provinsi tidak boleh kelola wilayah")
	}
	if HasCapability(RoleSuperAdmin, Capability("tidak-ada")) {
		t.Fatal("capability tak dikenal harus false")
	}
}

// TestCapabilityAccessors memastikan aksesor mengembalikan salinan & lookup benar.
func TestCapabilityAccessors(t *testing.T) {
	if !KnownCapability(CapManageUsers) || KnownCapability(Capability("x")) {
		t.Fatal("KnownCapability salah")
	}
	roles := RolesForCapability(CapManageUsers)
	if len(roles) != 1 || roles[0] != RoleSuperAdmin {
		t.Fatalf("RolesForCapability salah: %v", roles)
	}
	roles[0] = RoleUser // mutasi salinan
	if RolesForCapability(CapManageUsers)[0] != RoleSuperAdmin {
		t.Fatal("registry ter-mutasi lewat salinan")
	}
	if len(AllCapabilities()) != len(capabilityRegistry) {
		t.Fatal("AllCapabilities tidak lengkap")
	}
	// Kabupaten = 4 role admin semua; cek ada CapVerifyPendaftaran.
	var found bool
	for _, c := range CapabilitiesForRole(RoleAdminKabupaten) {
		if c == CapVerifyPendaftaran {
			found = true
		}
	}
	if !found {
		t.Fatal("CapabilitiesForRole(Kabupaten) harus memuat verify_pendaftaran")
	}
}
