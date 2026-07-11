package models

import "time"

// Role mendefinisikan peran pengguna dalam sistem.
type Role string

const (
	RoleAdmin          Role = "admin"
	RoleBendahara      Role = "bendahara"
	RoleKepalaSekolah  Role = "kepala_sekolah"
)

// User merepresentasikan pengguna aplikasi.
type User struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Nama      string    `gorm:"size:100;not null" json:"nama"`
	Email     string    `gorm:"size:100;not null;uniqueIndex" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Role      Role      `gorm:"type:varchar(20);not null" json:"role"`
	Aktif     bool      `gorm:"default:true" json:"aktif"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName memastikan nama tabel sesuai rencana skema.
func (User) TableName() string { return "users" }

// RoleLabel mengembalikan label role yang ramah dibaca untuk UI.
func (u User) RoleLabel() string {
	switch u.Role {
	case RoleAdmin:
		return "Administrator"
	case RoleBendahara:
		return "Bendahara"
	case RoleKepalaSekolah:
		return "Kepala Sekolah"
	default:
		return string(u.Role)
	}
}
