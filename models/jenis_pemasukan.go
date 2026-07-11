package models

import "time"

// JenisPemasukan adalah master sumber pemasukan kas (bebas ditambah).
// Contoh: Dana Dinas, Dana Yayasan, Pembayaran Orang Tua, Sumbangan.
type JenisPemasukan struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Nama       string    `gorm:"size:100;not null" json:"nama"`
	Keterangan string    `gorm:"type:text" json:"keterangan"`
	Aktif      bool      `gorm:"default:true" json:"aktif"`
	// Kode menandai jenis bawaan sistem (mis. "SPP"). Nil untuk jenis buatan user.
	// Jenis bersistem tidak boleh diedit/dihapus dan tidak dipilih manual di kas.
	// Keunikan dijaga di level aplikasi (seed memeriksa keberadaannya).
	Kode      *string   `gorm:"size:30" json:"kode"`
	CreatedAt time.Time `json:"created_at"`
}

func (JenisPemasukan) TableName() string { return "jenis_pemasukan" }

// Kode jenis pemasukan bawaan sistem.
const (
	KodeJenisSPP              = "SPP"
	KodeJenisDonasi           = "DONASI"
	KodeJenisPotonganTabungan = "POTONGAN_TABUNGAN"
	KodeJenisTagihan          = "TAGIHAN"
	KodeJenisSaldoAwal        = "SALDO_AWAL"
	KodeJenisPiutang          = "PIUTANG"
)

// IsSistem menandakan jenis ini dikelola sistem (tidak dapat diubah user).
func (j JenisPemasukan) IsSistem() bool { return j.Kode != nil && *j.Kode != "" }
