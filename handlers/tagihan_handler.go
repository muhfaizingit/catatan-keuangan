package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/models"
	"teras-keuangan/util"
)

// TagihanHandler menangani tagihan non-SPP (insidental).
type TagihanHandler struct {
	DB *gorm.DB
}

func NewTagihanHandler(db *gorm.DB) *TagihanHandler {
	return &TagihanHandler{DB: db}
}

// ---------- helper ----------

func (h *TagihanHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

func (h *TagihanHandler) kelasByTA(taID uint64) []models.Kelas {
	var list []models.Kelas
	h.DB.Where("tahun_ajaran_id = ?", taID).Order("nama asc").Find(&list)
	return list
}

func (h *TagihanHandler) jenisAktif() []models.JenisTagihan {
	var list []models.JenisTagihan
	h.DB.Where("aktif = ?", true).Order("nama asc").Find(&list)
	return list
}

func (h *TagihanHandler) siswaByKelas(taID, kelasID uint64) []models.Siswa {
	var list []models.Siswa
	q := h.DB.Where("aktif = ?", true)
	if kelasID != 0 {
		q = q.Where("kelas_id = ?", kelasID)
	} else {
		var kelasIDs []uint64
		h.DB.Model(&models.Kelas{}).Where("tahun_ajaran_id = ?", taID).Pluck("id", &kelasIDs)
		if len(kelasIDs) == 0 {
			return list
		}
		q = q.Where("kelas_id IN ?", kelasIDs)
	}
	q.Order("nama asc").Find(&list)
	return list
}

func (h *TagihanHandler) paidMap(ids []uint64) map[uint64]int64 {
	res := map[uint64]int64{}
	if len(ids) == 0 {
		return res
	}
	type row struct {
		TagihanID uint64
		Total     int64
	}
	var rows []row
	h.DB.Model(&models.TagihanPembayaran{}).
		Select("tagihan_id, COALESCE(SUM(jumlah_bayar),0) as total").
		Where("tagihan_id IN ?", ids).Group("tagihan_id").Scan(&rows)
	for _, r := range rows {
		res[r.TagihanID] = r.Total
	}
	return res
}

func (h *TagihanHandler) recompute(tx *gorm.DB, tagihanID uint64, nominal int64) error {
	var paid int64
	tx.Model(&models.TagihanPembayaran{}).Where("tagihan_id = ?", tagihanID).
		Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.TagihanBelum
	if paid >= nominal {
		status = models.TagihanLunas
	} else if paid > 0 {
		status = models.TagihanCicil
	}
	return tx.Model(&models.Tagihan{}).Where("id = ?", tagihanID).Update("status", status).Error
}

func (h *TagihanHandler) recordBayar(tx *gorm.DB, t models.Tagihan, jumlah int64, tanggal time.Time, payKet, kasKet string, userID uint64) error {
	bayar := models.TagihanPembayaran{
		TagihanID: t.ID, Tanggal: tanggal, JumlahBayar: jumlah, Keterangan: payKet, UserID: userID,
	}
	if err := tx.Create(&bayar).Error; err != nil {
		return err
	}
	var jenis models.JenisPemasukan
	if err := tx.Where("kode = ?", models.KodeJenisTagihan).First(&jenis).Error; err != nil {
		return err
	}
	kas := models.KasPemasukan{
		TahunAjaranID: t.TahunAjaranID, JenisPemasukanID: jenis.ID, Tanggal: tanggal,
		Jumlah: jumlah, Keterangan: kasKet, UserID: userID, TagihanPembayaranID: &bayar.ID,
	}
	if err := tx.Create(&kas).Error; err != nil {
		return err
	}
	return h.recompute(tx, t.ID, t.Nominal)
}

// ---------- list ----------

type TagihanRow struct {
	Tagihan models.Tagihan
	Paid    int64
	Sisa    int64
}

func (h *TagihanHandler) listContext(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()
	kelasList, jenisList := []models.Kelas{}, h.jenisAktif()
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}

	jenisID, _ := strconv.ParseUint(c.Query("jenis"), 10, 64)
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if kelasID == 0 && len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}

	var list []models.Tagihan
	var totTagih, totBayar int64
	var nLunas, nCicil, nBelum int
	if hasTA {
		siswa := h.siswaByKelas(ta.ID, kelasID)
		ids := make([]uint64, len(siswa))
		for i, s := range siswa {
			ids[i] = s.ID
		}
		if len(ids) > 0 {
			q := h.DB.Preload("JenisTagihan").Preload("Siswa").
				Where("tahun_ajaran_id = ? AND siswa_id IN ?", ta.ID, ids)
			if jenisID != 0 {
				q = q.Where("jenis_tagihan_id = ?", jenisID)
			}
			q.Order("tanggal desc, id desc").Find(&list)
		}
	}
	tids := make([]uint64, len(list))
	for i, t := range list {
		tids[i] = t.ID
	}
	paid := h.paidMap(tids)

	rows := make([]TagihanRow, 0, len(list))
	for _, t := range list {
		p := paid[t.ID]
		sisa := t.Nominal - p
		if sisa < 0 {
			sisa = 0
		}
		rows = append(rows, TagihanRow{Tagihan: t, Paid: p, Sisa: sisa})
		totTagih += t.Nominal
		totBayar += p
		switch t.Status {
		case models.TagihanLunas:
			nLunas++
		case models.TagihanCicil:
			nCicil++
		default:
			nBelum++
		}
	}

	return gin.H{
		"HasTA": hasTA, "TANama": ta.Nama,
		"JenisList": jenisList, "KelasList": kelasList,
		"SelectedJenis": jenisID, "SelectedKelas": kelasID,
		"Rows": rows, "TotalTagih": totTagih, "TotalBayar": totBayar, "TotalSisa": totTagih - totBayar,
		"NLunas": nLunas, "NCicil": nCicil, "NBelum": nBelum,
		"CanEdit": canEditKas(c),
	}
}

func (h *TagihanHandler) Index(c *gin.Context) {
	data := h.listContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "tagihan/list_content", data)
		return
	}
	data["Title"] = "Tagihan"
	data["ActiveMenu"] = "tagihan"
	data["ActiveTab"] = "daftar"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "tagihan/index", data)
}

func (h *TagihanHandler) refreshList(c *gin.Context, jenisID, kelasID uint64) {
	c.Request.URL.RawQuery = "jenis=" + strconv.FormatUint(jenisID, 10) + "&kelas=" + strconv.FormatUint(kelasID, 10)
	c.HTML(http.StatusOK, "tagihan/list_content_oob", h.listContext(c))
}

// ---------- terbitkan ----------

func (h *TagihanHandler) TerbitkanForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	var kelasID uint64
	if len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}
	c.HTML(http.StatusOK, "tagihan/terbitkan_form", gin.H{
		"Title": "Terbitkan Tagihan", "HasTA": hasTA,
		"JenisList": h.jenisAktif(), "KelasList": kelasList,
		"SelectedKelas": kelasID, "SiswaList": h.siswaByKelas(ta.ID, kelasID),
		"Today": time.Now().Format("2006-01-02"),
	})
}

func (h *TagihanHandler) TerbitkanSiswaOptions(c *gin.Context) {
	ta, _ := h.activeTA()
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	c.HTML(http.StatusOK, "tagihan/terbitkan_siswa_options", gin.H{"SiswaList": h.siswaByKelas(ta.ID, kelasID)})
}

func (h *TagihanHandler) terbitkanErr(c *gin.Context, msg string) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	c.HTML(http.StatusOK, "tagihan/terbitkan_form", gin.H{
		"Title": "Terbitkan Tagihan", "Error": msg, "HasTA": hasTA,
		"JenisList": h.jenisAktif(), "KelasList": kelasList,
		"SelectedKelas": kelasID, "SiswaList": h.siswaByKelas(ta.ID, kelasID),
		"Today": time.Now().Format("2006-01-02"), "Jenis": c.PostForm("jenis_tagihan_id"),
	})
}

func (h *TagihanHandler) Terbitkan(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		h.terbitkanErr(c, "Belum ada tahun ajaran aktif.")
		return
	}
	jenisID, _ := strconv.ParseUint(c.PostForm("jenis_tagihan_id"), 10, 64)
	var jenis models.JenisTagihan
	if jenisID == 0 || h.DB.First(&jenis, jenisID).Error != nil {
		h.terbitkanErr(c, "Jenis tagihan wajib dipilih.")
		return
	}
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)

	var siswaIDs []uint64
	for _, s := range c.PostFormArray("siswa") {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			siswaIDs = append(siswaIDs, id)
		}
	}
	if len(siswaIDs) == 0 {
		h.terbitkanErr(c, "Pilih minimal satu siswa.")
		return
	}

	// Nominal default dipakai bila nominal per-siswa dikosongkan.
	nominalDefault := util.ParseRupiah(c.PostForm("nominal_default"))

	userID := ctxUserID(c)
	created, skipped := 0, 0
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for _, sid := range siswaIDs {
			nominal := util.ParseRupiah(c.PostForm("nominal_" + strconv.FormatUint(sid, 10)))
			if nominal <= 0 {
				nominal = nominalDefault // ikuti nominal default bila siswa dikosongkan
			}
			if nominal <= 0 {
				skipped++
				continue
			}
			t := models.Tagihan{
				JenisTagihanID: jenisID, SiswaID: sid, TahunAjaranID: ta.ID,
				Nominal: nominal, Tanggal: tanggal, Keterangan: c.PostForm("keterangan"),
				Status: models.TagihanBelum, UserID: userID,
			}
			if err := tx.Create(&t).Error; err != nil {
				return err
			}
			created++
		}
		return nil
	})
	if err != nil {
		h.terbitkanErr(c, "Gagal menerbitkan tagihan.")
		return
	}
	msg := strconv.Itoa(created) + " tagihan " + jenis.Nama + " diterbitkan."
	if skipped > 0 {
		msg += " (" + strconv.Itoa(skipped) + " dilewati karena nominal kosong)"
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`"}}`)
	h.refreshList(c, jenisID, kelasID)
}

// ---------- bayar ----------

func (h *TagihanHandler) BayarForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.Tagihan
	if err := h.DB.Preload("Siswa").Preload("JenisTagihan").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{t.ID})[t.ID]
	sisa := t.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	c.HTML(http.StatusOK, "tagihan/bayar_form", gin.H{
		"Title": "Bayar Tagihan", "Tagihan": t, "Paid": paid, "Sisa": sisa,
		"Today": time.Now().Format("2006-01-02"),
		"Jenis": c.Query("jenis"), "Kelas": c.Query("kelas"),
	})
}

func (h *TagihanHandler) bayarErr(c *gin.Context, t models.Tagihan, paid, sisa int64, msg string) {
	c.HTML(http.StatusOK, "tagihan/bayar_form", gin.H{
		"Title": "Bayar Tagihan", "Error": msg, "Tagihan": t, "Paid": paid, "Sisa": sisa,
		"Today": time.Now().Format("2006-01-02"),
		"Jenis": c.PostForm("jenis"), "Kelas": c.PostForm("kelas"), "Jumlah": c.PostForm("jumlah"),
	})
}

func (h *TagihanHandler) Bayar(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.Tagihan
	if err := h.DB.Preload("Siswa").Preload("JenisTagihan").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{t.ID})[t.ID]
	sisa := t.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	jenisID, _ := strconv.ParseUint(c.PostForm("jenis"), 10, 64)
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)

	if sisa <= 0 {
		h.bayarErr(c, t, paid, sisa, "Tagihan ini sudah lunas.")
		return
	}
	if jumlah <= 0 {
		h.bayarErr(c, t, paid, sisa, "Jumlah bayar harus lebih dari 0.")
		return
	}
	if jumlah > sisa {
		h.bayarErr(c, t, paid, sisa, "Jumlah bayar melebihi sisa ("+util.Rupiah(sisa)+").")
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		kasKet := t.JenisTagihan.Nama + " — " + t.Siswa.Nama
		return h.recordBayar(tx, t, jumlah, tanggal, c.PostForm("keterangan"), kasKet, ctxUserID(c))
	})
	if err != nil {
		h.bayarErr(c, t, paid, sisa, "Gagal menyimpan pembayaran.")
		return
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran tersimpan."}}`)
	h.refreshList(c, jenisID, kelasID)
}

// ---------- riwayat & hapus pembayaran ----------

func (h *TagihanHandler) riwayatData(c *gin.Context, tagihanID uint64) (gin.H, bool) {
	var t models.Tagihan
	if err := h.DB.Preload("Siswa").Preload("JenisTagihan").First(&t, tagihanID).Error; err != nil {
		return nil, false
	}
	var pays []models.TagihanPembayaran
	h.DB.Where("tagihan_id = ?", t.ID).Order("tanggal asc, id asc").Find(&pays)
	var paid int64
	for _, p := range pays {
		paid += p.JumlahBayar
	}
	sisa := t.Nominal - paid
	if sisa < 0 {
		sisa = 0
	}
	// TAClosed: tahun ajaran asal sudah ditutup (dikunci) -> riwayat read-only.
	var ta models.TahunAjaran
	taClosed := h.DB.First(&ta, t.TahunAjaranID).Error == nil && ta.Ditutup
	return gin.H{
		"Title": "Riwayat Tagihan", "Tagihan": t, "Payments": pays, "Paid": paid, "Sisa": sisa,
		"Jenis": c.Query("jenis"), "Kelas": c.Query("kelas"), "CanEdit": canEditKas(c),
		"TAClosed": taClosed,
	}, true
}

func (h *TagihanHandler) RiwayatForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, ok := h.riwayatData(c, id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.HTML(http.StatusOK, "tagihan/riwayat", data)
}

func (h *TagihanHandler) HapusPembayaran(c *gin.Context) {
	payID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var pay models.TagihanPembayaran
	if err := h.DB.First(&pay, payID).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	tagihanID := pay.TagihanID
	h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tagihan_pembayaran_id = ?", payID).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.TagihanPembayaran{}, payID).Error; err != nil {
			return err
		}
		var t models.Tagihan
		if err := tx.First(&t, tagihanID).Error; err != nil {
			return err
		}
		return h.recompute(tx, tagihanID, t.Nominal)
	})
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran dihapus & kas disesuaikan."}}`)

	data, ok := h.riwayatData(c, tagihanID)
	jenisID, _ := strconv.ParseUint(c.Query("jenis"), 10, 64)
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if !ok {
		h.refreshList(c, jenisID, kelasID)
		return
	}
	c.Request.URL.RawQuery = "jenis=" + strconv.FormatUint(jenisID, 10) + "&kelas=" + strconv.FormatUint(kelasID, 10)
	data["List"] = h.listContext(c)
	c.HTML(http.StatusOK, "tagihan/riwayat_after", data)
}

// ---------- hapus tagihan (reverse penuh) ----------

func (h *TagihanHandler) HapusTagihan(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	jenisID, _ := strconv.ParseUint(c.Query("jenis"), 10, 64)
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)

	h.DB.Transaction(func(tx *gorm.DB) error {
		var pays []models.TagihanPembayaran
		tx.Where("tagihan_id = ?", id).Find(&pays)
		for _, p := range pays {
			if err := tx.Where("tagihan_pembayaran_id = ?", p.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("tagihan_id = ?", id).Delete(&models.TagihanPembayaran{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Tagihan{}, id).Error
	})
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Tagihan dihapus (pembayaran & kas ikut ditarik)."}}`)
	h.refreshList(c, jenisID, kelasID)
}

// ---------- tunggakan (per kelas, semua jenis) ----------

type TagihanTunggakanRow struct {
	Siswa      models.Siswa
	TotalTagih int64
	TotalBayar int64
	Tunggakan  int64
	NBelum     int
}

func (h *TagihanHandler) tunggakanContext(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if kelasID == 0 && len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}

	var siswaList []models.Siswa
	if kelasID != 0 {
		h.DB.Where("kelas_id = ? AND aktif = ?", kelasID, true).Order("nama asc").Find(&siswaList)
	}

	var rows []TagihanTunggakanRow
	var grand int64
	if hasTA && len(siswaList) > 0 {
		ids := make([]uint64, len(siswaList))
		for i, s := range siswaList {
			ids[i] = s.ID
		}
		var tagihan []models.Tagihan
		h.DB.Where("tahun_ajaran_id = ? AND siswa_id IN ?", ta.ID, ids).Find(&tagihan)
		var allIDs []uint64
		for _, t := range tagihan {
			allIDs = append(allIDs, t.ID)
		}
		paid := h.paidMap(allIDs)
		type agg struct {
			tagih, bayar int64
			nBelum       int
		}
		byS := map[uint64]*agg{}
		for _, t := range tagihan {
			a := byS[t.SiswaID]
			if a == nil {
				a = &agg{}
				byS[t.SiswaID] = a
			}
			a.tagih += t.Nominal
			a.bayar += paid[t.ID]
			if t.Status != models.TagihanLunas {
				a.nBelum++
			}
		}
		for _, s := range siswaList {
			a := byS[s.ID]
			if a == nil {
				a = &agg{}
			}
			tg := a.tagih - a.bayar
			if tg < 0 {
				tg = 0
			}
			grand += tg
			rows = append(rows, TagihanTunggakanRow{
				Siswa: s, TotalTagih: a.tagih, TotalBayar: a.bayar, Tunggakan: tg, NBelum: a.nBelum,
			})
		}
	}

	return gin.H{
		"HasTA": hasTA, "TANama": ta.Nama, "KelasList": kelasList,
		"SelectedKelas": kelasID, "Rows": rows, "GrandTunggakan": grand,
		"CanEdit": canEditKas(c),
	}
}

func (h *TagihanHandler) TunggakanIndex(c *gin.Context) {
	data := h.tunggakanContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "tagihan/tunggakan_content", data)
		return
	}
	data["Title"] = "Tagihan · Tunggakan"
	data["ActiveMenu"] = "tagihan"
	data["ActiveTab"] = "tunggakan"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "tagihan/tunggakan", data)
}
