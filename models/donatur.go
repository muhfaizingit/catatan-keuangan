package models

import "time"

// Donatur adalah pemberi dana bantuan (mis. yayasan, perusahaan, perorangan).
type Donatur struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Nama       string    `gorm:"size:100;not null" json:"nama"`
	Keterangan string    `gorm:"type:text" json:"keterangan"`
	Aktif      bool      `gorm:"default:true" json:"aktif"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Donatur) TableName() string { return "donatur" }
