package models

import "time"

// TabunganSetoran mencatat satu setoran tabungan siswa.
// Potongan sekolah di-snapshot (PersenPotong) agar perubahan setting tidak
// mengubah transaksi lama. JumlahBersih menambah saldo tabungan siswa,
// JumlahPotong menjadi pemasukan kas (jenis "Potongan Tabungan").
type TabunganSetoran struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	SiswaID       uint64    `gorm:"not null;index" json:"siswa_id"`
	TahunAjaranID uint64    `gorm:"not null;index" json:"tahun_ajaran_id"`
	Tanggal       time.Time `gorm:"type:date;not null" json:"tanggal"`
	JumlahSetor   int64     `gorm:"not null" json:"jumlah_setor"`
	PersenPotong  float64   `json:"persen_potong"`
	JumlahPotong  int64     `json:"jumlah_potong"`
	JumlahBersih  int64     `json:"jumlah_bersih"`
	Keterangan    string    `gorm:"type:text" json:"keterangan"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`

	Siswa Siswa `gorm:"foreignKey:SiswaID" json:"siswa"`
}

func (TabunganSetoran) TableName() string { return "tabungan_setoran" }

// TutupTabungan mencatat event "tutup tabungan" seluruh sekolah pada satu tahun
// ajaran. Saat ditutup: total potongan (dari snapshot tiap setoran) diakui ke
// kas sebagai "Potongan Tabungan"; saldo bersih diserahkan ke siswa (tidak
// memengaruhi kas). Unik per tahun ajaran (sekali tutup; bisa dibatalkan).
type TutupTabungan struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	TahunAjaranID uint64    `gorm:"not null;uniqueIndex" json:"tahun_ajaran_id"`
	Tanggal       time.Time `gorm:"type:date;not null" json:"tanggal"`
	TotalSetor    int64     `json:"total_setor"`
	TotalPotong   int64     `json:"total_potong"`
	TotalBersih   int64     `json:"total_bersih"`
	// TotalBayarTunggakan: bagian saldo bersih yang dipakai melunasi tunggakan SPP/Tagihan.
	// TotalDiserahkan: sisa saldo bersih yang dikembalikan ke siswa (tidak masuk kas).
	TotalBayarTunggakan int64 `json:"total_bayar_tunggakan"`
	TotalDiserahkan     int64 `json:"total_diserahkan"`
	JumlahSiswa         int   `json:"jumlah_siswa"`
	Keterangan    string    `gorm:"type:text" json:"keterangan"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`

	TahunAjaran TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
}

func (TutupTabungan) TableName() string { return "tutup_tabungan" }
