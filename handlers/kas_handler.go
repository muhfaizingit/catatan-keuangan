package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/middleware"
	"teras-keuangan/models"
	"teras-keuangan/util"
)

// KasHandler menangani kas pemasukan & pengeluaran.
type KasHandler struct {
	DB *gorm.DB
}

func NewKasHandler(db *gorm.DB) *KasHandler {
	return &KasHandler{DB: db}
}

// KasSummary merangkum posisi kas pada satu tahun ajaran.
type KasSummary struct {
	TotalPemasukan   int64
	TotalPengeluaran int64
	Saldo            int64
}

// ---------- helper bersama ----------

func ctxUserID(c *gin.Context) uint64 {
	v, _ := c.Get(middleware.CtxUserID)
	id, _ := v.(uint64)
	return id
}

// canEditKas: admin & bendahara boleh menulis; kepala sekolah hanya melihat.
func canEditKas(c *gin.Context) bool {
	role, _ := c.Get(middleware.CtxRole)
	r, _ := role.(string)
	return r == string(models.RoleAdmin) || r == string(models.RoleBendahara)
}

// tahunAjaranAll mengembalikan seluruh tahun ajaran (terbaru dulu).
func tahunAjaranAll(db *gorm.DB) []models.TahunAjaran {
	var list []models.TahunAjaran
	db.Order("nama desc").Find(&list)
	return list
}

// activeTahunAjaranID mengembalikan ID tahun ajaran yang sedang aktif (0 bila none).
func activeTahunAjaranID(db *gorm.DB) uint64 {
	var ta models.TahunAjaran
	if err := db.Where("aktif = ?", true).First(&ta).Error; err == nil {
		return ta.ID
	}
	return 0
}

// taClosed menandakan tahun ajaran sudah di-tutup-tahun (transaksi dibekukan).
func (h *KasHandler) taClosed(taID uint64) bool {
	var ta models.TahunAjaran
	if h.DB.First(&ta, taID).Error == nil {
		return ta.Ditutup
	}
	return false
}

// SummaryForTA menghitung total pemasukan, pengeluaran, dan saldo satu TA.
func SummaryForTA(db *gorm.DB, taID uint64) KasSummary {
	var s KasSummary
	db.Model(&models.KasPemasukan{}).Where("tahun_ajaran_id = ?", taID).
		Select("COALESCE(SUM(jumlah),0)").Scan(&s.TotalPemasukan)
	db.Model(&models.KasPengeluaran{}).Where("tahun_ajaran_id = ?", taID).
		Select("COALESCE(SUM(jumlah),0)").Scan(&s.TotalPengeluaran)
	s.Saldo = s.TotalPemasukan - s.TotalPengeluaran
	return s
}

// resolveFilter membaca query ta & bulan; ta default = TA aktif.
func (h *KasHandler) resolveFilter(c *gin.Context) (taID uint64, bulan int) {
	if v := c.Query("ta"); v != "" {
		taID, _ = strconv.ParseUint(v, 10, 64)
	}
	if taID == 0 {
		taID = activeTahunAjaranID(h.DB)
	}
	if v := c.Query("bulan"); v != "" {
		bulan, _ = strconv.Atoi(v)
	}
	return
}

// =====================================================================
// PEMASUKAN
// =====================================================================

func (h *KasHandler) pemasukanContext(c *gin.Context) gin.H {
	taID, bulan := h.resolveFilter(c)

	var list []models.KasPemasukan
	q := h.DB.Preload("JenisPemasukan").Where("tahun_ajaran_id = ?", taID)
	if bulan >= 1 && bulan <= 12 {
		q = q.Where("MONTH(tanggal) = ?", bulan)
	}
	q.Order("tanggal desc, id desc").Find(&list)

	return gin.H{
		"SelectedTA": taID,
		"Bulan":      bulan,
		"TahunList":  tahunAjaranAll(h.DB),
		"Summary":    SummaryForTA(h.DB, taID),
		"List":       list,
		"HasTA":      taID != 0,
		"CanEdit":    canEditKas(c),
	}
}

func (h *KasHandler) PemasukanIndex(c *gin.Context) {
	data := h.pemasukanContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "kas/pemasukan_content", data)
		return
	}
	data["Title"] = "Kas · Pemasukan"
	data["ActiveMenu"] = "kas"
	data["ActiveTab"] = "pemasukan"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "kas/pemasukan", data)
}

func (h *KasHandler) PemasukanForm(c *gin.Context) {
	taID, bulan := h.resolveFilter(c)
	var jenis []models.JenisPemasukan
	h.DB.Where("aktif = ? AND kode IS NULL", true).Order("nama asc").Find(&jenis)
	c.HTML(http.StatusOK, "kas/pemasukan_form", gin.H{
		"Title":     "Tambah Pemasukan",
		"SelectedTA": taID,
		"Bulan":     bulan,
		"JenisList": jenis,
		"Today":     time.Now().Format("2006-01-02"),
	})
}

func (h *KasHandler) pemasukanFormErr(c *gin.Context, msg string, taID uint64, bulan int) {
	var jenis []models.JenisPemasukan
	h.DB.Where("aktif = ? AND kode IS NULL", true).Order("nama asc").Find(&jenis)
	c.HTML(http.StatusOK, "kas/pemasukan_form", gin.H{
		"Title": "Tambah Pemasukan", "Error": msg,
		"SelectedTA": taID, "Bulan": bulan, "JenisList": jenis,
		"Today": time.Now().Format("2006-01-02"),
		"Jenis": c.PostForm("jenis_pemasukan_id"), "Jumlah": c.PostForm("jumlah"),
		"Tanggal": c.PostForm("tanggal"), "Keterangan": c.PostForm("keterangan"),
	})
}

func (h *KasHandler) refreshPemasukan(c *gin.Context) {
	c.HTML(http.StatusOK, "kas/pemasukan_content_oob", h.pemasukanContext(c))
}

func (h *KasHandler) PemasukanCreate(c *gin.Context) {
	taID, _ := strconv.ParseUint(c.PostForm("ta"), 10, 64)
	bulan, _ := strconv.Atoi(c.PostForm("bulan"))
	if taID == 0 {
		taID = activeTahunAjaranID(h.DB)
	}
	jenisID, _ := strconv.ParseUint(c.PostForm("jenis_pemasukan_id"), 10, 64)
	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())

	if taID == 0 {
		h.pemasukanFormErr(c, "Belum ada tahun ajaran aktif. Aktifkan dulu di Master Data.", taID, bulan)
		return
	}
	if h.taClosed(taID) {
		h.pemasukanFormErr(c, "Tahun ajaran ini sudah ditutup. Transaksi dibekukan.", taID, bulan)
		return
	}
	if jenisID == 0 {
		h.pemasukanFormErr(c, "Jenis pemasukan wajib dipilih.", taID, bulan)
		return
	}
	if jumlah <= 0 {
		h.pemasukanFormErr(c, "Jumlah harus lebih dari 0.", taID, bulan)
		return
	}

	row := models.KasPemasukan{
		TahunAjaranID:    taID,
		JenisPemasukanID: jenisID,
		Tanggal:          tanggal,
		Jumlah:           jumlah,
		Keterangan:       c.PostForm("keterangan"),
		UserID:           ctxUserID(c),
	}
	if err := h.DB.Create(&row).Error; err != nil {
		h.pemasukanFormErr(c, "Gagal menyimpan transaksi.", taID, bulan)
		return
	}
	// Refresh konten sesuai filter (ta, bulan) yang dibawa form.
	c.Request.URL.RawQuery = "ta=" + strconv.FormatUint(taID, 10) + "&bulan=" + strconv.Itoa(bulan)
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pemasukan tersimpan."}}`)
	h.refreshPemasukan(c)
}

func (h *KasHandler) PemasukanDelete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var row models.KasPemasukan
	if err := h.DB.First(&row, id).Error; err == nil {
		if h.taClosed(row.TahunAjaranID) {
			c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tahun ajaran sudah ditutup; transaksi dibekukan."}}`)
			h.refreshPemasukan(c)
			return
		}
		if row.Terkunci() {
			// Baris otomatis (SPP / dana bantuan) terkunci; kelola lewat modulnya.
			c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Baris otomatis (SPP/Bantuan) tidak bisa dihapus di sini."}}`)
			h.refreshPemasukan(c)
			return
		}
		h.DB.Delete(&models.KasPemasukan{}, id)
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Transaksi dihapus."}}`)
	}
	h.refreshPemasukan(c)
}

// =====================================================================
// PENGELUARAN
// =====================================================================

func (h *KasHandler) pengeluaranContext(c *gin.Context) gin.H {
	taID, bulan := h.resolveFilter(c)

	var list []models.KasPengeluaran
	q := h.DB.Preload("Kategori").Preload("SubKategori").Where("tahun_ajaran_id = ?", taID)
	if bulan >= 1 && bulan <= 12 {
		q = q.Where("MONTH(tanggal) = ?", bulan)
	}
	q.Order("tanggal desc, id desc").Find(&list)

	return gin.H{
		"SelectedTA": taID,
		"Bulan":      bulan,
		"TahunList":  tahunAjaranAll(h.DB),
		"Summary":    SummaryForTA(h.DB, taID),
		"List":       list,
		"HasTA":      taID != 0,
		"CanEdit":    canEditKas(c),
	}
}

func (h *KasHandler) PengeluaranIndex(c *gin.Context) {
	data := h.pengeluaranContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "kas/pengeluaran_content", data)
		return
	}
	data["Title"] = "Kas · Pengeluaran"
	data["ActiveMenu"] = "kas"
	data["ActiveTab"] = "pengeluaran"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "kas/pengeluaran", data)
}

func (h *KasHandler) PengeluaranForm(c *gin.Context) {
	taID, bulan := h.resolveFilter(c)
	var kategori []models.KategoriPengeluaran
	h.DB.Preload("SubList", func(db *gorm.DB) *gorm.DB {
		return db.Where("aktif = ?", true).Order("nama asc")
	}).Where("aktif = ?", true).Order("nama asc").Find(&kategori)
	c.HTML(http.StatusOK, "kas/pengeluaran_form", gin.H{
		"Title":        "Tambah Pengeluaran",
		"SelectedTA":   taID,
		"Bulan":        bulan,
		"KategoriList": kategori,
		"Today":        time.Now().Format("2006-01-02"),
	})
}

func (h *KasHandler) pengeluaranFormErr(c *gin.Context, msg string, taID uint64, bulan int) {
	var kategori []models.KategoriPengeluaran
	h.DB.Preload("SubList", func(db *gorm.DB) *gorm.DB {
		return db.Where("aktif = ?", true).Order("nama asc")
	}).Where("aktif = ?", true).Order("nama asc").Find(&kategori)
	c.HTML(http.StatusOK, "kas/pengeluaran_form", gin.H{
		"Title": "Tambah Pengeluaran", "Error": msg,
		"SelectedTA": taID, "Bulan": bulan, "KategoriList": kategori,
		"Today":    time.Now().Format("2006-01-02"),
		"Kategori": c.PostForm("kategori_id"), "Sub": c.PostForm("sub_kategori_id"),
		"Jumlah": c.PostForm("jumlah"), "Tanggal": c.PostForm("tanggal"), "Keterangan": c.PostForm("keterangan"),
	})
}

func (h *KasHandler) refreshPengeluaran(c *gin.Context) {
	c.HTML(http.StatusOK, "kas/pengeluaran_content_oob", h.pengeluaranContext(c))
}

func (h *KasHandler) PengeluaranCreate(c *gin.Context) {
	taID, _ := strconv.ParseUint(c.PostForm("ta"), 10, 64)
	bulan, _ := strconv.Atoi(c.PostForm("bulan"))
	if taID == 0 {
		taID = activeTahunAjaranID(h.DB)
	}
	kategoriID, _ := strconv.ParseUint(c.PostForm("kategori_id"), 10, 64)
	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())

	if taID == 0 {
		h.pengeluaranFormErr(c, "Belum ada tahun ajaran aktif. Aktifkan dulu di Master Data.", taID, bulan)
		return
	}
	if h.taClosed(taID) {
		h.pengeluaranFormErr(c, "Tahun ajaran ini sudah ditutup. Transaksi dibekukan.", taID, bulan)
		return
	}
	if kategoriID == 0 {
		h.pengeluaranFormErr(c, "Kategori wajib dipilih.", taID, bulan)
		return
	}
	if jumlah <= 0 {
		h.pengeluaranFormErr(c, "Jumlah harus lebih dari 0.", taID, bulan)
		return
	}

	row := models.KasPengeluaran{
		TahunAjaranID: taID,
		KategoriID:    kategoriID,
		Tanggal:       tanggal,
		Jumlah:        jumlah,
		Keterangan:    c.PostForm("keterangan"),
		UserID:        ctxUserID(c),
	}
	if v := c.PostForm("sub_kategori_id"); v != "" {
		if subID, err := strconv.ParseUint(v, 10, 64); err == nil && subID != 0 {
			row.SubKategoriID = &subID
		}
	}
	if err := h.DB.Create(&row).Error; err != nil {
		h.pengeluaranFormErr(c, "Gagal menyimpan transaksi.", taID, bulan)
		return
	}
	c.Request.URL.RawQuery = "ta=" + strconv.FormatUint(taID, 10) + "&bulan=" + strconv.Itoa(bulan)
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pengeluaran tersimpan."}}`)
	h.refreshPengeluaran(c)
}

func (h *KasHandler) PengeluaranDelete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var row models.KasPengeluaran
	if err := h.DB.First(&row, id).Error; err == nil {
		if h.taClosed(row.TahunAjaranID) {
			c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tahun ajaran sudah ditutup; transaksi dibekukan."}}`)
			h.refreshPengeluaran(c)
			return
		}
		h.DB.Delete(&models.KasPengeluaran{}, id)
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Transaksi dihapus."}}`)
	}
	h.refreshPengeluaran(c)
}
