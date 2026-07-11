package models

import "time"

// Guru adalah tenaga pendidik/kependidikan sekolah penerima gaji.
type Guru struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Nama      string    `gorm:"size:100;not null" json:"nama"`
	Aktif     bool      `gorm:"default:true" json:"aktif"`
	CreatedAt time.Time `json:"created_at"`
}

func (Guru) TableName() string { return "guru" }

// StatusGaji sama semantiknya dengan status SPP/Tagihan (belum/cicil/lunas).
type StatusGaji string

const (
	GajiBelum StatusGaji = "belum"
	GajiCicil StatusGaji = "cicil"
	GajiLunas StatusGaji = "lunas"
)

// GajiGuru adalah satu kewajiban gaji sekolah kepada seorang guru untuk satu
// bulan pada satu tahun ajaran. Bila gaji telat dibayar, sisa menjadi "hutang
// sekolah" (payable) yang bisa dibayar cicil lewat GajiPembayaran.
//
// Total terbayar dihitung dari SUM(GajiPembayaran.jumlah_bayar); field Terbayar
// tidak disimpan (dihitung saat query) agar tidak ada data ganda.
type GajiGuru struct {
	ID            uint64     `gorm:"primaryKey" json:"id"`
	GuruID        uint64     `gorm:"not null;uniqueIndex:uq_guru_ta_bulan" json:"guru_id"`
	TahunAjaranID uint64     `gorm:"not null;uniqueIndex:uq_guru_ta_bulan;index" json:"tahun_ajaran_id"`
	Bulan         int        `gorm:"not null;uniqueIndex:uq_guru_ta_bulan" json:"bulan"` // 1-12
	Jumlah        int64      `gorm:"not null" json:"jumlah"`
	Status        StatusGaji `gorm:"type:varchar(10);default:'belum'" json:"status"`
	Keterangan    string     `gorm:"type:text" json:"keterangan"`
	UserID        uint64     `gorm:"not null" json:"user_id"`
	CreatedAt     time.Time  `json:"created_at"`

	Guru        Guru        `gorm:"foreignKey:GuruID" json:"guru"`
	TahunAjaran TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
	Pembayaran  []GajiPembayaran `gorm:"foreignKey:GajiID" json:"pembayaran"`
}

func (GajiGuru) TableName() string { return "gaji_guru" }

// GajiPembayaran mencatat satu pembayaran (bisa cicilan) atas kewajiban gaji.
// Saat dibayar, otomatis tercatat sebagai KasPengeluaran (kategori Gaji).
type GajiPembayaran struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	GajiID           uint64    `gorm:"not null;index" json:"gaji_id"`
	Tanggal          time.Time `gorm:"type:date;not null" json:"tanggal"`
	JumlahBayar      int64     `gorm:"not null" json:"jumlah_bayar"`
	Keterangan       string    `gorm:"type:text" json:"keterangan"`
	KasPengeluaranID *uint64   `json:"kas_pengeluaran_id"` // baris kas terkait (untuk reverse saat hapus)
	UserID           uint64    `gorm:"not null" json:"user_id"`
	CreatedAt        time.Time `json:"created_at"`
}

func (GajiPembayaran) TableName() string { return "gaji_pembayaran" }

// KodeKategoriGaji adalah kode kategori pengeluaran "Gaji" (di-seed di main).
const KodeKategoriGaji = "GAJI"
