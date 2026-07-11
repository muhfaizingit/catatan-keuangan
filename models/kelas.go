package models

// Kelas merepresentasikan rombongan belajar pada satu tahun ajaran.
type Kelas struct {
	ID            uint64      `gorm:"primaryKey" json:"id"`
	Nama          string      `gorm:"size:50;not null" json:"nama"`
	TahunAjaranID uint64      `gorm:"not null;index" json:"tahun_ajaran_id"`
	TahunAjaran   TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahun_ajaran"`
}

func (Kelas) TableName() string { return "kelas" }
