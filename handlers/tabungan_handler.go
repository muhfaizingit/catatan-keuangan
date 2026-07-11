package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/models"
	"teras-keuangan/util"
)

// TabunganHandler menangani tabungan siswa.
type TabunganHandler struct {
	DB *gorm.DB
}

func NewTabunganHandler(db *gorm.DB) *TabunganHandler {
	return &TabunganHandler{DB: db}
}

// hitungPotong menghitung potongan (dibulatkan ke rupiah terdekat).
func hitungPotong(setor int64, persen float64) int64 {
	return int64(math.Round(float64(setor) * persen / 100.0))
}

func (h *TabunganHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

// tutupRecord mengambil catatan tutup tabungan untuk satu TA (bila ada).
func (h *TabunganHandler) tutupRecord(taID uint64) (models.TutupTabungan, bool) {
	var t models.TutupTabungan
	ok := h.DB.Where("tahun_ajaran_id = ?", taID).First(&t).Error == nil
	return t, ok
}

func (h *TabunganHandler) isClosed(taID uint64) bool {
	_, ok := h.tutupRecord(taID)
	return ok
}

// tutupAlokasi = satu pelunasan tunggakan (SPP/Tagihan) dari saldo tabungan.
type tutupAlokasi struct {
	IsSPP          bool
	TagihanID      uint64
	Amount         int64
	NominalTagihan int64
	KasKeterangan  string
}

// tutupPlan = rencana lengkap tutup tabungan se-sekolah.
type tutupPlan struct {
	TotalSetor          int64
	TotalPotong         int64
	TotalBayarTunggakan int64
	TotalDiserahkan     int64
	JumlahSiswa         int
	Items               []tutupAlokasi
}

func (h *TabunganHandler) sumPaidSPP(tagihanID uint64) int64 {
	var v int64
	h.DB.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihanID).
		Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&v)
	return v
}

func (h *TabunganHandler) sumPaidTagihan(tagihanID uint64) int64 {
	var v int64
	h.DB.Model(&models.TagihanPembayaran{}).Where("tagihan_id = ?", tagihanID).
		Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&v)
	return v
}

// planTutup: potongan → kas; saldo bersih melunasi tunggakan SPP (per bulan)
// lalu Tagihan (per tanggal); sisa diserahkan ke siswa.
func (h *TabunganHandler) planTutup(taID uint64) tutupPlan {
	var plan tutupPlan
	type sp struct {
		SiswaID uint64
		Setor   int64
		Potong  int64
	}
	var sps []sp
	h.DB.Model(&models.TabunganSetoran{}).
		Select("siswa_id, COALESCE(SUM(jumlah_setor),0) as setor, COALESCE(SUM(jumlah_potong),0) as potong").
		Where("tahun_ajaran_id = ?", taID).Group("siswa_id").Scan(&sps)
	plan.JumlahSiswa = len(sps)

	for _, s := range sps {
		plan.TotalSetor += s.Setor
		plan.TotalPotong += s.Potong
		remaining := s.Setor - s.Potong

		var siswa models.Siswa
		h.DB.First(&siswa, s.SiswaID)

		// 1) SPP dulu, per bulan urut
		if remaining > 0 {
			var sppList []models.SppTagihan
			h.DB.Where("siswa_id = ? AND tahun_ajaran_id = ? AND status <> ?", s.SiswaID, taID, models.SPPLunas).
				Order("bulan asc").Find(&sppList)
			for _, st := range sppList {
				if remaining <= 0 {
					break
				}
				sisa := st.Jumlah - h.sumPaidSPP(st.ID)
				if sisa <= 0 {
					continue
				}
				amt := minI64(remaining, sisa)
				plan.Items = append(plan.Items, tutupAlokasi{
					IsSPP: true, TagihanID: st.ID, Amount: amt, NominalTagihan: st.Jumlah,
					KasKeterangan: "SPP " + util.NamaBulan(st.Bulan) + " — " + siswa.Nama + " (dari tabungan)",
				})
				remaining -= amt
				plan.TotalBayarTunggakan += amt
			}
		}
		// 2) Tagihan non-SPP, per tanggal urut
		if remaining > 0 {
			var tagList []models.Tagihan
			h.DB.Preload("JenisTagihan").
				Where("siswa_id = ? AND tahun_ajaran_id = ? AND status <> ?", s.SiswaID, taID, models.TagihanLunas).
				Order("tanggal asc, id asc").Find(&tagList)
			for _, t := range tagList {
				if remaining <= 0 {
					break
				}
				sisa := t.Nominal - h.sumPaidTagihan(t.ID)
				if sisa <= 0 {
					continue
				}
				amt := minI64(remaining, sisa)
				plan.Items = append(plan.Items, tutupAlokasi{
					IsSPP: false, TagihanID: t.ID, Amount: amt, NominalTagihan: t.Nominal,
					KasKeterangan: t.JenisTagihan.Nama + " — " + siswa.Nama + " (dari tabungan)",
				})
				remaining -= amt
				plan.TotalBayarTunggakan += amt
			}
		}
		plan.TotalDiserahkan += remaining
	}
	return plan
}

func (h *TabunganHandler) recomputeSPP(tx *gorm.DB, tagihanID uint64, jumlah int64) error {
	var paid int64
	tx.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihanID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.SPPBelum
	if paid >= jumlah {
		status = models.SPPLunas
	} else if paid > 0 {
		status = models.SPPCicil
	}
	return tx.Model(&models.SppTagihan{}).Where("id = ?", tagihanID).Update("status", status).Error
}

func (h *TabunganHandler) recomputeTagihan(tx *gorm.DB, tagihanID uint64, nominal int64) error {
	var paid int64
	tx.Model(&models.TagihanPembayaran{}).Where("tagihan_id = ?", tagihanID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	status := models.TagihanBelum
	if paid >= nominal {
		status = models.TagihanLunas
	} else if paid > 0 {
		status = models.TagihanCicil
	}
	return tx.Model(&models.Tagihan{}).Where("id = ?", tagihanID).Update("status", status).Error
}

func (h *TabunganHandler) kelasByTA(taID uint64) []models.Kelas {
	var list []models.Kelas
	h.DB.Where("tahun_ajaran_id = ?", taID).Order("nama asc").Find(&list)
	return list
}

// recordSetoran membuat satu setoran tabungan.
//
// Catatan penting: potongan TIDAK diposting ke kas saat setor. Persen & jumlah
// potongan hanya di-snapshot (JumlahPotong = calon potongan) dan baru diakui ke
// kas saat tabungan ditutup (Tutup Tahun, Phase 6). Selama belum ditutup, saldo
// siswa = total setoran penuh dan kas belum menerima potongan apa pun.
func (h *TabunganHandler) recordSetoran(tx *gorm.DB, siswa models.Siswa, taID uint64, setor int64, persen float64, tanggal time.Time, ket string, userID uint64) error {
	potong := hitungPotong(setor, persen)
	st := models.TabunganSetoran{
		SiswaID: siswa.ID, TahunAjaranID: taID, Tanggal: tanggal,
		JumlahSetor: setor, PersenPotong: persen, JumlahPotong: potong,
		JumlahBersih: setor - potong, Keterangan: ket, UserID: userID,
	}
	return tx.Create(&st).Error
}

// TabunganRow merangkum posisi tabungan satu siswa.
// Saldo (= TotalSetor) adalah setoran penuh karena potongan baru diambil saat
// tutup. PerkiraanPotong & PerkiraanBersih hanya info calon saat tutup.
type TabunganRow struct {
	Siswa       models.Siswa
	TotalSetor  int64     // = saldo saat ini (setoran penuh)
	LastAmount  int64     // jumlah setoran terakhir
	LastTanggal time.Time // tanggal setoran terakhir (zero bila belum ada)
}

func (h *TabunganHandler) context(c *gin.Context) gin.H {
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

	// Muat setoran siswa di kelas (untuk agregat + setoran terakhir).
	type sums struct {
		setor, potong, bersih int64
		last                  models.TabunganSetoran
		hasLast               bool
	}
	acc := map[uint64]*sums{}
	if hasTA && len(siswaList) > 0 {
		ids := make([]uint64, len(siswaList))
		for i, s := range siswaList {
			ids[i] = s.ID
		}
		var setoran []models.TabunganSetoran
		h.DB.Where("tahun_ajaran_id = ? AND siswa_id IN ?", ta.ID, ids).
			Order("tanggal desc, id desc").Find(&setoran)
		for _, st := range setoran {
			a := acc[st.SiswaID]
			if a == nil {
				a = &sums{}
				acc[st.SiswaID] = a
			}
			a.setor += st.JumlahSetor
			a.potong += st.JumlahPotong
			a.bersih += st.JumlahBersih
			if !a.hasLast { // baris pertama (urut desc) = setoran terakhir
				a.last, a.hasLast = st, true
			}
		}
	}

	var rows []TabunganRow
	var totSetor, totPotong, totBersih int64
	for _, s := range siswaList {
		a := acc[s.ID]
		if a == nil {
			a = &sums{}
		}
		row := TabunganRow{Siswa: s, TotalSetor: a.setor}
		if a.hasLast {
			row.LastAmount = a.last.JumlahSetor
			row.LastTanggal = a.last.Tanggal
		}
		rows = append(rows, row)
		totSetor += a.setor
		totPotong += a.potong
		totBersih += a.bersih
	}

	tutup, closed := models.TutupTabungan{}, false
	if hasTA {
		tutup, closed = h.tutupRecord(ta.ID)
	}

	return gin.H{
		"HasTA":         hasTA,
		"TANama":        ta.Nama,
		"Persen":        models.PersenPotonganTabungan(h.DB),
		"KelasList":     kelasList,
		"SelectedKelas": kelasID,
		"Rows":          rows,
		"TotalSaldo":    totSetor, // saldo = setoran penuh (potongan belum diambil)
		"TotalPotong":   totPotong,
		"TotalBersih":   totBersih,
		"Closed":        closed,
		"Tutup":         tutup,
		"CanEdit":       canEditKas(c),
	}
}

func (h *TabunganHandler) Index(c *gin.Context) {
	data := h.context(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "tabungan/content", data)
		return
	}
	data["Title"] = "Tabungan"
	data["ActiveMenu"] = "tabungan"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "tabungan/index", data)
}

func (h *TabunganHandler) refresh(c *gin.Context, kelasID uint64) {
	c.Request.URL.RawQuery = "kelas=" + strconv.FormatUint(kelasID, 10)
	c.HTML(http.StatusOK, "tabungan/content_oob", h.context(c))
}

// ---------- setor per siswa ----------

func (h *TabunganHandler) SetorForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var siswa models.Siswa
	if err := h.DB.First(&siswa, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.HTML(http.StatusOK, "tabungan/setor_form", gin.H{
		"Title": "Setor Tabungan", "Siswa": siswa,
		"Persen": models.PersenPotonganTabungan(h.DB),
		"Kelas":  c.Query("kelas"), "Today": time.Now().Format("2006-01-02"),
	})
}

func (h *TabunganHandler) setorFormErr(c *gin.Context, siswa models.Siswa, msg string) {
	c.HTML(http.StatusOK, "tabungan/setor_form", gin.H{
		"Title": "Setor Tabungan", "Siswa": siswa, "Error": msg,
		"Persen": models.PersenPotonganTabungan(h.DB),
		"Kelas":  c.PostForm("kelas"), "Today": time.Now().Format("2006-01-02"),
		"Jumlah": c.PostForm("jumlah"), "Keterangan": c.PostForm("keterangan"),
	})
}

func (h *TabunganHandler) Setor(c *gin.Context) {
	ta, hasTA := h.activeTA()
	siswaID, _ := strconv.ParseUint(c.PostForm("siswa_id"), 10, 64)
	var siswa models.Siswa
	if h.DB.First(&siswa, siswaID).Error != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if !hasTA {
		h.setorFormErr(c, siswa, "Belum ada tahun ajaran aktif.")
		return
	}
	if h.isClosed(ta.ID) {
		h.setorFormErr(c, siswa, "Tabungan tahun ajaran ini sudah ditutup.")
		return
	}
	setor := util.ParseRupiah(c.PostForm("jumlah"))
	if setor <= 0 {
		h.setorFormErr(c, siswa, "Jumlah setoran harus lebih dari 0.")
		return
	}
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	persen := models.PersenPotonganTabungan(h.DB)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		return h.recordSetoran(tx, siswa, ta.ID, setor, persen, tanggal, c.PostForm("keterangan"), ctxUserID(c))
	})
	if err != nil {
		h.setorFormErr(c, siswa, "Gagal menyimpan setoran.")
		return
	}
	kelasID := uint64(0)
	if siswa.KelasID != nil {
		kelasID = *siswa.KelasID
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Setoran tersimpan."}}`)
	h.refresh(c, kelasID)
}

// ---------- setor bulk per kelas ----------

func (h *TabunganHandler) BulkForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if kelasID == 0 && len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}
	c.HTML(http.StatusOK, "tabungan/bulk_form", gin.H{
		"Title": "Setor Bulk per Kelas", "HasTA": hasTA,
		"KelasList": kelasList, "SelectedKelas": kelasID,
		"SiswaList": h.siswaByKelas(kelasID),
		"Persen":    models.PersenPotonganTabungan(h.DB),
		"Today":     time.Now().Format("2006-01-02"),
	})
}

func (h *TabunganHandler) siswaByKelas(kelasID uint64) []models.Siswa {
	var list []models.Siswa
	if kelasID != 0 {
		h.DB.Where("kelas_id = ? AND aktif = ?", kelasID, true).Order("nama asc").Find(&list)
	}
	return list
}

// BulkSiswaOptions memuat ulang daftar siswa saat kelas berubah.
func (h *TabunganHandler) BulkSiswaOptions(c *gin.Context) {
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	c.HTML(http.StatusOK, "tabungan/bulk_siswa_options", gin.H{"SiswaList": h.siswaByKelas(kelasID)})
}

func (h *TabunganHandler) bulkFormErr(c *gin.Context, msg string) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	c.HTML(http.StatusOK, "tabungan/bulk_form", gin.H{
		"Title": "Setor Bulk per Kelas", "Error": msg, "HasTA": hasTA,
		"KelasList": kelasList, "SelectedKelas": kelasID,
		"SiswaList": h.siswaByKelas(kelasID),
		"Persen":    models.PersenPotonganTabungan(h.DB),
		"Today":     time.Now().Format("2006-01-02"),
		"Jumlah":    c.PostForm("jumlah"),
	})
}

func (h *TabunganHandler) Bulk(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		h.bulkFormErr(c, "Belum ada tahun ajaran aktif.")
		return
	}
	if h.isClosed(ta.ID) {
		h.bulkFormErr(c, "Tabungan tahun ajaran ini sudah ditutup.")
		return
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	setor := util.ParseRupiah(c.PostForm("jumlah"))
	if setor <= 0 {
		h.bulkFormErr(c, "Jumlah setoran harus lebih dari 0.")
		return
	}
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	persen := models.PersenPotonganTabungan(h.DB)

	var siswaIDs []uint64
	for _, s := range c.PostFormArray("siswa") {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			siswaIDs = append(siswaIDs, id)
		}
	}
	if len(siswaIDs) == 0 {
		h.bulkFormErr(c, "Pilih minimal satu siswa.")
		return
	}

	userID := ctxUserID(c)
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for _, sid := range siswaIDs {
			var siswa models.Siswa
			if err := tx.First(&siswa, sid).Error; err != nil {
				return err
			}
			if err := h.recordSetoran(tx, siswa, ta.ID, setor, persen, tanggal, c.PostForm("keterangan"), userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		h.bulkFormErr(c, "Gagal menyimpan setoran bulk.")
		return
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Setoran bulk tersimpan untuk `+strconv.Itoa(len(siswaIDs))+` siswa."}}`)
	h.refresh(c, kelasID)
}

// ---------- riwayat & hapus ----------

func (h *TabunganHandler) riwayatData(c *gin.Context, siswaID uint64) (gin.H, bool) {
	var siswa models.Siswa
	if h.DB.First(&siswa, siswaID).Error != nil {
		return nil, false
	}
	ta, _ := h.activeTA()

	bulan, _ := strconv.Atoi(c.Query("bulan"))

	// Daftar setoran (bisa difilter per bulan).
	q := h.DB.Where("siswa_id = ? AND tahun_ajaran_id = ?", siswaID, ta.ID)
	if bulan >= 1 && bulan <= 12 {
		q = q.Where("MONTH(tanggal) = ?", bulan)
	}
	var list []models.TabunganSetoran
	q.Order("tanggal desc, id desc").Find(&list)

	// Saldo = total setoran penuh (seluruh bulan di TA), tidak ikut terfilter.
	var saldo int64
	h.DB.Model(&models.TabunganSetoran{}).
		Where("siswa_id = ? AND tahun_ajaran_id = ?", siswaID, ta.ID).
		Select("COALESCE(SUM(jumlah_setor),0)").Scan(&saldo)

	// Subtotal setoran pada filter aktif.
	var subtotal int64
	for _, s := range list {
		subtotal += s.JumlahSetor
	}

	return gin.H{
		"Title": "Riwayat Tabungan", "Siswa": siswa, "List": list, "Saldo": saldo,
		"Bulan": bulan, "Subtotal": subtotal,
		"Kelas": c.Query("kelas"), "CanEdit": canEditKas(c),
	}, true
}

func (h *TabunganHandler) RiwayatForm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, ok := h.riwayatData(c, id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "tabungan/riwayat_body", data)
		return
	}
	c.HTML(http.StatusOK, "tabungan/riwayat", data)
}

func (h *TabunganHandler) HapusSetoran(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var st models.TabunganSetoran
	if h.DB.First(&st, id).Error != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if h.isClosed(st.TahunAjaranID) {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tabungan sudah ditutup; setoran tidak bisa dihapus."}}`)
		data, ok := h.riwayatData(c, st.SiswaID)
		if ok {
			c.HTML(http.StatusOK, "tabungan/riwayat_body", data)
			return
		}
		c.Status(http.StatusOK)
		return
	}
	siswaID := st.SiswaID
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tabungan_setoran_id = ?", id).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.TabunganSetoran{}, id).Error
	})
	if err != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Gagal menghapus setoran."}}`)
	} else {
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Setoran dihapus & kas disesuaikan."}}`)
	}

	data, ok := h.riwayatData(c, siswaID)
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	if !ok {
		h.refresh(c, kelasID)
		return
	}
	c.Request.URL.RawQuery = "kelas=" + strconv.FormatUint(kelasID, 10)
	data["List2"] = h.context(c)
	c.HTML(http.StatusOK, "tabungan/riwayat_after", data)
}

// ---------- ubah persen potongan ----------

func (h *TabunganHandler) PersenForm(c *gin.Context) {
	c.HTML(http.StatusOK, "tabungan/persen_form", gin.H{
		"Title": "Ubah Persen Potongan", "Persen": models.PersenPotonganTabungan(h.DB),
		"Kelas": c.Query("kelas"),
	})
}

func (h *TabunganHandler) PersenSave(c *gin.Context) {
	v := c.PostForm("persen")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || f > 100 {
		c.HTML(http.StatusOK, "tabungan/persen_form", gin.H{
			"Title": "Ubah Persen Potongan", "Persen": models.PersenPotonganTabungan(h.DB),
			"Kelas": c.PostForm("kelas"), "Error": "Persen harus antara 0 dan 100.",
		})
		return
	}
	// Simpan sebagai string ringkas (buang .0 bila bulat).
	nilai := strconv.FormatFloat(f, 'f', -1, 64)
	models.SetSetting(h.DB, models.KeyPersenPotonganTabungan, nilai, ctxUserID(c))
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Persen potongan diperbarui (berlaku untuk setoran baru)."}}`)
	h.refresh(c, kelasID)
}

// ---------- tutup tabungan (seluruh sekolah, per tahun ajaran) ----------

func (h *TabunganHandler) TutupForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		c.HTML(http.StatusOK, "tabungan/tutup_form", gin.H{"Title": "Tutup Tabungan", "HasTA": false})
		return
	}
	p := h.planTutup(ta.ID)
	c.HTML(http.StatusOK, "tabungan/tutup_form", gin.H{
		"Title": "Tutup Tabungan", "HasTA": true, "TANama": ta.Nama,
		"TotalSetor": p.TotalSetor, "TotalPotong": p.TotalPotong, "TotalBersih": p.TotalSetor - p.TotalPotong,
		"TotalBayarTunggakan": p.TotalBayarTunggakan, "TotalDiserahkan": p.TotalDiserahkan,
		"JumlahSiswa": p.JumlahSiswa, "Persen": models.PersenPotonganTabungan(h.DB),
		"Kelas": c.Query("kelas"), "Today": time.Now().Format("2006-01-02"),
	})
}

func (h *TabunganHandler) Tutup(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Belum ada tahun ajaran aktif."}}`)
		c.Status(http.StatusOK)
		return
	}
	if h.isClosed(ta.ID) {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tabungan sudah ditutup."}}`)
		kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
		h.refresh(c, kelasID)
		return
	}

	p := h.planTutup(ta.ID)
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	userID := ctxUserID(c)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		tutup := models.TutupTabungan{
			TahunAjaranID: ta.ID, Tanggal: tanggal,
			TotalSetor: p.TotalSetor, TotalPotong: p.TotalPotong, TotalBersih: p.TotalSetor - p.TotalPotong,
			TotalBayarTunggakan: p.TotalBayarTunggakan, TotalDiserahkan: p.TotalDiserahkan,
			JumlahSiswa: p.JumlahSiswa, Keterangan: c.PostForm("keterangan"), UserID: userID,
		}
		if err := tx.Create(&tutup).Error; err != nil {
			return err
		}

		// Potongan agregat -> kas.
		if p.TotalPotong > 0 {
			var jenis models.JenisPemasukan
			if err := tx.Where("kode = ?", models.KodeJenisPotonganTabungan).First(&jenis).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.KasPemasukan{
				TahunAjaranID: ta.ID, JenisPemasukanID: jenis.ID, Tanggal: tanggal,
				Jumlah: p.TotalPotong, Keterangan: "Potongan tabungan (tutup) — " + ta.Nama,
				UserID: userID, TutupTabunganID: &tutup.ID,
			}).Error; err != nil {
				return err
			}
		}

		// Pelunasan tunggakan dari saldo bersih -> kas (SPP & Tagihan).
		var jenisSPP, jenisTag models.JenisPemasukan
		if err := tx.Where("kode = ?", models.KodeJenisSPP).First(&jenisSPP).Error; err != nil {
			return err
		}
		if err := tx.Where("kode = ?", models.KodeJenisTagihan).First(&jenisTag).Error; err != nil {
			return err
		}
		for _, it := range p.Items {
			if it.IsSPP {
				bayar := models.SppPembayaran{
					TagihanID: it.TagihanID, Tanggal: tanggal, JumlahBayar: it.Amount,
					Keterangan: "Pelunasan dari tabungan (tutup)", Sumber: models.SumberTabungan,
					UserID: userID, TutupTabunganID: &tutup.ID,
				}
				if err := tx.Create(&bayar).Error; err != nil {
					return err
				}
				if err := tx.Create(&models.KasPemasukan{
					TahunAjaranID: ta.ID, JenisPemasukanID: jenisSPP.ID, Tanggal: tanggal,
					Jumlah: it.Amount, Keterangan: it.KasKeterangan, UserID: userID, SppPembayaranID: &bayar.ID,
				}).Error; err != nil {
					return err
				}
				if err := h.recomputeSPP(tx, it.TagihanID, it.NominalTagihan); err != nil {
					return err
				}
			} else {
				bayar := models.TagihanPembayaran{
					TagihanID: it.TagihanID, Tanggal: tanggal, JumlahBayar: it.Amount,
					Keterangan: "Pelunasan dari tabungan (tutup)", UserID: userID, TutupTabunganID: &tutup.ID,
				}
				if err := tx.Create(&bayar).Error; err != nil {
					return err
				}
				if err := tx.Create(&models.KasPemasukan{
					TahunAjaranID: ta.ID, JenisPemasukanID: jenisTag.ID, Tanggal: tanggal,
					Jumlah: it.Amount, Keterangan: it.KasKeterangan, UserID: userID, TagihanPembayaranID: &bayar.ID,
				}).Error; err != nil {
					return err
				}
				if err := h.recomputeTagihan(tx, it.TagihanID, it.NominalTagihan); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Gagal menutup tabungan."}}`)
	} else {
		msg := "Tabungan ditutup. Ke kas: potongan " + util.Rupiah(p.TotalPotong) + " + pelunasan tunggakan " + util.Rupiah(p.TotalBayarTunggakan) + "."
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`"}}`)
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	h.refresh(c, kelasID)
}

func (h *TabunganHandler) BatalTutup(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		c.Status(http.StatusOK)
		return
	}
	tutup, ok := h.tutupRecord(ta.ID)
	if ok {
		h.DB.Transaction(func(tx *gorm.DB) error {
			// Baris kas potongan agregat (punya tutup_tabungan_id).
			if err := tx.Where("tutup_tabungan_id = ?", tutup.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
				return err
			}
			// Pelunasan SPP dari tutup: hapus baris kas + pembayaran, recompute status.
			var sppPays []models.SppPembayaran
			tx.Where("tutup_tabungan_id = ?", tutup.ID).Find(&sppPays)
			sppTag := map[uint64]bool{}
			for _, p := range sppPays {
				if err := tx.Where("spp_pembayaran_id = ?", p.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
					return err
				}
				sppTag[p.TagihanID] = true
			}
			if err := tx.Where("tutup_tabungan_id = ?", tutup.ID).Delete(&models.SppPembayaran{}).Error; err != nil {
				return err
			}
			for tid := range sppTag {
				var st models.SppTagihan
				if err := tx.First(&st, tid).Error; err == nil {
					if err := h.recomputeSPP(tx, tid, st.Jumlah); err != nil {
						return err
					}
				}
			}
			// Pelunasan Tagihan dari tutup.
			var tagPays []models.TagihanPembayaran
			tx.Where("tutup_tabungan_id = ?", tutup.ID).Find(&tagPays)
			tagSet := map[uint64]bool{}
			for _, p := range tagPays {
				if err := tx.Where("tagihan_pembayaran_id = ?", p.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
					return err
				}
				tagSet[p.TagihanID] = true
			}
			if err := tx.Where("tutup_tabungan_id = ?", tutup.ID).Delete(&models.TagihanPembayaran{}).Error; err != nil {
				return err
			}
			for tid := range tagSet {
				var t models.Tagihan
				if err := tx.First(&t, tid).Error; err == nil {
					if err := h.recomputeTagihan(tx, tid, t.Nominal); err != nil {
						return err
					}
				}
			}
			return tx.Delete(&models.TutupTabungan{}, tutup.ID).Error
		})
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Tutup tabungan dibatalkan; potongan & pelunasan ditarik kembali."}}`)
	}
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	h.refresh(c, kelasID)
}
