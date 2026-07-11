package models

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Kunci setting bawaan.
const KeyPersenPotonganTabungan = "persen_potongan_tabungan"

// Setting menyimpan konfigurasi aplikasi berbasis key-value.
type Setting struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Kunci      string    `gorm:"size:50;uniqueIndex;not null" json:"kunci"`
	Nilai      string    `gorm:"size:255;not null" json:"nilai"`
	Keterangan string    `gorm:"type:text" json:"keterangan"`
	UpdatedAt  time.Time `json:"updated_at"`
	UpdatedBy  *uint64   `json:"updated_by"`
}

func (Setting) TableName() string { return "setting" }

// GetSetting mengambil nilai setting; mengembalikan def bila belum ada.
func GetSetting(db *gorm.DB, kunci, def string) string {
	var s Setting
	if err := db.Where("kunci = ?", kunci).First(&s).Error; err != nil {
		return def
	}
	return s.Nilai
}

// SetSetting menyimpan/memperbarui nilai setting.
func SetSetting(db *gorm.DB, kunci, nilai string, by uint64) error {
	var s Setting
	err := db.Where("kunci = ?", kunci).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		s = Setting{Kunci: kunci, Nilai: nilai, UpdatedBy: &by}
		return db.Create(&s).Error
	} else if err != nil {
		return err
	}
	s.Nilai = nilai
	s.UpdatedBy = &by
	return db.Save(&s).Error
}

// PersenPotonganTabungan membaca persen potongan tabungan (default 0).
func PersenPotonganTabungan(db *gorm.DB) float64 {
	v := GetSetting(db, KeyPersenPotonganTabungan, "0")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}
