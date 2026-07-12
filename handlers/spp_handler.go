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

// SppHandler menangani tagihan & pembayaran SPP.
type SppHandler struct {
	DB *gorm.DB
}

func NewSppHandler(db *gorm.DB) *SppHandler {
	return &SppHandler{DB: db}
}

// ---------- helper umum ----------

func (h *SppHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

func (h *SppHandler) kelasByTA(taID uint64) []models.Kelas {
	var list []models.Kelas
	h.DB.Where("tahun_ajaran_id = ?", taID).Order("nama asc").Find(&list)
	return list
}

// paidMap mengembalikan total terbayar per tagihan untuk sekumpulan id.
func (h *SppHandler) paidMap(tagihanIDs []uint64) map[uint64]int64 {
	res := map[uint64]int64{}
	if len(tagihanIDs) == 0 {
		return res
	}
	type row struct {
		TagihanID uint64
		Total     int64
	}
	var rows []row
	h.DB.Model(&models.SppPembayaran{}).
		Select("tagihan_id, COALESCE(SUM(jumlah_bayar),0) as total").
		Where("tagihan_id IN ?", tagihanIDs).
		Group("tagihan_id").Scan(&rows)
	for _, r := range rows {
		res[r.TagihanID] = r.Total
	}
	return res
}

// recomputeStatus menghitung ulang status tagihan dari total pembayaran.
func (h *SppHandler) recomputeStatus(tx *gorm.DB, tagihanID uint64, jumlahTagihan int64) error {
	var paid int64
	tx.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihanID).
		Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.SPPBelum
	if paid >= jumlahTagihan {
		status = models.SPPLunas
	} else if paid > 0 {
		status = models.SPPCicil
	}
	return tx.Model(&models.SppTagihan{}).Where("id = ?", tagihanID).Update("status", status).Error
}

// recordPembayaran membuat satu pembayaran SPP + baris kas "SPP" tertaut, lalu
// menghitung ulang status. Dipakai pembayaran tunai maupun alokasi bantuan.
func (h *SppHandler) recordPembayaran(tx *gorm.DB, t models.SppTagihan, jumlah int64, tanggal time.Time, payKet, kasKet string, userID uint64, sumber string, danaID *uint64) error {
	bayar := models.SppPembayaran{
		TagihanID: t.ID, Tanggal: tanggal, JumlahBayar: jumlah,
		Keterangan: payKet, Sumber: sumber, DanaBantuanID: danaID, UserID: userID,
	}
	if err := tx.Create(&bayar).Error; err != nil {
		return err
	}
	var jenis models.JenisPemasukan
	if err := tx.Where("kode = ?", models.KodeJenisSPP).First(&jenis).Error; err != nil {
		return err
	}
	kas := models.KasPemasukan{
		TahunAjaranID: t.TahunAjaranID, JenisPemasukanID: jenis.ID,
		Tanggal: tanggal, Jumlah: jumlah, Keterangan: kasKet,
		UserID: userID, SppPembayaranID: &bayar.ID, DanaBantuanID: danaID,
	}
	if err := tx.Create(&kas).Error; err != nil {
		return err
	}
	return h.recomputeStatus(tx, t.ID, t.Jumlah)
}

// PembayaranRow adalah satu baris siswa pada tab pembayaran.
type PembayaranRow struct {
	Siswa      models.Siswa
	HasTagihan bool
	Tagihan    models.SppTagihan
	Paid       int64
	Sisa       int64
}

// =====================================================================
// PEMBAYARAN (list per kelas + bulan)
// =====================================================================

func (h *SppHandler) pembayaranContext(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}

	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if kelasID == 0 && len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}
	bulan, _ := strconv.Atoi(c.Query("bulan"))
	if bulan < 1 || bulan > 12 {
		bulan = int(time.Now().Month())
	}

	var siswaList []models.Siswa
	if kelasID != 0 {
		h.DB.Where("kelas_id = ? AND aktif = ?", kelasID, true).Order("nama asc").Find(&siswaList)
	}

	// Tagihan bulan ini untuk siswa di kelas.
	tagihanBySiswa := map[uint64]models.SppTagihan{}
	var tagihanIDs []uint64
	if hasTA && len(siswaList) > 0 {
		ids := make([]uint64, len(siswaList))
		for i, s := range siswaList {
			ids[i] = s.ID
		}
		var tagihan []models.SppTagihan
		h.DB.Where("tahun_ajaran_id = ? AND bulan = ? AND siswa_id IN ?", ta.ID, bulan, ids).Find(&tagihan)
		for _, t := range tagihan {
			tagihanBySiswa[t.SiswaID] = t
			tagihanIDs = append(tagihanIDs, t.ID)
		}
	}
	paid := h.paidMap(tagihanIDs)

	var rows []PembayaranRow
	var totalTagih, totalBayar int64
	var nLunas, nCicil, nBelum int
	for _, s := range siswaList {
		r := PembayaranRow{Siswa: s}
		if t, ok := tagihanBySiswa[s.ID]; ok {
			r.HasTagihan = true
			r.Tagihan = t
			r.Paid = paid[t.ID]
			r.Sisa = t.Jumlah - r.Paid
			if r.Sisa < 0 {
				r.Sisa = 0
			}
			totalTagih += t.Jumlah
			totalBayar += r.Paid
			switch t.Status {
			case models.SPPLunas:
				nLunas++
			case models.SPPCicil:
				nCicil++
			default:
				nBelum++
			}
		}
		rows = append(rows, r)
	}

	return gin.H{
		"HasTA":      hasTA,
		"TANama":     ta.Nama,
		"KelasList":  kelasList,
		"SelectedKelas": kelasID,
		"Bulan":      bulan,
		"Rows":       rows,
		"TotalTagih": totalTagih,
		"TotalBayar": totalBayar,
		"TotalSisa":  totalTagih - totalBayar,
		"NLunas":     nLunas,
		"NCicil":     nCicil,
		"NBelum":     nBelum,
		"CanEdit":    canEditKas(c),
	}
}

func (h *SppHandler) PembayaranIndex(c *gin.Context) {
	data := h.pembayaranContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "spp/pembayaran_content", data)
		return
	}
	data["Title"] = "SPP · Pembayaran"
	data["ActiveMenu"] = "spp"
	data["ActiveTab"] = "pembayaran"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "spp/pembayaran", data)
}

func (h *SppHandler) refreshPembayaran(c *gin.Context, kelasID uint64, bulan int) {
	c.Request.URL.RawQuery = "kelas=" + strconv.FormatUint(kelasID, 10) + "&bulan=" + strconv.Itoa(bulan)
	c.HTML(http.StatusOK, "spp/pembayaran_content_oob", h.pembayaranContext(c))
}

// =====================================================================
// GENERATE TAGIHAN
// =====================================================================

func (h *SppHandler) GenerateForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	bulan, _ := strconv.Atoi(c.Query("bulan"))
	c.HTML(http.StatusOK, "spp/generate_form", gin.H{
		"Title":         "Generate Tagihan SPP",
		"HasTA":         hasTA,
		"TANama":        ta.Nama,
		"KelasList":     kelasList,
		"SelectedKelas": kelasID,
		"Bulan":         bulan,
	})
}

func (h *SppHandler) generateFormErr(c *gin.Context, msg string) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	c.HTML(http.StatusOK, "spp/generate_form", gin.H{
		"Title": "Generate Tagihan SPP", "Error": msg,
		"HasTA": hasTA, "TANama": ta.Nama, "KelasList": kelasList,
		"SelectedKelas": kelasID, "Nominal": c.PostForm("nominal"),
	})
}

func (h *SppHandler) Generate(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		h.generateFormErr(c, "Belum ada tahun ajaran aktif.")
		return
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	nominal := util.ParseRupiah(c.PostForm("nominal"))
	bulanStr := c.PostFormArray("bulan")

	if nominal <= 0 {
		h.generateFormErr(c, "Nominal SPP per bulan harus lebih dari 0.")
		return
	}
	if len(bulanStr) == 0 {
		h.generateFormErr(c, "Pilih minimal satu bulan.")
		return
	}
	var bulanList []int
	for _, b := range bulanStr {
		if n, err := strconv.Atoi(b); err == nil && n >= 1 && n <= 12 {
			bulanList = append(bulanList, n)
		}
	}

	// Siswa target: satu kelas atau semua kelas di TA aktif.
	var siswaList []models.Siswa
	q := h.DB.Where("aktif = ?", true)
	if kelasID != 0 {
		q = q.Where("kelas_id = ?", kelasID)
	} else {
		// semua siswa aktif yang terdaftar pada kelas di TA aktif
		var kelasIDs []uint64
		h.DB.Model(&models.Kelas{}).Where("tahun_ajaran_id = ?", ta.ID).Pluck("id", &kelasIDs)
		if len(kelasIDs) == 0 {
			h.generateFormErr(c, "Belum ada kelas pada tahun ajaran aktif.")
			return
		}
		q = q.Where("kelas_id IN ?", kelasIDs)
	}
	q.Find(&siswaList)

	if len(siswaList) == 0 {
		h.generateFormErr(c, "Tidak ada siswa aktif pada pilihan tersebut.")
		return
	}

	created := 0
	h.DB.Transaction(func(tx *gorm.DB) error {
		for _, s := range siswaList {
			for _, b := range bulanList {
				tagihan := models.SppTagihan{
					SiswaID: s.ID, TahunAjaranID: ta.ID, Bulan: b,
					Jumlah: nominal, Status: models.SPPBelum,
				}
				// Lewati bila sudah ada (unique siswa+ta+bulan).
				res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tagihan)
				if res.RowsAffected > 0 {
					created++
				}
			}
		}
		return nil
	})

	bulanNow := int(time.Now().Month())
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+strconv.Itoa(created)+` tagihan SPP dibuat."}}`)
	h.refreshPembayaran(c, kelasID, bulanNow)
}

// =====================================================================
// EDIT NOMINAL TAGIHAN (penyesuaian per siswa)
// =====================================================================

// taClosedByID memeriksa apakah tahun ajaran tertentu sudah ditutup (dikunci).
func (h *SppHandler) taClosedByID(taID uint64) bool {
	var ta models.TahunAjaran
	if err := h.DB.First(&ta, taID).Error; err != nil {
		return false
	}
	return ta.Ditutup
}

func (h *SppHandler) EditForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.SppTagihan
	if err := h.DB.Preload("Siswa").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ta, _ := h.activeTA()
	taClosed := h.taClosedByID(t.TahunAjaranID)

	// Kumpulkan semua tagihan siswa di TA ini + total terbayar per tagihan.
	var tagihan []models.SppTagihan
	h.DB.Where("tahun_ajaran_id = ? AND siswa_id = ?", t.TahunAjaranID, t.SiswaID).Find(&tagihan)
	ids := make([]uint64, 0, len(tagihan))
	for _, x := range tagihan {
		ids = append(ids, x.ID)
	}
	paid := h.paidMap(ids)
	byBulan := map[int]models.SppTagihan{}
	for _, x := range tagihan {
		byBulan[x.Bulan] = x
	}

	// Susun 12 bulan dengan info centang default + status.
	type BulanRow struct {
		Bulan     int
		Nama      string
		Ada       bool   // sudah ada tagihan
		Checked   bool   // default centang (ada tagihan & belum dibayar & TA aktif)
		Disabled  bool   // sudah dibayar atau TA ditutup
		Status    string
		Paid      int64
	}
	rows := make([]BulanRow, 0, 12)
	for b := 1; b <= 12; b++ {
		x, ok := byBulan[b]
		if !ok {
			rows = append(rows, BulanRow{Bulan: b, Nama: util.NamaBulan(b), Ada: false, Checked: false, Disabled: taClosed})
			continue
		}
		pd := paid[x.ID]
		disabled := taClosed || pd > 0
		rows = append(rows, BulanRow{
			Bulan: b, Nama: util.NamaBulan(b), Ada: true,
			Checked: !disabled, Disabled: disabled, Status: string(x.Status), Paid: pd,
		})
	}

	c.HTML(http.StatusOK, "spp/edit_form", gin.H{
		"Title":     "Edit Nominal SPP",
		"Tagihan":   t,
		"TAClosed":  taClosed,
		"TANama":    ta.Nama,
		"Kelas":     c.Query("kelas"),
		"Rows":      rows,
	})
}

func (h *SppHandler) editErr(c *gin.Context, t models.SppTagihan, msg string) {
	ta, _ := h.activeTA()
	c.HTML(http.StatusOK, "spp/edit_form", gin.H{
		"Title": "Edit Nominal SPP", "Error": msg, "Tagihan": t,
		"TAClosed": h.taClosedByID(t.TahunAjaranID), "TANama": ta.Nama,
		"Kelas": c.PostForm("kelas"),
		"Jumlah": c.PostForm("jumlah"), "Keterangan": c.PostForm("keterangan"),
	})
}

func (h *SppHandler) Edit(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.SppTagihan
	if err := h.DB.Preload("Siswa").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)

	if h.taClosedByID(t.TahunAjaranID) {
		h.editErr(c, t, "Tahun ajaran ini sudah ditutup. Tagihan dikunci.")
		return
	}

	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	if jumlah <= 0 {
		h.editErr(c, t, "Nominal harus lebih dari 0.")
		return
	}

	bulanStr := c.PostFormArray("bulan")
	if len(bulanStr) == 0 {
		h.editErr(c, t, "Pilih minimal satu bulan.")
		return
	}
	var bulanList []int
	for _, b := range bulanStr {
		if n, err := strconv.Atoi(b); err == nil && n >= 1 && n <= 12 {
			bulanList = append(bulanList, n)
		}
	}

	keterangan := c.PostForm("keterangan")
	updated, skipped := 0, 0
	h.DB.Transaction(func(tx *gorm.DB) error {
		for _, b := range bulanList {
			var tg models.SppTagihan
			res := tx.Where("tahun_ajaran_id = ? AND siswa_id = ? AND bulan = ?", t.TahunAjaranID, t.SiswaID, b).
				First(&tg)
			if res.Error != nil {
				// Belum ada tagihan -> buat baru dengan nominal ini.
				tg = models.SppTagihan{
					SiswaID: t.SiswaID, TahunAjaranID: t.TahunAjaranID, Bulan: b,
					Jumlah: jumlah, Status: models.SPPBelum, Keterangan: keterangan,
				}
				if err := tx.Create(&tg).Error; err != nil {
					return err
				}
				updated++
				continue
			}
			// Sudah ada: lewati bila sudah dibayar (jaga integritas).
			pd := h.paidMap([]uint64{tg.ID})[tg.ID]
			if pd > 0 {
				skipped++
				continue
			}
			tg.Jumlah = jumlah
			tg.Keterangan = keterangan
			tg.Status = models.SPPBelum
			if err := tx.Save(&tg).Error; err != nil {
				return err
			}
			updated++
		}
		return nil
	})

	msg := strconv.Itoa(updated) + " bulan diperbarui."
	if skipped > 0 {
		msg += " " + strconv.Itoa(skipped) + " dilewati (sudah dibayar)."
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`"}}`)
	h.refreshPembayaran(c, kelasID, int(time.Now().Month()))
}

// =====================================================================
// BAYAR
// =====================================================================

func (h *SppHandler) BayarForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.SppTagihan
	if err := h.DB.Preload("Siswa").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{t.ID})[t.ID]
	sisa := t.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}
	c.HTML(http.StatusOK, "spp/bayar_form", gin.H{
		"Title":   "Bayar SPP",
		"Tagihan": t,
		"Paid":    paid,
		"Sisa":    sisa,
		"Today":   time.Now().Format("2006-01-02"),
		"Kelas":   c.Query("kelas"),
		"Bulan":   c.Query("bulan"),
	})
}

func (h *SppHandler) bayarFormErr(c *gin.Context, t models.SppTagihan, paid, sisa int64, msg string) {
	c.HTML(http.StatusOK, "spp/bayar_form", gin.H{
		"Title": "Bayar SPP", "Error": msg,
		"Tagihan": t, "Paid": paid, "Sisa": sisa,
		"Today": time.Now().Format("2006-01-02"),
		"Kelas": c.PostForm("kelas"), "Bulan": c.PostForm("bulan"),
		"Jumlah": c.PostForm("jumlah"),
	})
}

func (h *SppHandler) Bayar(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t models.SppTagihan
	if err := h.DB.Preload("Siswa").First(&t, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	paid := h.paidMap([]uint64{t.ID})[t.ID]
	sisa := t.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}

	jumlah := util.ParseRupiah(c.PostForm("jumlah"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	bulan, _ := strconv.Atoi(c.PostForm("bulan"))

	if sisa <= 0 {
		h.bayarFormErr(c, t, paid, sisa, "Tagihan ini sudah lunas.")
		return
	}
	if jumlah <= 0 {
		h.bayarFormErr(c, t, paid, sisa, "Jumlah bayar harus lebih dari 0.")
		return
	}
	if jumlah > sisa {
		h.bayarFormErr(c, t, paid, sisa, "Jumlah bayar melebihi sisa tagihan ("+util.Rupiah(sisa)+").")
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		kasKet := "SPP " + util.NamaBulan(t.Bulan) + " — " + t.Siswa.Nama
		return h.recordPembayaran(tx, t, jumlah, tanggal, c.PostForm("keterangan"), kasKet, ctxUserID(c), models.SumberTunai, nil)
	})
	if err != nil {
		h.bayarFormErr(c, t, paid, sisa, "Gagal menyimpan pembayaran.")
		return
	}

	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran tersimpan."}}`)
	h.refreshPembayaran(c, kelasID, bulan)
}

// =====================================================================
// TUNGGAKAN (rekap per kelas)
// =====================================================================

// TunggakanRow merangkum tunggakan satu siswa lintas seluruh bulan di TA.
type TunggakanRow struct {
	Siswa      models.Siswa
	TotalTagih int64
	TotalBayar int64
	Tunggakan  int64
	NBelum     int
}

func (h *SppHandler) tunggakanContext(c *gin.Context) gin.H {
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

	var rows []TunggakanRow
	var grandTunggakan int64
	if hasTA && len(siswaList) > 0 {
		ids := make([]uint64, len(siswaList))
		for i, s := range siswaList {
			ids[i] = s.ID
		}
		var tagihan []models.SppTagihan
		h.DB.Where("tahun_ajaran_id = ? AND siswa_id IN ?", ta.ID, ids).Find(&tagihan)

		var allTagihanIDs []uint64
		for _, t := range tagihan {
			allTagihanIDs = append(allTagihanIDs, t.ID)
		}
		paid := h.paidMap(allTagihanIDs)

		type agg struct {
			tagih  int64
			bayar  int64
			nBelum int
		}
		byS := map[uint64]*agg{}
		for _, t := range tagihan {
			a := byS[t.SiswaID]
			if a == nil {
				a = &agg{}
				byS[t.SiswaID] = a
			}
			a.tagih += t.Jumlah
			a.bayar += paid[t.ID]
			if t.Status != models.SPPLunas {
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
			grandTunggakan += tg
			rows = append(rows, TunggakanRow{
				Siswa: s, TotalTagih: a.tagih, TotalBayar: a.bayar,
				Tunggakan: tg, NBelum: a.nBelum,
			})
		}
	}

	return gin.H{
		"HasTA":          hasTA,
		"TANama":         ta.Nama,
		"KelasList":      kelasList,
		"SelectedKelas":  kelasID,
		"Rows":           rows,
		"GrandTunggakan": grandTunggakan,
		"CanEdit":        canEditKas(c),
	}
}

// =====================================================================
// RIWAYAT & HAPUS PEMBAYARAN
// =====================================================================

// riwayatData menyiapkan data modal riwayat untuk satu tagihan.
func (h *SppHandler) riwayatData(c *gin.Context, tagihanID uint64) (gin.H, bool) {
	var t models.SppTagihan
	if err := h.DB.Preload("Siswa").First(&t, tagihanID).Error; err != nil {
		return nil, false
	}
	var payments []models.SppPembayaran
	h.DB.Where("tagihan_id = ?", t.ID).Order("tanggal asc, id asc").Find(&payments)

	var paid int64
	for _, p := range payments {
		paid += p.JumlahBayar
	}
	sisa := t.Jumlah - paid
	if sisa < 0 {
		sisa = 0
	}
	// TAClosed: tahun ajaran asal sudah ditutup (dikunci) -> riwayat read-only.
	var ta models.TahunAjaran
	taClosed := h.DB.First(&ta, t.TahunAjaranID).Error == nil && ta.Ditutup
	return gin.H{
		"Title":    "Riwayat Pembayaran SPP",
		"Tagihan":  t,
		"Payments": payments,
		"Paid":     paid,
		"Sisa":     sisa,
		"Kelas":    c.Query("kelas"),
		"Bulan":    c.Query("bulan"),
		"CanEdit":  canEditKas(c),
		"TAClosed": taClosed,
	}, true
}

func (h *SppHandler) RiwayatForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, ok := h.riwayatData(c, id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.HTML(http.StatusOK, "spp/riwayat", data)
}

// HapusPembayaran menghapus satu pembayaran, baris kas tertaut, lalu
// menghitung ulang status tagihan. Modal riwayat & daftar utama di-refresh.
func (h *SppHandler) HapusPembayaran(c *gin.Context) {
	payID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var pay models.SppPembayaran
	if err := h.DB.First(&pay, payID).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	tagihanID := pay.TagihanID

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Hapus baris kas yang dibuat dari pembayaran ini.
		if err := tx.Where("spp_pembayaran_id = ?", payID).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.SppPembayaran{}, payID).Error; err != nil {
			return err
		}
		// Hitung ulang status dari sisa pembayaran.
		var t models.SppTagihan
		if err := tx.First(&t, tagihanID).Error; err != nil {
			return err
		}
		var paid int64
		tx.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihanID).
			Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
		status := models.SPPBelum
		if paid >= t.Jumlah {
			status = models.SPPLunas
		} else if paid > 0 {
			status = models.SPPCicil
		}
		return tx.Model(&models.SppTagihan{}).Where("id = ?", tagihanID).Update("status", status).Error
	})
	if err != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Gagal menghapus pembayaran."}}`)
	} else {
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Pembayaran dihapus & kas disesuaikan."}}`)
	}

	// Re-render modal riwayat + OOB daftar pembayaran.
	data, ok := h.riwayatData(c, tagihanID)
	if !ok {
		// Tagihan hilang? cukup refresh daftar.
		kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
		bulan, _ := strconv.Atoi(c.Query("bulan"))
		h.refreshPembayaran(c, kelasID, bulan)
		return
	}
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	bulan, _ := strconv.Atoi(c.Query("bulan"))
	c.Request.URL.RawQuery = "kelas=" + strconv.FormatUint(kelasID, 10) + "&bulan=" + strconv.Itoa(bulan)
	data["List"] = h.pembayaranContext(c)
	c.HTML(http.StatusOK, "spp/riwayat_after", data)
}

func (h *SppHandler) TunggakanIndex(c *gin.Context) {
	data := h.tunggakanContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "spp/tunggakan_content", data)
		return
	}
	data["Title"] = "SPP · Tunggakan"
	data["ActiveMenu"] = "spp"
	data["ActiveTab"] = "tunggakan"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "spp/tunggakan", data)
}
