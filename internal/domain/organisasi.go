package domain

import "time"

// OrganisasiProfile adalah profil organisasi (tabel 1-baris).
type OrganisasiProfile struct {
	ID        int       `db:"id" json:"id"`
	Nama      string    `db:"nama" json:"nama"`
	Singkatan string    `db:"singkatan" json:"singkatan"`
	Deskripsi string    `db:"deskripsi" json:"deskripsi"`
	Visi      string    `db:"visi" json:"visi"`
	Misi      string    `db:"misi" json:"misi"`
	Alamat    string    `db:"alamat" json:"alamat"`
	Email     string    `db:"email" json:"email"`
	Telepon   string    `db:"telepon" json:"telepon"`
	Whatsapp  string    `db:"whatsapp" json:"whatsapp"`
	Website   string    `db:"website" json:"website"`
	Instagram string    `db:"instagram" json:"instagram"`
	Facebook  string    `db:"facebook" json:"facebook"`
	Youtube   string    `db:"youtube" json:"youtube"`
	Tiktok    string    `db:"tiktok" json:"tiktok"`
	LogoURL   string    `db:"logo_url" json:"logo_url"`
	UpdatedBy *string   `db:"updated_by" json:"updated_by,omitempty"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// OrganisasiUpdateRequest payload sunting profil organisasi (Super Admin).
type OrganisasiUpdateRequest struct {
	Nama      string `json:"nama"`
	Singkatan string `json:"singkatan"`
	Deskripsi string `json:"deskripsi"`
	Visi      string `json:"visi"`
	Misi      string `json:"misi"`
	Alamat    string `json:"alamat"`
	Email     string `json:"email"`
	Telepon   string `json:"telepon"`
	Whatsapp  string `json:"whatsapp"`
	Website   string `json:"website"`
	Instagram string `json:"instagram"`
	Facebook  string `json:"facebook"`
	Youtube   string `json:"youtube"`
	Tiktok    string `json:"tiktok"`
	LogoURL   string `json:"logo_url"`
}
