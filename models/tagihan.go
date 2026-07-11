package models

import "time"

// StatusTagihan sama semantiknya dengan status SPP (belum/cicil/lunas).
type StatusTagihan string

const (
	TagihanBelum StatusTagihan = "belum"
	TagihanCicil StatusTagihan = "cicil"
	TagihanLunas StatusTagihan = "lunas"
)

// Tagihan adalah satu tagihan non-SPP untuk satu siswa (nominal bisa beda per siswa).
type Tagihan struct {
	ID             uint64        `gorm:"primaryKey" json:"id"`
	JenisTagihanID uint64        `gorm:"not null;index" json:"jenis_tagihan_id"`
	SiswaID        uint64        `gorm:"not null;index" json:"siswa_id"`
	TahunAjaranID  uint64        `gorm:"not null;index" json:"tahun_ajaran_id"`
	Nominal        int64         `gorm:"not null" json:"nominal"`
	Tanggal        time.Time     `gorm:"type:date;not null" json:"tanggal"`
	Keterangan     string        `gorm:"type:text" json:"keterangan"`
	Status         StatusTagihan `gorm:"type:varchar(10);default:'belum'" json:"status"`
	UserID         uint64        `gorm:"not null" json:"user_id"`
	CreatedAt      time.Time     `json:"created_at"`

	JenisTagihan JenisTagihan        `gorm:"foreignKey:JenisTagihanID" json:"jenis_tagihan"`
	Siswa        Siswa               `gorm:"foreignKey:SiswaID" json:"siswa"`
	Pembayaran   []TagihanPembayaran `gorm:"foreignKey:TagihanID" json:"pembayaran"`
}

func (Tagihan) TableName() string { return "tagihan" }

// TagihanPembayaran mencatat satu pembayaran (bisa cicilan) atas sebuah tagihan.
type TagihanPembayaran struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	TagihanID   uint64    `gorm:"not null;index" json:"tagihan_id"`
	Tanggal     time.Time `gorm:"type:date;not null" json:"tanggal"`
	JumlahBayar int64     `gorm:"not null" json:"jumlah_bayar"`
	Keterangan  string    `gorm:"type:text" json:"keterangan"`
	// TutupTabunganID terisi bila pembayaran ini hasil pelunasan otomatis saat tutup tabungan.
	TutupTabunganID *uint64   `json:"tutup_tabungan_id"`
	UserID          uint64    `gorm:"not null" json:"user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

func (TagihanPembayaran) TableName() string { return "tagihan_pembayaran" }
