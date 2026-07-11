package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"teras-keuangan/models"
	"teras-keuangan/util"
)

// GajiHandler menangani kewajiban & pembayaran gaji guru (payable sekolah).
type GajiHandler struct {
	DB *gorm.DB
}

func NewGajiHandler(db *gorm.DB) *GajiHandler {
	return &GajiHandler{DB: db}
}

func (h *GajiHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

// paidMap mengembalikan total terbayar per gaji untuk sekumpulan id.
func (h *GajiHandler) paidMap(gajiIDs []uint64) map[uint64]int64 {
	res := map[uint64]int64{}
	if len(gajiIDs) == 0 {
		return res
	}
	type row struct {
		GajiID uint64
		Total  int64
	}
	var rows []row
	h.DB.Model(&models.GajiPembayaran{}).
		Select("gaji_id, COALESCE(SUM(jumlah_bayar),0) as total").
		Where("gaji_id IN ?", gajiIDs).Group("gaji_id").Scan(&rows)
	for _, r := range rows {
		res[r.GajiID] = r.Total
	}
	return res
}

// recomputeStatus menghitung ulang status gaji dari total pembayaran.
func (h *GajiHandler) recompute(tx *gorm.DB, gajiID uint64, jumlah int64) error {
	var paid int64
	tx.Model(&models.GajiPembayaran{}).Where("gaji_id = ?", gajiID).
		Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.GajiBelum
	if paid >= jumlah {
		status = models.GajiLunas
	} else if paid > 0 {
		status = models.GajiCicil
	}
	return tx.Model(&models.GajiGuru{}).Where("id = ?", gajiID).Update("status", status).Error
}

// subGaji menemukan/membuat sub-kategori pengeluaran "Gaji <nama guru>" agar
// setiap pembayaran gaji tercatat rapi di Kas Pengeluaran.
func (h *GajiHandler) subGaji(tx *gorm.DB, kategoriID uint64, guru models.Guru) (uint64, error) {
	nama := "Gaji " + guru.Nama
	var sub models.SubKategoriPengeluaran
	err := tx.Where("kategori_id = ? AND nama = ?", kategoriID, nama).First(&sub).Error
	if err == nil {
		return sub.ID, nil
	}
	sub = models.SubKategoriPengeluaran{KategoriID: kategoriID, Nama: nama, Aktif: true}
	if err := tx.Create(&sub).Error; err != nil {
		return 0, err
	}
	return sub.ID, nil
}

// ---------- list ----------

type GajiRow struct {
	Gaji   models.GajiGuru
	Guru   models.Guru
	Paid   int64
	Sisa  int64
}

func (h *GajiHandler) listContext(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()
	guruList := []models.Guru{}
	if hasTA {
		h.DB.Where("aktif = ?", true).Order("nama asc").Find(&guruList)
	}

	bulan, _ := strconv.Atoi(c.Query("bulan"))
	if bulan < 1 || bulan > 12 {
		bulan = int(time.Now().Month())
	}

	var rows []GajiRow
	var totalGaji, totalBayar, totalSisa int64
	var nLunas, nCicil, nBelum int
	if hasTA && len(guruList) > 0 {
		ids := make([]uint64, len(guruList))
		for i, g := range guruList {
			ids[i] = g.ID
		}
		var gaji []models.GajiGuru
		h.DB.Preload("Guru").Where("tahun_ajaran_id = ? AND bulan = ? AND guru_id IN ?", ta.ID, bulan, ids).Find(&gaji)
		gajiByGuru := map[uint64]models.GajiGuru{}
		for _, gg := range gaji {
			gajiByGuru[gg.GuruID] = gg
		}
		gajiIDs := make([]uint64, 0, len(gaji))
		for _, gg := range gaji {
			gajiIDs = append(gajiIDs, gg.ID)
		}
		paid := h.paidMap(gajiIDs)
		for _, g := range guruList {
			row := GajiRow{Guru: g}
			if gg, ok := gajiByGuru[g.ID]; ok {
				row.Gaji = gg
				pd := paid[gg.ID]
				sisa := gg.Jumlah - pd
				if sisa < 0 {
					sisa = 0
				}
				row.Paid = pd
				row.Sisa = sisa
				totalGaji += gg.Jumlah
				totalBayar += pd
				totalSisa += sisa
				switch gg.Status {
				case models.GajiLunas:
					nLunas++
				case models.GajiCicil:
					nCicil++
				default:
					nBelum++
				}
			}
			rows = append(rows, row)
		}
	}

	return gin.H{
		"HasTA":      hasTA,
		"TANama":     ta.Nama,
		"Bulan":      bulan,
		"Rows":       rows,
		"TotalGaji":  totalGaji,
		"TotalBayar": totalBayar,
		"TotalSisa":  totalSisa,
		"NLunas":     nLunas,
		"NCicil":     nCicil,
		"NBelum":     nBelum,
		"CanEdit":    canEditKas(c),
	}
}

func (h *GajiHandler) Index(c *gin.Context) {
	data := h.listContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "gaji/content", data)
		return
	}
	data["Title"] = "Gaji Guru"
	data["ActiveMenu"] = "gaji"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "gaji/index", data)
}

func (h *GajiHandler) refresh(c *gin.Context) {
	c.HTML(http.StatusOK, "gaji/content_oob", h.listContext(c))
}

// ---------- generate tagihan gaji bulanan ----------

func (h *GajiHandler) GenerateForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	guruList := []models.Guru{}
	if hasTA {
		h.DB.Where("aktif = ?", true).Order("nama asc").Find(&guruList)
	}
	bulan, _ := strconv.Atoi(c.Query("bulan"))
	if bulan < 1 || bulan > 12 {
		bulan = int(time.Now().Month())
	}
	c.HTML(http.StatusOK, "gaji/generate_form", gin.H{
		"Title": "Terbitkan Gaji Bulanan", "HasTA": hasTA, "TANama": ta.Nama,
		"Bulan": bulan, "GuruList": guruList, "Today": time.Now().Format("2006-01-02"),
	})
}

func (h *GajiHandler) Generate(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Belum ada tahun ajaran aktif."}}`)
		h.refresh(c)
		return
	}
	bulan, _ := strconv.Atoi(c.PostForm("bulan"))
	if bulan < 1 || bulan > 12 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Bulan tidak valid."}}`)
		h.refresh(c)
		return
	}
	guruIDs := []uint64{}
	for _, s := range c.PostFormArray("guru_id") {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			guruIDs = append(guruIDs, id)
		}
	}
	if len(guruIDs) == 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Pilih minimal satu guru."}}`)
		h.refresh(c)
		return
	}

	created := 0
	h.DB.Transaction(func(tx *gorm.DB) error {
		for _, gid := range guruIDs {
			nominal := util.ParseRupiah(c.PostForm("nominal_"+strconv.FormatUint(gid, 10)))
			if nominal <= 0 {
				continue
			}
			g := models.GajiGuru{
				GuruID: gid, TahunAjaranID: ta.ID, Bulan: bulan,
				Jumlah: nominal, Status: models.GajiBelum, UserID: ctxUserID(c),
				Keterangan: c.PostForm("keterangan"),
			}
			// Lewati bila sudah ada (unique guru+ta+bulan).
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&g)
			if res.RowsAffected > 0 {
				created++
			}
		}
		return nil
	})
	msg := strconv.Itoa(created) + " kewajiban gaji dibuat."
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`"}}`)
	h.refresh(c)
}

// ---------- bayar (cicil) ----------

func (h *GajiHandler) BayarForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var g models.GajiGuru
	if err := h.DB.Preload("Guru").First(&g, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{g.ID})[g.ID]
	sisa := g.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}
	ta, hasTA := h.activeTA()
	c.HTML(http.StatusOK, "gaji/bayar_form", gin.H{
		"Title": "Bayar Gaji", "Gaji": g, "Paid": paid, "Sisa": sisa,
		"HasTA": hasTA, "TANama": ta.Nama, "Bulan": c.Query("bulan"),
		"Today": time.Now().Format("2006-01-02"),
	})
}

func (h *GajiHandler) bayarErr(c *gin.Context, g models.GajiGuru, paid, sisa int64, msg string) {
	ta, hasTA := h.activeTA()
	c.HTML(http.StatusOK, "gaji/bayar_form", gin.H{
		"Title": "Bayar Gaji", "Error": msg, "Gaji": g, "Paid": paid, "Sisa": sisa,
		"HasTA": hasTA, "TANama": ta.Nama, "Bulan": c.PostForm("bulan"),
		"Jumlah": c.PostForm("jumlah"),
	})
}

func (h *GajiHandler) Bayar(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var g models.GajiGuru
	if err := h.DB.Preload("Guru").First(&g, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	_, hasTA := h.activeTA()
	paid := h.paidMap([]uint64{g.ID})[g.ID]
	sisa := g.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}

	if !hasTA {
		h.bayarErr(c, g, paid, sisa, "Belum ada tahun ajaran aktif untuk mencatat kas.")
		return
	}
	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())

	if sisa <= 0 {
		h.bayarErr(c, g, paid, sisa, "Kewajiban gaji ini sudah lunas.")
		return
	}
	if jumlah <= 0 {
		h.bayarErr(c, g, paid, sisa, "Jumlah bayar harus lebih dari 0.")
		return
	}
	if jumlah > sisa {
		h.bayarErr(c, g, paid, sisa, "Jumlah bayar melebihi sisa ("+util.Rupiah(sisa)+").")
		return
	}

	userID := ctxUserID(c)
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		bayar := models.GajiPembayaran{
			GajiID: g.ID, Tanggal: tanggal, JumlahBayar: jumlah,
			Keterangan: c.PostForm("keterangan"), UserID: userID,
		}
		if err := tx.Create(&bayar).Error; err != nil {
			return err
		}
		// Auto-posting ke Kas Pengeluaran (kategori Gaji, sub = nama guru).
		var kat models.KategoriPengeluaran
		if err := tx.Where("kode = ?", models.KodeKategoriGaji).First(&kat).Error; err != nil {
			return err
		}
		subID, err := h.subGaji(tx, kat.ID, g.Guru)
		if err != nil {
			return err
		}
		subIDPtr := subID
		kas := models.KasPengeluaran{
			TahunAjaranID: g.TahunAjaranID, KategoriID: kat.ID, SubKategoriID: &subIDPtr,
			Tanggal: tanggal, Jumlah: jumlah,
			Keterangan: "Gaji " + g.Guru.Nama + " · " + util.NamaBulan(g.Bulan),
			UserID: userID,
		}
		if err := tx.Create(&kas).Error; err != nil {
			return err
		}
		// Tautkan baris kas ke pembayaran (untuk reverse saat hapus).
		if err := tx.Model(&bayar).Update("kas_pengeluaran_id", kas.ID).Error; err != nil {
			return err
		}
		return h.recompute(tx, g.ID, g.Jumlah)
	})
	if err != nil {
		h.bayarErr(c, g, paid, sisa, "Gagal menyimpan pembayaran.")
		return
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran gaji tersimpan (masuk Kas Pengeluaran)."}}`)
	h.refresh(c)
}

// ---------- riwayat & hapus pembayaran ----------

func (h *GajiHandler) riwayatData(c *gin.Context, gajiID uint64) (gin.H, bool) {
	var g models.GajiGuru
	if err := h.DB.Preload("Guru").First(&g, gajiID).Error; err != nil {
		return nil, false
	}
	var pays []models.GajiPembayaran
	h.DB.Where("gaji_id = ?", g.ID).Order("tanggal asc, id asc").Find(&pays)
	var paid int64
	for _, x := range pays {
		paid += x.JumlahBayar
	}
	sisa := g.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}
	return gin.H{
		"Title": "Riwayat Gaji", "Gaji": g, "Payments": pays, "Paid": paid, "Sisa": sisa,
		"Bulan": c.Query("bulan"), "CanEdit": canEditKas(c),
	}, true
}

func (h *GajiHandler) RiwayatForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, ok := h.riwayatData(c, id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.HTML(http.StatusOK, "gaji/riwayat", data)
}

func (h *GajiHandler) HapusPembayaran(c *gin.Context) {
	payID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var pay models.GajiPembayaran
	if err := h.DB.First(&pay, payID).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	gajiID := pay.GajiID
	h.DB.Transaction(func(tx *gorm.DB) error {
		if pay.KasPengeluaranID != nil {
			if err := tx.Delete(&models.KasPengeluaran{}, *pay.KasPengeluaranID).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&models.GajiPembayaran{}, payID).Error; err != nil {
			return err
		}
		var g models.GajiGuru
		if err := tx.First(&g, gajiID).Error; err != nil {
			return err
		}
		return h.recompute(tx, gajiID, g.Jumlah)
	})
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran dihapus & kas disesuaikan."}}`)

	data, ok := h.riwayatData(c, gajiID)
	if !ok {
		h.refresh(c)
		return
	}
	c.Request.URL.RawQuery = "bulan=" + c.Query("bulan")
	data["List"] = h.listContext(c)
	c.HTML(http.StatusOK, "gaji/riwayat_after", data)
}
