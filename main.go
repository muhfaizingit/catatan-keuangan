package main

import (
	"errors"
	"html/template"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/auth"
	"teras-keuangan/config"
	"teras-keuangan/models"
	"teras-keuangan/routes"
	"teras-keuangan/util"
)

func main() {
	cfg := config.Load()

	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("gagal koneksi database: %v", err)
	}

	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("gagal migrasi: %v", err)
	}

	if err := seedAdmin(db, cfg); err != nil {
		log.Fatalf("gagal seed admin: %v", err)
	}

	if err := seedSystemData(db); err != nil {
		log.Fatalf("gagal seed data sistem: %v", err)
	}

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.SetFuncMap(template.FuncMap{
		"dict":      dict,
		"derefID":   derefID,
		"rupiah":    util.Rupiah,
		"ribuan":    util.FormatThousands,
		"tanggal":   func(t time.Time) string { return t.Format("02 Jan 2006") },
		"namaBulan": util.NamaBulan,
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
	})
	r.LoadHTMLGlob("templates/**/*.html")
	r.Static("/static", "./static")

	routes.Register(r, db, cfg)

	addr := ":" + cfg.AppPort
	log.Printf("server berjalan di http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server berhenti: %v", err)
	}
}

// seedAdmin membuat akun admin awal bila belum ada user sama sekali.
func seedAdmin(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(cfg.SeedAdminPassword)
	if err != nil {
		return err
	}

	admin := models.User{
		Nama:     cfg.SeedAdminNama,
		Email:    cfg.SeedAdminEmail,
		Password: hash,
		Role:     models.RoleAdmin,
		Aktif:    true,
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Printf("akun admin awal dibuat: %s (password sesuai .env)", cfg.SeedAdminEmail)
	return nil
}

// seedSystemData memastikan jenis pemasukan bawaan sistem tersedia.
func seedSystemData(db *gorm.DB) error {
	defaults := []struct {
		Kode, Nama, Keterangan string
	}{
		{models.KodeJenisSPP, "SPP", "Pemasukan dari pembayaran SPP (otomatis)"},
		{models.KodeJenisDonasi, "Donasi/Bantuan", "Donasi & sisa dana bantuan (otomatis)"},
		{models.KodeJenisPotonganTabungan, "Potongan Tabungan", "Potongan setoran tabungan siswa (otomatis)"},
		{models.KodeJenisTabungan, "Tabungan Siswa", "Setoran tabungan siswa masuk kas (otomatis)"},
		{models.KodeJenisTagihan, "Tagihan", "Pembayaran tagihan non-SPP (otomatis)"},
		{models.KodeJenisSaldoAwal, "Saldo Awal", "Saldo kas awal dari tutup tahun (otomatis)"},
		{models.KodeJenisPiutang, "Piutang", "Pembayaran piutang tahun sebelumnya (otomatis)"},
	}
	for _, d := range defaults {
		kode := d.Kode
		var count int64
		if err := db.Model(&models.JenisPemasukan{}).Where("kode = ?", kode).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			j := models.JenisPemasukan{Nama: d.Nama, Keterangan: d.Keterangan, Aktif: true, Kode: &kode}
			if err := db.Create(&j).Error; err != nil {
				return err
			}
			log.Printf("jenis pemasukan sistem '%s' dibuat", kode)
		}
	}

	// Setting bawaan: persen potongan tabungan.
	var cnt int64
	if err := db.Model(&models.Setting{}).Where("kunci = ?", models.KeyPersenPotonganTabungan).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		s := models.Setting{
			Kunci:      models.KeyPersenPotonganTabungan,
			Nilai:      "5",
			Keterangan: "Persen potongan tabungan siswa untuk sekolah",
		}
		if err := db.Create(&s).Error; err != nil {
			return err
		}
		log.Printf("setting '%s' dibuat (default 5%%)", models.KeyPersenPotonganTabungan)
	}

	// Kategori pengeluaran bawaan sistem (auto-posting).
	kategoriSistem := []struct{ Kode, Nama string }{
		{models.KodeKategoriGaji, "Gaji"},         // pembayaran gaji guru
		{models.KodeKategoriTabungan, "Tabungan"}, // pencairan saldo tabungan saat tutup
	}
	for _, kd := range kategoriSistem {
		var kc int64
		if err := db.Model(&models.KategoriPengeluaran{}).Where("kode = ?", kd.Kode).Count(&kc).Error; err != nil {
			return err
		}
		if kc == 0 {
			k := models.KategoriPengeluaran{Nama: kd.Nama, Kode: kd.Kode, Aktif: true}
			if err := db.Create(&k).Error; err != nil {
				return err
			}
			log.Printf("kategori pengeluaran '%s' dibuat", kd.Kode)
		}
	}
	return nil
}

// dict membangun map dari pasangan key-value untuk dipakai di template
// (mis. memanggil sub-template dengan beberapa parameter).
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: jumlah argumen harus genap")
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict: key harus string")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// derefID mengembalikan nilai *uint64 (0 bila nil); dipakai untuk menandai
// opsi <select> yang terpilih di template.
func derefID(p *uint64) uint64 {
	if p == nil {
		return 0
	}
	return *p
}
