package models

import "time"

// Catatan desain: nilai uang disimpan sebagai int64 dalam satuan rupiah penuh
// (bukan float/DECIMAL) untuk menghindari galat pembulatan pada perhitungan uang.
// Rupiah praktis tidak memakai sen, sehingga ini akurat dan mudah diformat.

// KasPemasukan mencatat satu transaksi pemasukan kas sekolah.
type KasPemasukan struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	TahunAjaranID    uint64    `gorm:"not null;index" json:"tahun_ajaran_id"`
	JenisPemasukanID uint64    `gorm:"not null;index" json:"jenis_pemasukan_id"`
	Tanggal          time.Time `gorm:"type:date;not null;index" json:"tanggal"`
	Jumlah           int64     `gorm:"not null" json:"jumlah"`
	Keterangan       string    `gorm:"type:text" json:"keterangan"`
	UserID           uint64    `gorm:"not null" json:"user_id"`
	// SppPembayaranID terisi bila baris ini dibuat otomatis dari pembayaran SPP.
	SppPembayaranID *uint64 `json:"spp_pembayaran_id"`
	// DanaBantuanID terisi bila baris ini berasal dari dana bantuan
	// (baris SPP via bantuan, atau baris donasi dari sisa dana).
	DanaBantuanID *uint64 `json:"dana_bantuan_id"`
	// TabunganSetoranID terisi bila baris ini potongan dari satu setoran (model lama).
	TabunganSetoranID *uint64 `json:"tabungan_setoran_id"`
	// TutupTabunganID terisi bila baris ini potongan agregat hasil tutup tabungan.
	TutupTabunganID *uint64 `json:"tutup_tabungan_id"`
	// TagihanPembayaranID terisi bila baris ini dari pembayaran tagihan non-SPP.
	TagihanPembayaranID *uint64 `json:"tagihan_pembayaran_id"`
	// PiutangPembayaranID terisi bila baris ini dari pembayaran piutang.
	PiutangPembayaranID *uint64 `json:"piutang_pembayaran_id"`
	// TutupTahunID terisi bila baris ini "Saldo Awal" hasil tutup tahun.
	TutupTahunID *uint64   `json:"tutup_tahun_id"`
	CreatedAt    time.Time `json:"created_at"`

	TahunAjaran    TahunAjaran    `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
	JenisPemasukan JenisPemasukan `gorm:"foreignKey:JenisPemasukanID" json:"jenis_pemasukan"`
	User           User           `gorm:"foreignKey:UserID" json:"user"`
}

func (KasPemasukan) TableName() string { return "kas_pemasukan" }

// DariSPP menandakan baris ini hasil auto-posting pembayaran SPP.
func (k KasPemasukan) DariSPP() bool { return k.SppPembayaranID != nil }

// DariBantuan menandakan baris ini berasal dari dana bantuan (donasi sisa).
func (k KasPemasukan) DariBantuan() bool { return k.DanaBantuanID != nil && k.SppPembayaranID == nil }

// DariTabungan menandakan baris ini potongan tabungan (per setoran atau tutup).
func (k KasPemasukan) DariTabungan() bool {
	return k.TabunganSetoranID != nil || k.TutupTabunganID != nil
}

// DariTagihan menandakan baris ini dari pembayaran tagihan non-SPP.
func (k KasPemasukan) DariTagihan() bool { return k.TagihanPembayaranID != nil }

// DariPiutang menandakan baris ini dari pembayaran piutang.
func (k KasPemasukan) DariPiutang() bool { return k.PiutangPembayaranID != nil }

// DariSaldoAwal menandakan baris ini saldo awal hasil tutup tahun.
func (k KasPemasukan) DariSaldoAwal() bool { return k.TutupTahunID != nil }

// Terkunci: baris otomatis tidak boleh dihapus manual dari Kas.
func (k KasPemasukan) Terkunci() bool {
	return k.SppPembayaranID != nil || k.DanaBantuanID != nil ||
		k.TabunganSetoranID != nil || k.TutupTabunganID != nil ||
		k.TagihanPembayaranID != nil || k.PiutangPembayaranID != nil ||
		k.TutupTahunID != nil
}

// KasPengeluaran mencatat satu transaksi pengeluaran kas sekolah.
type KasPengeluaran struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	TahunAjaranID uint64    `gorm:"not null;index" json:"tahun_ajaran_id"`
	KategoriID    uint64    `gorm:"not null;index" json:"kategori_id"`
	SubKategoriID *uint64   `gorm:"index" json:"sub_kategori_id"`
	Tanggal       time.Time `gorm:"type:date;not null;index" json:"tanggal"`
	Jumlah        int64     `gorm:"not null" json:"jumlah"`
	Keterangan    string    `gorm:"type:text" json:"keterangan"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`

	TahunAjaran TahunAjaran             `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
	Kategori    KategoriPengeluaran    `gorm:"foreignKey:KategoriID" json:"kategori"`
	SubKategori *SubKategoriPengeluaran `gorm:"foreignKey:SubKategoriID" json:"sub_kategori"`
	User        User                   `gorm:"foreignKey:UserID" json:"user"`
}

func (KasPengeluaran) TableName() string { return "kas_pengeluaran" }
