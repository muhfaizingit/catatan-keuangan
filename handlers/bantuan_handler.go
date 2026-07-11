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

func minI64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// RekapDonatur merangkum total bantuan per donatur dalam satu tahun ajaran.
type RekapDonatur struct {
	Donatur  string
	Diterima int64
	KeSPP    int64
	Donasi   int64
}

func (h *SppHandler) donaturAktif() []models.Donatur {
	var list []models.Donatur
	h.DB.Where("aktif = ?", true).Order("nama asc").Find(&list)
	return list
}

func (h *SppHandler) siswaByKelas(kelasID uint64) []models.Siswa {
	var list []models.Siswa
	if kelasID != 0 {
		h.DB.Where("kelas_id = ? AND aktif = ?", kelasID, true).Order("nama asc").Find(&list)
	}
	return list
}

// ---------- list & rekap ----------

func (h *SppHandler) bantuanContext(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()

	var danaList []models.DanaBantuan
	rekap := []RekapDonatur{}
	if hasTA {
		h.DB.Preload("Donatur").Where("tahun_ajaran_id = ?", ta.ID).
			Order("tanggal desc, id desc").Find(&danaList)

		// Rekap agregat per donatur.
		byD := map[uint64]*RekapDonatur{}
		order := []uint64{}
		for _, d := range danaList {
			r := byD[d.DonaturID]
			if r == nil {
				r = &RekapDonatur{Donatur: d.Donatur.Nama}
				byD[d.DonaturID] = r
				order = append(order, d.DonaturID)
			}
			r.Diterima += d.JumlahDiterima
			r.KeSPP += d.TotalKeSPP
			r.Donasi += d.TotalDonasi
		}
		for _, id := range order {
			rekap = append(rekap, *byD[id])
		}
	}

	return gin.H{
		"HasTA":    hasTA,
		"TANama":   ta.Nama,
		"DanaList": danaList,
		"Rekap":    rekap,
		"CanEdit":  canEditKas(c),
	}
}

func (h *SppHandler) BantuanIndex(c *gin.Context) {
	data := h.bantuanContext(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "spp/bantuan_content", data)
		return
	}
	data["Title"] = "SPP · Dana Bantuan"
	data["ActiveMenu"] = "spp"
	data["ActiveTab"] = "bantuan"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "spp/bantuan", data)
}

func (h *SppHandler) refreshBantuan(c *gin.Context) {
	c.HTML(http.StatusOK, "spp/bantuan_content_oob", h.bantuanContext(c))
}

// ---------- form ----------

func (h *SppHandler) BantuanForm(c *gin.Context) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	var kelasID uint64
	if len(kelasList) > 0 {
		kelasID = kelasList[0].ID
	}
	c.HTML(http.StatusOK, "spp/bantuan_form", gin.H{
		"Title":        "Catat Dana Bantuan",
		"HasTA":        hasTA,
		"DonaturList":  h.donaturAktif(),
		"KelasList":    kelasList,
		"SelectedKelas": kelasID,
		"SiswaList":    h.siswaByKelas(kelasID),
		"Today":        time.Now().Format("2006-01-02"),
	})
}

// BantuanSiswaOptions mengembalikan daftar checkbox siswa untuk kelas terpilih.
func (h *SppHandler) BantuanSiswaOptions(c *gin.Context) {
	kelasID, _ := strconv.ParseUint(c.Query("kelas"), 10, 64)
	c.HTML(http.StatusOK, "spp/bantuan_siswa_options", gin.H{
		"SiswaList": h.siswaByKelas(kelasID),
	})
}

func (h *SppHandler) bantuanFormErr(c *gin.Context, msg string) {
	ta, hasTA := h.activeTA()
	kelasList := []models.Kelas{}
	if hasTA {
		kelasList = h.kelasByTA(ta.ID)
	}
	kelasID, _ := strconv.ParseUint(c.PostForm("kelas"), 10, 64)
	c.HTML(http.StatusOK, "spp/bantuan_form", gin.H{
		"Title": "Catat Dana Bantuan", "Error": msg,
		"HasTA": hasTA, "DonaturList": h.donaturAktif(), "KelasList": kelasList,
		"SelectedKelas": kelasID, "SiswaList": h.siswaByKelas(kelasID),
		"Today":          time.Now().Format("2006-01-02"),
		"Donatur":        c.PostForm("donatur_id"),
		"NominalSPP":     c.PostForm("nominal_spp"),
		"NominalDonatur": c.PostForm("nominal_donatur"),
	})
}

// ---------- create (alokasi) ----------

func (h *SppHandler) BantuanCreate(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		h.bantuanFormErr(c, "Belum ada tahun ajaran aktif.")
		return
	}
	donaturID, _ := strconv.ParseUint(c.PostForm("donatur_id"), 10, 64)
	var donatur models.Donatur
	if donaturID == 0 || h.DB.First(&donatur, donaturID).Error != nil {
		h.bantuanFormErr(c, "Donatur wajib dipilih.")
		return
	}
	nominalSPP := util.ParseRupiah(c.PostForm("nominal_spp"))
	nominalDonatur := util.ParseRupiah(c.PostForm("nominal_donatur"))
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())

	if nominalSPP <= 0 {
		h.bantuanFormErr(c, "Nominal SPP/bulan harus lebih dari 0.")
		return
	}
	if nominalDonatur <= 0 {
		h.bantuanFormErr(c, "Nominal dari donatur/bulan harus lebih dari 0.")
		return
	}

	var siswaIDs []uint64
	for _, s := range c.PostFormArray("siswa") {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			siswaIDs = append(siswaIDs, id)
		}
	}
	var bulanList []int
	for _, b := range c.PostFormArray("bulan") {
		if n, err := strconv.Atoi(b); err == nil && n >= 1 && n <= 12 {
			bulanList = append(bulanList, n)
		}
	}
	if len(siswaIDs) == 0 {
		h.bantuanFormErr(c, "Pilih minimal satu siswa.")
		return
	}
	if len(bulanList) == 0 {
		h.bantuanFormErr(c, "Pilih minimal satu bulan.")
		return
	}

	userID := ctxUserID(c)
	var totalKeSPP, totalDonasi int64
	jumlahSlot := len(siswaIDs) * len(bulanList)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Buat record dana bantuan dulu agar punya ID untuk ditautkan.
		dana := models.DanaBantuan{
			DonaturID: donaturID, TahunAjaranID: ta.ID, Tanggal: tanggal,
			NominalSPP: nominalSPP, NominalDonatur: nominalDonatur,
			JumlahSlot: jumlahSlot, Keterangan: c.PostForm("keterangan"), UserID: userID,
		}
		if err := tx.Create(&dana).Error; err != nil {
			return err
		}

		for _, sid := range siswaIDs {
			var siswa models.Siswa
			if err := tx.First(&siswa, sid).Error; err != nil {
				return err
			}
			for _, bulan := range bulanList {
				// Cari tagihan; bila belum ada, buat dengan nominal SPP.
				var tagihan models.SppTagihan
				err := tx.Where("siswa_id = ? AND tahun_ajaran_id = ? AND bulan = ?", sid, ta.ID, bulan).First(&tagihan).Error
				if err == gorm.ErrRecordNotFound {
					tagihan = models.SppTagihan{
						SiswaID: sid, TahunAjaranID: ta.ID, Bulan: bulan,
						Jumlah: nominalSPP, Status: models.SPPBelum,
					}
					if err := tx.Create(&tagihan).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				}

				var paid int64
				tx.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihan.ID).
					Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
				sisa := tagihan.Jumlah - paid
				if sisa < 0 {
					sisa = 0
				}
				bayar := minI64(nominalDonatur, sisa)
				excess := nominalDonatur - bayar

				if bayar > 0 {
					payKet := "Bantuan: " + donatur.Nama
					kasKet := "SPP " + util.NamaBulan(bulan) + " — " + siswa.Nama + " (Bantuan: " + donatur.Nama + ")"
					if err := h.recordPembayaran(tx, tagihan, bayar, tanggal, payKet, kasKet, userID, models.SumberBantuan, &dana.ID); err != nil {
						return err
					}
				}
				totalKeSPP += bayar
				totalDonasi += excess
			}
		}

		// Sisa dana -> kas "Donasi" (pemasukan sekolah).
		if totalDonasi > 0 {
			var jenis models.JenisPemasukan
			if err := tx.Where("kode = ?", models.KodeJenisDonasi).First(&jenis).Error; err != nil {
				return err
			}
			kas := models.KasPemasukan{
				TahunAjaranID: ta.ID, JenisPemasukanID: jenis.ID, Tanggal: tanggal,
				Jumlah: totalDonasi, Keterangan: "Sisa dana bantuan — " + donatur.Nama,
				UserID: userID, DanaBantuanID: &dana.ID,
			}
			if err := tx.Create(&kas).Error; err != nil {
				return err
			}
		}

		dana.JumlahDiterima = nominalDonatur * int64(jumlahSlot)
		dana.TotalKeSPP = totalKeSPP
		dana.TotalDonasi = totalDonasi
		return tx.Save(&dana).Error
	})
	if err != nil {
		h.bantuanFormErr(c, "Gagal menyimpan dana bantuan.")
		return
	}

	msg := "Dana bantuan tersimpan: SPP " + util.Rupiah(totalKeSPP)
	if totalDonasi > 0 {
		msg += ", donasi " + util.Rupiah(totalDonasi)
	}
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`."}}`)
	h.refreshBantuan(c)
}

// ---------- detail ----------

func (h *SppHandler) BantuanDetail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var dana models.DanaBantuan
	if err := h.DB.Preload("Donatur").First(&dana, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	var pays []models.SppPembayaran
	h.DB.Where("dana_bantuan_id = ?", id).Find(&pays)

	// Ambil info tagihan (siswa, bulan) untuk tiap pembayaran.
	type Baris struct {
		Siswa  string
		Bulan  int
		Jumlah int64
	}
	var baris []Baris
	for _, p := range pays {
		var t models.SppTagihan
		if err := h.DB.Preload("Siswa").First(&t, p.TagihanID).Error; err == nil {
			baris = append(baris, Baris{Siswa: t.Siswa.Nama, Bulan: t.Bulan, Jumlah: p.JumlahBayar})
		}
	}

	c.HTML(http.StatusOK, "spp/bantuan_detail", gin.H{
		"Title": "Detail Dana Bantuan",
		"Dana":  dana,
		"Baris": baris,
	})
}

// ---------- hapus (reverse) ----------

func (h *SppHandler) BantuanDelete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var pays []models.SppPembayaran
		tx.Where("dana_bantuan_id = ?", id).Find(&pays)

		tagihanIDs := map[uint64]bool{}
		for _, p := range pays {
			// Hapus baris kas SPP tertaut pembayaran ini.
			if err := tx.Where("spp_pembayaran_id = ?", p.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
				return err
			}
			tagihanIDs[p.TagihanID] = true
		}
		// Hapus pembayaran & baris kas donasi dana ini.
		if err := tx.Where("dana_bantuan_id = ?", id).Delete(&models.SppPembayaran{}).Error; err != nil {
			return err
		}
		if err := tx.Where("dana_bantuan_id = ? AND spp_pembayaran_id IS NULL", id).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		// Hitung ulang status tagihan terdampak.
		for tid := range tagihanIDs {
			var t models.SppTagihan
			if err := tx.First(&t, tid).Error; err == nil {
				if err := h.recomputeStatus(tx, tid, t.Jumlah); err != nil {
					return err
				}
			}
		}
		return tx.Delete(&models.DanaBantuan{}, id).Error
	})
	if err != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Gagal menghapus dana bantuan."}}`)
	} else {
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Dana bantuan dihapus & kas/SPP disesuaikan."}}`)
	}
	h.refreshBantuan(c)
}
