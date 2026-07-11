package models

import "time"

// DanaBantuan mencatat satu pemberian dana dari satu donatur untuk membayar
// SPP sejumlah siswa pada beberapa bulan. Nilai uang dalam rupiah (int64).
//
// Aturan alokasi per slot (siswa×bulan):
//   bayar  = min(NominalDonatur, sisa tagihan)  -> masuk SPP (kas "SPP")
//   selisih lebih (NominalDonatur > sisa)        -> TotalDonasi (kas "Donasi")
//   selisih kurang (NominalDonatur < sisa)        -> tagihan tetap cicil
//
// JumlahDiterima = NominalDonatur × jumlah slot = TotalKeSPP + TotalDonasi.
type DanaBantuan struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	DonaturID      uint64    `gorm:"not null" json:"donatur_id"`
	TahunAjaranID  uint64    `gorm:"not null" json:"tahun_ajaran_id"`
	Tanggal        time.Time `gorm:"type:date;not null" json:"tanggal"`
	NominalSPP     int64     `json:"nominal_spp"`     // nominal tagihan SPP/bulan
	NominalDonatur int64     `json:"nominal_donatur"` // kontribusi donatur/slot
	JumlahDiterima int64     `json:"jumlah_diterima"`
	TotalKeSPP     int64     `json:"total_ke_spp"`
	TotalDonasi    int64     `json:"total_donasi"`
	JumlahSlot     int       `json:"jumlah_slot"`
	Keterangan     string    `gorm:"type:text" json:"keterangan"`
	UserID         uint64    `gorm:"not null" json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`

	Donatur     Donatur     `gorm:"foreignKey:DonaturID" json:"donatur"`
	TahunAjaran TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
}

func (DanaBantuan) TableName() string { return "dana_bantuan" }
