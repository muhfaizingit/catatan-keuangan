package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/models"
	"teras-keuangan/util"
)

// PiutangHandler menangani penagihan piutang (tunggakan tahun lalu).
type PiutangHandler struct {
	DB *gorm.DB
}

func NewPiutangHandler(db *gorm.DB) *PiutangHandler {
	return &PiutangHandler{DB: db}
}

func (h *PiutangHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

func (h *PiutangHandler) paidMap(ids []uint64) map[uint64]int64 {
	res := map[uint64]int64{}
	if len(ids) == 0 {
		return res
	}
	type row struct {
		PiutangID uint64
		Total     int64
	}
	var rows []row
	h.DB.Model(&models.PiutangPembayaran{}).
		Select("piutang_id, COALESCE(SUM(jumlah_bayar),0) as total").
		Where("piutang_id IN ?", ids).Group("piutang_id").Scan(&rows)
	for _, r := range rows {
		res[r.PiutangID] = r.Total
	}
	return res
}

func (h *PiutangHandler) recompute(tx *gorm.DB, piutangID uint64, nominal int64) error {
	var paid int64
	tx.Model(&models.PiutangPembayaran{}).Where("piutang_id = ?", piutangID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.TagihanBelum
	if paid >= nominal {
		status = models.TagihanLunas
	} else if paid > 0 {
		status = models.TagihanCicil
	}
	return tx.Model(&models.Piutang{}).Where("id = ?", piutangID).Update("status", status).Error
}

type PiutangRow struct {
	Piutang     models.Piutang
	Paid        int64
	Sisa        int64
	SumberLabel string
	SumberURL   string
	// LunasInfo terisi bila piutang sudah lunas: keterangan TA & tanggal bayar terakhir.
	LunasInfo string
}

// sumberInfo membangun label & URL (read-only) penelusuran asal tunggakan di TA lama.
func sumberInfo(p models.Piutang) (string, string) {
	switch p.SumberTipe {
	case models.SumberPiutangSPP:
		if p.SumberID == nil {
			return "SPP", ""
		}
		return "SPP", fmt.Sprintf("/spp/tagihan/%d/riwayat", *p.SumberID)
	case models.SumberPiutangTagihan:
		if p.SumberID == nil {
			return "Tagihan", ""
		}
		return "Tagihan", fmt.Sprintf("/tagihan/%d/riwayat", *p.SumberID)
	default:
		return "Carry-over", "" // piutang lama tanpa jejak sumber
	}
}

// lunasInfo mengambil keterangan pembayaran terakhir bila piutang sudah lunas:
// "Lunas · <TA bayar> · <tanggal>". Opsi 1 murni: sumber (TA lama) tetap immutable.
func (h *PiutangHandler) lunasInfo(p models.Piutang) string {
	if p.Status != models.TagihanLunas {
		return ""
	}
	var last models.PiutangPembayaran
	if err := h.DB.Preload("TahunAjaranBayar").
		Where("piutang_id = ?", p.ID).Order("tanggal desc, id desc").First(&last).Error; err != nil {
		return "Lunas"
	}
	taNama := last.TahunAjaranBayar.Nama
	if taNama == "" {
		taNama = "—"
	}
	return "Lunas · TA " + taNama + " · " + last.Tanggal.Format("02 Jan 2006")
}

func (h *PiutangHandler) listContext(c *gin.Context) gin.H {
	filter := c.Query("f")
	if filter == "" {
		filter = "belum"
	}
	q := h.DB.Preload("Siswa").Preload("TahunAjaranAsal")
	if filter == "belum" {
		q = q.Where("status <> ?", models.TagihanLunas)
	}
	var list []models.Piutang
	q.Order("status asc, id desc").Find(&list)

	ids := make([]uint64, len(list))
	for i, p := range list {
		ids[i] = p.ID
	}
	paid := h.paidMap(ids)

	rows := make([]PiutangRow, 0, len(list))
	var totNominal, totBayar int64
	for _, p := range list {
		pd := paid[p.ID]
		sisa := p.Nominal - pd
		if sisa < 0 {
			sisa = 0
		}
		lbl, url := sumberInfo(p)
		rows = append(rows, PiutangRow{Piutang: p, Paid: pd, Sisa: sisa, SumberLabel: lbl, SumberURL: url, LunasInfo: h.lunasInfo(p)})
		totNominal += p.Nominal
		totBayar += pd
	}

	_, hasTA := h.activeTA()
	return gin.H{
		"Rows": rows, "Filter": filter,
		"TotalNominal": totNominal, "TotalBayar": totBayar, "TotalSisa": totNominal - totBayar,
		"HasTA": hasTA, "CanEdit": canEditKas(c),
	}
}

func (h *PiutangHandler) Index(c *gin.Context) {
	data := h.listContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "piutang/content", data)
		return
	}
	data["Title"] = "Piutang"
	data["ActiveMenu"] = "piutang"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "piutang/index", data)
}

func (h *PiutangHandler) refresh(c *gin.Context, filter string) {
	c.Request.URL.RawQuery = "f=" + filter
	c.HTML(http.StatusOK, "piutang/content_oob", h.listContext(c))
}

func (h *PiutangHandler) BayarForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p models.Piutang
	if err := h.DB.Preload("Siswa").Preload("TahunAjaranAsal").First(&p, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{p.ID})[p.ID]
	sisa := p.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	ta, hasTA := h.activeTA()
	c.HTML(http.StatusOK, "piutang/bayar_form", gin.H{
		"Title": "Bayar Piutang", "Piutang": p, "Paid": paid, "Sisa": sisa,
		"HasTA": hasTA, "TANama": ta.Nama, "Today": time.Now().Format("2006-01-02"),
		"Filter": c.Query("f"),
	})
}

func (h *PiutangHandler) bayarErr(c *gin.Context, p models.Piutang, paid, sisa int64, msg string) {
	ta, hasTA := h.activeTA()
	c.HTML(http.StatusOK, "piutang/bayar_form", gin.H{
		"Title": "Bayar Piutang", "Error": msg, "Piutang": p, "Paid": paid, "Sisa": sisa,
		"HasTA": hasTA, "TANama": ta.Nama, "Today": time.Now().Format("2006-01-02"),
		"Filter": c.PostForm("f"), "Jumlah": c.PostForm("jumlah"),
	})
}

func (h *PiutangHandler) Bayar(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p models.Piutang
	if err := h.DB.Preload("Siswa").First(&p, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ta, hasTA := h.activeTA()
	paid := h.paidMap([]uint64{p.ID})[p.ID]
	sisa := p.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	if !hasTA {
		h.bayarErr(c, p, paid, sisa, "Belum ada tahun ajaran aktif untuk mencatat kas.")
		return
	}
	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	filter := c.PostForm("f")

	if sisa <= 0 {
		h.bayarErr(c, p, paid, sisa, "Piutang ini sudah lunas.")
		return
	}
	if jumlah <= 0 {
		h.bayarErr(c, p, paid, sisa, "Jumlah bayar harus lebih dari 0.")
		return
	}
	if jumlah > sisa {
		h.bayarErr(c, p, paid, sisa, "Jumlah bayar melebihi sisa ("+util.Rupiah(sisa)+").")
		return
	}

	userID := ctxUserID(c)
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		bayar := models.PiutangPembayaran{
			PiutangID: p.ID, Tanggal: tanggal, JumlahBayar: jumlah,
			TahunAjaranBayarID: ta.ID, Keterangan: c.PostForm("keterangan"), UserID: userID,
		}
		if err := tx.Create(&bayar).Error; err != nil {
			return err
		}
		var jenis models.JenisPemasukan
		if err := tx.Where("kode = ?", models.KodeJenisPiutang).First(&jenis).Error; err != nil {
			return err
		}
		kas := models.KasPemasukan{
			TahunAjaranID: ta.ID, JenisPemasukanID: jenis.ID, Tanggal: tanggal, Jumlah: jumlah,
			Keterangan: "Piutang " + p.Siswa.Nama, UserID: userID, PiutangPembayaranID: &bayar.ID,
		}
		if err := tx.Create(&kas).Error; err != nil {
			return err
		}
		return h.recompute(tx, p.ID, p.Nominal)
	})
	if err != nil {
		h.bayarErr(c, p, paid, sisa, "Gagal menyimpan pembayaran.")
		return
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran piutang tersimpan (masuk kas TA aktif)."}}`)
	h.refresh(c, filter)
}

func (h *PiutangHandler) riwayatData(c *gin.Context, piutangID uint64) (gin.H, bool) {
	var p models.Piutang
	if err := h.DB.Preload("Siswa").Preload("TahunAjaranAsal").First(&p, piutangID).Error; err != nil {
		return nil, false
	}
	var pays []models.PiutangPembayaran
	h.DB.Where("piutang_id = ?", p.ID).Order("tanggal asc, id asc").Find(&pays)
	var paid int64
	for _, x := range pays {
		paid += x.JumlahBayar
	}
	sisa := p.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	lbl, url := sumberInfo(p)
	return gin.H{
		"Title": "Riwayat Piutang", "Piutang": p, "Payments": pays, "Paid": paid, "Sisa": sisa,
		"Filter": c.Query("f"), "CanEdit": canEditKas(c),
		"SumberLabel": lbl, "SumberURL": url,
		"LunasInfo": h.lunasInfo(p),
	}, true
}

func (h *PiutangHandler) RiwayatForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, ok := h.riwayatData(c, id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.HTML(http.StatusOK, "piutang/riwayat", data)
}

func (h *PiutangHandler) HapusPembayaran(c *gin.Context) {
	payID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var pay models.PiutangPembayaran
	if err := h.DB.First(&pay, payID).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	piutangID := pay.PiutangID
	h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("piutang_pembayaran_id = ?", payID).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.PiutangPembayaran{}, payID).Error; err != nil {
			return err
		}
		var p models.Piutang
		if err := tx.First(&p, piutangID).Error; err != nil {
			return err
		}
		return h.recompute(tx, piutangID, p.Nominal)
	})
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran dihapus & kas disesuaikan."}}`)

	data, ok := h.riwayatData(c, piutangID)
	filter := c.Query("f")
	if !ok {
		h.refresh(c, filter)
		return
	}
	c.Request.URL.RawQuery = "f=" + filter
	data["List"] = h.listContext(c)
	c.HTML(http.StatusOK, "piutang/riwayat_after", data)
}
