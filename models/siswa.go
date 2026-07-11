package models

import "time"

// Siswa merepresentasikan peserta didik.
// KelasID bersifat opsional agar siswa bisa belum ditempatkan di kelas.
type Siswa struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	NIS       string    `gorm:"size:20;not null;uniqueIndex" json:"nis"`
	Nama      string    `gorm:"size:100;not null" json:"nama"`
	KelasID   *uint64   `gorm:"index" json:"kelas_id"`
	Kelas     *Kelas    `gorm:"foreignKey:KelasID" json:"kelas"`
	Aktif     bool      `gorm:"default:true" json:"aktif"`
	CreatedAt time.Time `json:"created_at"`
}

func (Siswa) TableName() string { return "siswa" }
