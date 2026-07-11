package models

import "time"

// KategoriPengeluaran mengelompokkan pengeluaran kas (mis. Operasional, Gaji, Sarana).
type KategoriPengeluaran struct {
	ID    uint64                   `gorm:"primaryKey" json:"id"`
	Kode  string                   `gorm:"size:20" json:"kode"` // mis. "GAJI" untuk sistem
	Nama  string                   `gorm:"size:100;not null" json:"nama"`
	Aktif bool                     `gorm:"default:true" json:"aktif"`
	CreatedAt time.Time            `json:"created_at"`
	SubList   []SubKategoriPengeluaran `gorm:"foreignKey:KategoriID" json:"sub_list"`
}

func (KategoriPengeluaran) TableName() string { return "kategori_pengeluaran" }

// SubKategoriPengeluaran adalah rincian di bawah satu kategori pengeluaran.
type SubKategoriPengeluaran struct {
	ID         uint64 `gorm:"primaryKey" json:"id"`
	KategoriID uint64 `gorm:"not null;index" json:"kategori_id"`
	Nama       string `gorm:"size:100;not null" json:"nama"`
	Aktif      bool   `gorm:"default:true" json:"aktif"`
}

func (SubKategoriPengeluaran) TableName() string { return "sub_kategori_pengeluaran" }
