package domain

// ============================================================
// MANAJEMEN PENGGUNA (ADMIN) — Super Admin
// ============================================================

// AdminUserCounts hitungan akun admin per role (untuk kartu halaman).
type AdminUserCounts struct {
	Total     int `db:"total" json:"total"`
	Super     int `db:"super" json:"super"`
	Nasional  int `db:"nasional" json:"nasional"`
	Provinsi  int `db:"provinsi" json:"provinsi"`
	Kabupaten int `db:"kabupaten" json:"kabupaten"`
}

// UserCreateRequest payload pembuatan akun admin.
type UserCreateRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	ProvinsiID  *int   `json:"provinsi_id"`
	KabupatenID *int   `json:"kabupaten_id"`
	Status      string `json:"status"`
}

// UserUpdateRequest payload perubahan akun admin. Password kosong = tetap;
// ResetPassword=true meminta password baru auto-generate.
type UserUpdateRequest struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	ProvinsiID    *int   `json:"provinsi_id"`
	KabupatenID   *int   `json:"kabupaten_id"`
	Status        string `json:"status"`
	Password      string `json:"password"`
	ResetPassword bool   `json:"reset_password"`
}
