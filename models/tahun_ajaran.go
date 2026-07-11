package models

import "time"

// TahunAjaran merepresentasikan satu periode tahun ajaran sekolah.
// Hanya satu tahun ajaran yang boleh berstatus aktif pada satu waktu.
type TahunAjaran struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Nama      string    `gorm:"size:20;not null" json:"nama"` // contoh: "2024/2025"
	Aktif     bool      `gorm:"default:false" json:"aktif"`
	// Ditutup: tahun ajaran sudah di-"tutup tahun"; transaksi kas dibekukan.
	Ditutup   bool      `gorm:"default:false" json:"ditutup"`
	CreatedAt time.Time `json:"created_at"`
}

func (TahunAjaran) TableName() string { return "tahun_ajaran" }
