package models

import "time"

// TutupTahun mencatat event tutup tahun ajaran: TA lama dikunci, TA baru
// diaktifkan, saldo kas dibawa, dan sisa tunggakan jadi piutang. Bisa dibatalkan.
type TutupTahun struct {
	ID                uint64    `gorm:"primaryKey" json:"id"`
	TahunAjaranLamaID uint64    `gorm:"not null;uniqueIndex" json:"tahun_ajaran_lama_id"`
	TahunAjaranBaruID uint64    `gorm:"not null" json:"tahun_ajaran_baru_id"`
	Tanggal           time.Time `gorm:"type:date;not null" json:"tanggal"`
	SaldoDibawa       int64     `json:"saldo_dibawa"`
	TotalPiutang      int64     `json:"total_piutang"`
	JumlahPiutang     int       `json:"jumlah_piutang"`
	Keterangan        string    `gorm:"type:text" json:"keterangan"`
	UserID            uint64    `gorm:"not null" json:"user_id"`
	CreatedAt         time.Time `json:"created_at"`

	TahunAjaranLama TahunAjaran `gorm:"foreignKey:TahunAjaranLamaID" json:"tahun_ajaran_lama"`
	TahunAjaranBaru TahunAjaran `gorm:"foreignKey:TahunAjaranBaruID" json:"tahun_ajaran_baru"`
}

func (TutupTahun) TableName() string { return "tutup_tahun" }
