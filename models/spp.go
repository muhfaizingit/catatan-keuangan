package models

import "time"

// StatusSPP menandakan kondisi pelunasan satu tagihan SPP.
type StatusSPP string

const (
	SPPBelum StatusSPP = "belum"
	SPPCicil StatusSPP = "cicil"
	SPPLunas StatusSPP = "lunas"
)

// SppTagihan adalah tagihan SPP satu siswa untuk satu bulan pada satu tahun ajaran.
// Unik per (siswa, tahun ajaran, bulan) agar tidak dobel saat generate.
type SppTagihan struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	SiswaID       uint64    `gorm:"not null;uniqueIndex:uq_siswa_ta_bulan" json:"siswa_id"`
	TahunAjaranID uint64    `gorm:"not null;uniqueIndex:uq_siswa_ta_bulan;index" json:"tahun_ajaran_id"`
	Bulan         int       `gorm:"not null;uniqueIndex:uq_siswa_ta_bulan" json:"bulan"` // 1-12
	Jumlah        int64     `gorm:"not null" json:"jumlah"`
	Status        StatusSPP `gorm:"type:varchar(10);default:'belum'" json:"status"`
	CreatedAt     time.Time `json:"created_at"`

	Siswa      Siswa          `gorm:"foreignKey:SiswaID" json:"siswa"`
	Pembayaran []SppPembayaran `gorm:"foreignKey:TagihanID" json:"pembayaran"`
}

func (SppTagihan) TableName() string { return "spp_tagihan" }

// Sumber pembayaran SPP.
const (
	SumberTunai    = "tunai"
	SumberBantuan  = "bantuan"
	SumberTabungan = "tabungan"
)

// SppPembayaran mencatat satu pembayaran (bisa cicilan) atas sebuah tagihan.
type SppPembayaran struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	TagihanID   uint64    `gorm:"not null;index" json:"tagihan_id"`
	Tanggal     time.Time `gorm:"type:date;not null" json:"tanggal"`
	JumlahBayar int64     `gorm:"not null" json:"jumlah_bayar"`
	Keterangan  string    `gorm:"type:text" json:"keterangan"`
	// Sumber: "tunai" (orang tua) atau "bantuan" (donatur).
	Sumber string `gorm:"size:10;default:'tunai'" json:"sumber"`
	// DanaBantuanID terisi bila pembayaran ini berasal dari dana bantuan.
	DanaBantuanID *uint64 `json:"dana_bantuan_id"`
	// TutupTabunganID terisi bila pembayaran ini hasil pelunasan otomatis saat tutup tabungan.
	TutupTabunganID *uint64   `json:"tutup_tabungan_id"`
	UserID          uint64    `gorm:"not null" json:"user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

func (SppPembayaran) TableName() string { return "spp_pembayaran" }

// DariBantuan menandakan pembayaran berasal dari dana bantuan donatur.
func (p SppPembayaran) DariBantuan() bool { return p.Sumber == SumberBantuan }
