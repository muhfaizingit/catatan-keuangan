package models

import "time"

// Sumber piutang: dari sisa SPP (per bulan) atau sisa Tagihan (non-SPP).
const (
	SumberPiutangSPP     = "spp"
	SumberPiutangTagihan = "tagihan"
)

// Piutang adalah sisa tunggakan siswa dari tahun ajaran yang ditutup, dibawa
// agar tetap bisa ditagih. Bila dibayar, pembayaran dicatat sebagai pemasukan
// kas pada tahun ajaran yang AKTIF saat pembayaran (bukan TA asal).
//
// SumberTipe/SumberID mencatat asal tunggakan secara presisi (bukan digabung
// per siswa) agar bisa ditampilkan jejaknya di view piutang. TA asal tetap
// immutable — field ini hanya untuk rekonsiliasi tampilan, bukan mutasi.
type Piutang struct {
	ID                uint64        `gorm:"primaryKey" json:"id"`
	SiswaID           uint64        `gorm:"not null;index" json:"siswa_id"`
	TahunAjaranAsalID uint64        `gorm:"not null;index" json:"tahun_ajaran_asal_id"`
	TutupTahunID      *uint64       `json:"tutup_tahun_id"`
	Nominal           int64         `gorm:"not null" json:"nominal"`
	Keterangan        string        `gorm:"type:text" json:"keterangan"`
	Status            StatusTagihan `gorm:"type:varchar(10);default:'belum'" json:"status"`
	// SumberTipe: "spp" / "tagihan" / kosong untuk piutang legacy (carry-over).
	SumberTipe string  `gorm:"size:10" json:"sumber_tipe"`
	SumberID   *uint64 `json:"sumber_id"`
	CreatedAt  time.Time `json:"created_at"`

	Siswa           Siswa               `gorm:"foreignKey:SiswaID" json:"siswa"`
	TahunAjaranAsal TahunAjaran         `gorm:"foreignKey:TahunAjaranAsalID" json:"tahun_ajaran_asal"`
	Pembayaran      []PiutangPembayaran `gorm:"foreignKey:PiutangID" json:"pembayaran"`
}

func (Piutang) TableName() string { return "piutang" }

// PiutangPembayaran mencatat pembayaran (bisa cicilan) atas sebuah piutang.
// TahunAjaranBayarID = TA aktif saat pembayaran (untuk pencatatan kas).
type PiutangPembayaran struct {
	ID                uint64    `gorm:"primaryKey" json:"id"`
	PiutangID         uint64    `gorm:"not null;index" json:"piutang_id"`
	Tanggal           time.Time `gorm:"type:date;not null" json:"tanggal"`
	JumlahBayar       int64     `gorm:"not null" json:"jumlah_bayar"`
	TahunAjaranBayarID uint64   `gorm:"not null" json:"tahun_ajaran_bayar_id"`
	Keterangan        string    `gorm:"type:text" json:"keterangan"`
	UserID            uint64    `gorm:"not null" json:"user_id"`
	CreatedAt         time.Time `json:"created_at"`

	TahunAjaranBayar TahunAjaran `gorm:"foreignKey:TahunAjaranBayarID" json:"tahun_ajaran_bayar"`
}

func (PiutangPembayaran) TableName() string { return "piutang_pembayaran" }
