package models

import "time"

// KodeKategoriTabungan adalah kode kategori pengeluaran "Tabungan" (di-seed di
// main), dipakai untuk mencatat pencairan saldo tabungan saat tutup.
const KodeKategoriTabungan = "TABUNGAN"

// TabunganSetoran mencatat satu setoran tabungan siswa.
// Uang setoran (JumlahSetor, penuh) langsung masuk kas sebagai pemasukan saat
// disetor. Potongan sekolah di-snapshot (PersenPotong/JumlahPotong) dan baru
// direalisasikan saat tutup: saldo bersih (JumlahBersih) dikeluarkan lagi dari
// kas, sehingga potongan efektif tertahan di kas sebagai pendapatan sekolah.
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

// TabunganPenarikan mencatat pencairan tabungan per siswa sewaktu-waktu di tengah
// tahun ajaran (kasus pengecualian). Uang keluar dari kas sebesar Jumlah dan
// saldo siswa berkurang. TIDAK dikenai potongan — potongan sekolah hanya berlaku
// atas saldo yang mengendap sampai tutup tabungan.
type TabunganPenarikan struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	SiswaID       uint64    `gorm:"not null;index" json:"siswa_id"`
	TahunAjaranID uint64    `gorm:"not null;index" json:"tahun_ajaran_id"`
	Tanggal       time.Time `gorm:"type:date;not null" json:"tanggal"`
	Jumlah        int64     `gorm:"not null" json:"jumlah"`
	Keterangan    string    `gorm:"type:text" json:"keterangan"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`

	Siswa Siswa `gorm:"foreignKey:SiswaID" json:"siswa"`
}

func (TabunganPenarikan) TableName() string { return "tabungan_penarikan" }

// TutupTabungan mencatat event "tutup tabungan" seluruh sekolah pada satu tahun
// ajaran. Saat ditutup: saldo bersih (setoran − potongan) dikeluarkan lagi dari
// kas sebagai "Pencairan Tabungan"; potongan tertahan di kas sebagai pendapatan
// sekolah. Bagian saldo bersih yang melunasi tunggakan SPP/Tagihan tetap dicatat
// sebagai pemasukan (tetap di kas). Unik per tahun ajaran (sekali tutup; bisa dibatalkan).
type TutupTabungan struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	TahunAjaranID uint64    `gorm:"not null;uniqueIndex" json:"tahun_ajaran_id"`
	Tanggal       time.Time `gorm:"type:date;not null" json:"tanggal"`
	TotalSetor    int64     `json:"total_setor"`
	TotalPotong   int64     `json:"total_potong"`
	TotalBersih   int64     `json:"total_bersih"`
	// TotalBayarTunggakan: bagian saldo bersih yang dipakai melunasi tunggakan SPP/Tagihan.
	// TotalDiserahkan: sisa saldo bersih yang dikembalikan ke siswa (tidak masuk kas).
	TotalBayarTunggakan int64     `json:"total_bayar_tunggakan"`
	TotalDiserahkan     int64     `json:"total_diserahkan"`
	JumlahSiswa         int       `json:"jumlah_siswa"`
	Keterangan          string    `gorm:"type:text" json:"keterangan"`
	UserID              uint64    `gorm:"not null" json:"user_id"`
	CreatedAt           time.Time `json:"created_at"`

	TahunAjaran TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
}

func (TutupTabungan) TableName() string { return "tutup_tabungan" }
