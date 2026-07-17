package models

import "gorm.io/gorm"

// AutoMigrate menjalankan migrasi skema untuk seluruh model.
// Model baru ditambahkan ke daftar ini seiring berkembangnya fitur.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&TahunAjaran{},
		&Kelas{},
		&Siswa{},
		&JenisPemasukan{},
		&KategoriPengeluaran{},
		&SubKategoriPengeluaran{},
		&KasPemasukan{},
		&KasPengeluaran{},
		&SppTagihan{},
		&SppPembayaran{},
		&Donatur{},
		&DanaBantuan{},
		&Setting{},
		&TabunganSetoran{},
		&TabunganPenarikan{},
		&TutupTabungan{},
		&JenisTagihan{},
		&Tagihan{},
		&TagihanPembayaran{},
		&Piutang{},
		&PiutangPembayaran{},
		&TutupTahun{},
		&Guru{},
		&GajiGuru{},
		&GajiPembayaran{},
	)
}
