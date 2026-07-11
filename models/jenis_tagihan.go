package models

import "time"

// JenisTagihan adalah master jenis pembayaran non-SPP (insidental), mis.
// Uang Seragam, Uang Buku, Daftar Ulang, Study Tour, Uang Ujian.
type JenisTagihan struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Nama       string    `gorm:"size:100;not null" json:"nama"`
	Keterangan string    `gorm:"type:text" json:"keterangan"`
	Aktif      bool      `gorm:"default:true" json:"aktif"`
	CreatedAt  time.Time `json:"created_at"`
}

func (JenisTagihan) TableName() string { return "jenis_tagihan" }
