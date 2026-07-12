package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/config"
	"teras-keuangan/handlers"
	"teras-keuangan/middleware"
	"teras-keuangan/models"
)

// Register memasang seluruh rute aplikasi.
func Register(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	authH := handlers.NewAuthHandler(db, cfg)
	dashboardH := handlers.NewDashboardHandler(db)
	masterH := handlers.NewMasterHandler(db)
	kasH := handlers.NewKasHandler(db)
	sppH := handlers.NewSppHandler(db)
	tabunganH := handlers.NewTabunganHandler(db)
	tagihanH := handlers.NewTagihanHandler(db)
	tutupTahunH := handlers.NewTutupTahunHandler(db)
	piutangH := handlers.NewPiutangHandler(db)
	gajiH := handlers.NewGajiHandler(db)

	// Rute publik
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/dashboard") })
	r.GET("/login", authH.ShowLogin)
	r.POST("/login", authH.Login)
	r.GET("/logout", authH.Logout)
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Rute terproteksi (butuh login)
	app := r.Group("/")
	app.Use(middleware.Auth(cfg.JWTSecret))
	{
		app.GET("/dashboard", dashboardH.Index)

		// ---- Master Data (admin only) ----
		master := app.Group("/master")
		master.Use(middleware.RequireRole(string(models.RoleAdmin)))
		{
			// Tahun Ajaran
			master.GET("/tahun-ajaran", masterH.TahunAjaranIndex)
			master.GET("/tahun-ajaran/new", masterH.TahunAjaranForm)
			master.GET("/tahun-ajaran/:id/edit", masterH.TahunAjaranForm)
			master.POST("/tahun-ajaran", masterH.TahunAjaranCreate)
			master.PUT("/tahun-ajaran/:id", masterH.TahunAjaranUpdate)
			master.POST("/tahun-ajaran/:id/aktif", masterH.TahunAjaranSetAktif)
			master.DELETE("/tahun-ajaran/:id", masterH.TahunAjaranDelete)

			// Kelas
			master.GET("/kelas", masterH.KelasIndex)
			master.GET("/kelas/new", masterH.KelasForm)
			master.GET("/kelas/:id/edit", masterH.KelasForm)
			master.POST("/kelas", masterH.KelasCreate)
			master.PUT("/kelas/:id", masterH.KelasUpdate)
			master.DELETE("/kelas/:id", masterH.KelasDelete)

			// Siswa
			master.GET("/siswa", masterH.SiswaIndex)
			master.GET("/siswa/new", masterH.SiswaForm)
			master.GET("/siswa/:id/edit", masterH.SiswaForm)
			master.POST("/siswa", masterH.SiswaCreate)
			master.PUT("/siswa/:id", masterH.SiswaUpdate)
			master.DELETE("/siswa/:id", masterH.SiswaDelete)

			// Jenis Pemasukan
			master.GET("/jenis-pemasukan", masterH.JenisIndex)
			master.GET("/jenis-pemasukan/new", masterH.JenisForm)
			master.GET("/jenis-pemasukan/:id/edit", masterH.JenisForm)
			master.POST("/jenis-pemasukan", masterH.JenisCreate)
			master.PUT("/jenis-pemasukan/:id", masterH.JenisUpdate)
			master.DELETE("/jenis-pemasukan/:id", masterH.JenisDelete)

			// Kategori + Sub Kategori Pengeluaran
			master.GET("/kategori", masterH.KategoriIndex)
			master.GET("/kategori/new", masterH.KategoriForm)
			master.GET("/kategori/:id/edit", masterH.KategoriForm)
			master.POST("/kategori", masterH.KategoriCreate)
			master.PUT("/kategori/:id", masterH.KategoriUpdate)
			master.DELETE("/kategori/:id", masterH.KategoriDelete)
			master.GET("/kategori/:id/sub/new", masterH.SubForm)
			master.GET("/kategori/:id/sub/:subID/edit", masterH.SubForm)
			master.POST("/kategori/:id/sub", masterH.SubCreate)
			master.PUT("/kategori/:id/sub/:subID", masterH.SubUpdate)
			master.DELETE("/kategori/:id/sub/:subID", masterH.SubDelete)

			// Donatur
			master.GET("/donatur", masterH.DonaturIndex)
			master.GET("/donatur/new", masterH.DonaturForm)
			master.GET("/donatur/:id/edit", masterH.DonaturForm)
			master.POST("/donatur", masterH.DonaturCreate)
			master.PUT("/donatur/:id", masterH.DonaturUpdate)
			master.DELETE("/donatur/:id", masterH.DonaturDelete)

			// Guru
			master.GET("/guru", masterH.GuruIndex)
			master.GET("/guru/new", masterH.GuruForm)
			master.GET("/guru/:id/edit", masterH.GuruForm)
			master.POST("/guru", masterH.GuruCreate)
			master.PUT("/guru/:id", masterH.GuruUpdate)
			master.DELETE("/guru/:id", masterH.GuruDelete)

			// Jenis Tagihan
			master.GET("/jenis-tagihan", masterH.JenisTagihanIndex)
			master.GET("/jenis-tagihan/new", masterH.JenisTagihanForm)
			master.GET("/jenis-tagihan/:id/edit", masterH.JenisTagihanForm)
			master.POST("/jenis-tagihan", masterH.JenisTagihanCreate)
			master.PUT("/jenis-tagihan/:id", masterH.JenisTagihanUpdate)
			master.DELETE("/jenis-tagihan/:id", masterH.JenisTagihanDelete)
		}

		// ---- Kas Sekolah ----
		// Lihat: admin, bendahara, kepala sekolah. Tulis: admin, bendahara.
		view := middleware.RequireRole(string(models.RoleAdmin), string(models.RoleBendahara), string(models.RoleKepalaSekolah))
		write := middleware.RequireRole(string(models.RoleAdmin), string(models.RoleBendahara))

		app.GET("/kas/pemasukan", view, kasH.PemasukanIndex)
		app.GET("/kas/pemasukan/new", write, kasH.PemasukanForm)
		app.POST("/kas/pemasukan", write, kasH.PemasukanCreate)
		app.DELETE("/kas/pemasukan/:id", write, kasH.PemasukanDelete)

		app.GET("/kas/pengeluaran", view, kasH.PengeluaranIndex)
		app.GET("/kas/pengeluaran/new", write, kasH.PengeluaranForm)
		app.POST("/kas/pengeluaran", write, kasH.PengeluaranCreate)
		app.DELETE("/kas/pengeluaran/:id", write, kasH.PengeluaranDelete)

		// ---- SPP ----
		app.GET("/spp/pembayaran", view, sppH.PembayaranIndex)
		app.GET("/spp/tunggakan", view, sppH.TunggakanIndex)
		app.GET("/spp/generate", write, sppH.GenerateForm)
		app.POST("/spp/generate", write, sppH.Generate)
		app.GET("/spp/bayar/:id", write, sppH.BayarForm)
		app.POST("/spp/bayar/:id", write, sppH.Bayar)
		app.GET("/spp/tagihan/:id/riwayat", view, sppH.RiwayatForm)
		app.GET("/spp/tagihan/:id/edit", write, sppH.EditForm)
		app.PUT("/spp/tagihan/:id", write, sppH.Edit)
		app.DELETE("/spp/pembayaran/:id", write, sppH.HapusPembayaran)

		// SPP · Dana Bantuan
		app.GET("/spp/bantuan", view, sppH.BantuanIndex)
		app.GET("/spp/bantuan/new", write, sppH.BantuanForm)
		app.GET("/spp/bantuan/siswa", write, sppH.BantuanSiswaOptions)
		app.POST("/spp/bantuan", write, sppH.BantuanCreate)
		app.GET("/spp/bantuan/:id/detail", view, sppH.BantuanDetail)
		app.DELETE("/spp/bantuan/:id", write, sppH.BantuanDelete)

		// ---- Tabungan ----
		app.GET("/tabungan", view, tabunganH.Index)
		app.GET("/tabungan/setor-form/:id", write, tabunganH.SetorForm)
		app.POST("/tabungan/setor", write, tabunganH.Setor)
		app.GET("/tabungan/bulk-form", write, tabunganH.BulkForm)
		app.GET("/tabungan/bulk-siswa", write, tabunganH.BulkSiswaOptions)
		app.POST("/tabungan/bulk", write, tabunganH.Bulk)
		app.GET("/tabungan/riwayat/:id", view, tabunganH.RiwayatForm)
		app.DELETE("/tabungan/setoran/:id", write, tabunganH.HapusSetoran)
		app.GET("/tabungan/persen-form", write, tabunganH.PersenForm)
		app.POST("/tabungan/persen", write, tabunganH.PersenSave)
		app.GET("/tabungan/tutup-form", write, tabunganH.TutupForm)
		app.POST("/tabungan/tutup", write, tabunganH.Tutup)
		app.DELETE("/tabungan/tutup", write, tabunganH.BatalTutup)

		// ---- Tagihan (non-SPP) ----
		app.GET("/tagihan", view, tagihanH.Index)
		app.GET("/tagihan/tunggakan", view, tagihanH.TunggakanIndex)
		app.GET("/tagihan/terbitkan", write, tagihanH.TerbitkanForm)
		app.GET("/tagihan/terbitkan-siswa", write, tagihanH.TerbitkanSiswaOptions)
		app.POST("/tagihan/terbitkan", write, tagihanH.Terbitkan)
		app.GET("/tagihan/:id/riwayat", view, tagihanH.RiwayatForm)
		app.GET("/tagihan/:id/bayar", write, tagihanH.BayarForm)
		app.POST("/tagihan/:id/bayar", write, tagihanH.Bayar)
		app.DELETE("/tagihan/:id", write, tagihanH.HapusTagihan)
		app.DELETE("/tagihan/pembayaran/:id", write, tagihanH.HapusPembayaran)

		// ---- Piutang ----
		app.GET("/piutang", view, piutangH.Index)
		app.GET("/piutang/:id/riwayat", view, piutangH.RiwayatForm)
		app.GET("/piutang/:id/bayar", write, piutangH.BayarForm)
		app.POST("/piutang/:id/bayar", write, piutangH.Bayar)
		app.DELETE("/piutang/pembayaran/:id", write, piutangH.HapusPembayaran)

		// ---- Gaji Guru (payable) ----
		app.GET("/gaji", view, gajiH.Index)
		app.GET("/gaji/generate", write, gajiH.GenerateForm)
		app.POST("/gaji/generate", write, gajiH.Generate)
		app.GET("/gaji/:id/bayar", write, gajiH.BayarForm)
		app.POST("/gaji/:id/bayar", write, gajiH.Bayar)
		app.GET("/gaji/:id/riwayat", view, gajiH.RiwayatForm)
		app.DELETE("/gaji/pembayaran/:id", write, gajiH.HapusPembayaran)

		// ---- Tutup Tahun (admin) ----
		adminOnly := middleware.RequireRole(string(models.RoleAdmin))
		app.GET("/tutup-tahun", adminOnly, tutupTahunH.Index)
		app.POST("/tutup-tahun", adminOnly, tutupTahunH.Proses)
		app.DELETE("/tutup-tahun", adminOnly, tutupTahunH.Batal)
	}
}
