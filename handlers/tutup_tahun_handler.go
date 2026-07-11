package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/models"
	"teras-keuangan/util"
)

// TutupTahunHandler menangani penutupan tahun ajaran.
type TutupTahunHandler struct {
	DB *gorm.DB
}

func NewTutupTahunHandler(db *gorm.DB) *TutupTahunHandler {
	return &TutupTahunHandler{DB: db}
}

func (h *TutupTahunHandler) activeTA() (models.TahunAjaran, bool) {
	var ta models.TahunAjaran
	ok := h.DB.Where("aktif = ?", true).First(&ta).Error == nil
	return ta, ok
}

// piutangItem adalah satu sisa tunggakan yang akan dibawa jadi piutang.
// Dibuat per-tagihan (bukan gabung per siswa) agar jejak asalnya presisi:
// SPP bulan keberapa / Tagihan jenis apa.
type piutangItem struct {
	SiswaID     uint64
	Nominal     int64
	SumberTipe  string
	SumberID    uint64
	Keterangan  string
}

func (h *TutupTahunHandler) sisaTagihan(nama string, tagihanID uint64, jumlah, lunas int64) (int64, int64) {
	var paid int64
	h.DB.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", tagihanID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
	if paid >= lunas {
		paid = lunas
	}
	return jumlah - paid, paid
}

// piutangPlan menghitung sisa tunggakan (SPP + Tagihan) per tagihan pada satu TA,
// masing-masing dibawa sebagai piutang tersendiri dengan jejak SumberTipe/SumberID.
func (h *TutupTahunHandler) piutangPlan(taID uint64) (items []piutangItem, total int64) {
	var spp []models.SppTagihan
	h.DB.Where("tahun_ajaran_id = ? AND status <> ?", taID, models.SPPLunas).Find(&spp)
	for _, st := range spp {
		var paid int64
		h.DB.Model(&models.SppPembayaran{}).Where("tagihan_id = ?", st.ID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
		if sisa := st.Jumlah - paid; sisa > 0 {
			items = append(items, piutangItem{
				SiswaID:    st.SiswaID,
				Nominal:    sisa,
				SumberTipe: models.SumberPiutangSPP,
				SumberID:   st.ID,
				Keterangan: "SPP " + util.NamaBulan(st.Bulan),
			})
			total += sisa
		}
	}

	var tg []models.Tagihan
	h.DB.Preload("JenisTagihan").Where("tahun_ajaran_id = ? AND status <> ?", taID, models.TagihanLunas).Find(&tg)
	for _, t := range tg {
		var paid int64
		h.DB.Model(&models.TagihanPembayaran{}).Where("tagihan_id = ?", t.ID).Select("COALESCE(SUM(jumlah_bayar),0)").Scan(&paid)
		if sisa := t.Nominal - paid; sisa > 0 {
			jenis := ""
			if t.JenisTagihan.ID != 0 {
				jenis = t.JenisTagihan.Nama
			}
			items = append(items, piutangItem{
				SiswaID:    t.SiswaID,
				Nominal:    sisa,
				SumberTipe: models.SumberPiutangTagihan,
				SumberID:   t.ID,
				Keterangan: jenis,
			})
			total += sisa
		}
	}
	return
}

func (h *TutupTahunHandler) tabunganDitutup(taID uint64) bool {
	var c int64
	h.DB.Model(&models.TutupTabungan{}).Where("tahun_ajaran_id = ?", taID).Count(&c)
	return c > 0
}

func (h *TutupTahunHandler) context(c *gin.Context) gin.H {
	ta, hasTA := h.activeTA()
	data := gin.H{"HasTA": hasTA, "CanEdit": canEditKas(c)}
	if !hasTA {
		return data
	}
	sum := SummaryForTA(h.DB, ta.ID)
	items, totalPiutang := h.piutangPlan(ta.ID)

	// Apakah TA aktif ini hasil tutup tahun sebelumnya (untuk opsi batal)?
	var incoming models.TutupTahun
	fromTutup := h.DB.Preload("TahunAjaranLama").Where("tahun_ajaran_baru_id = ?", ta.ID).First(&incoming).Error == nil

	data["TA"] = ta
	data["Saldo"] = sum.Saldo
	data["TotalPemasukan"] = sum.TotalPemasukan
	data["TotalPengeluaran"] = sum.TotalPengeluaran
	data["TotalPiutang"] = totalPiutang
	data["JumlahPiutang"] = len(items)
	data["TabunganDitutup"] = h.tabunganDitutup(ta.ID)
	data["FromTutup"] = fromTutup
	data["Incoming"] = incoming
	data["Today"] = time.Now().Format("2006-01-02")
	return data
}

func (h *TutupTahunHandler) Index(c *gin.Context) {
	data := h.context(c)
	if c.Query("partial") == "1" {
		c.HTML(http.StatusOK, "tutup_tahun/content", data)
		return
	}
	data["Title"] = "Tutup Tahun"
	data["ActiveMenu"] = "tutup"
	data["User"] = currentUser(c)
	c.HTML(http.StatusOK, "tutup_tahun/index", data)
}

func (h *TutupTahunHandler) refresh(c *gin.Context) {
	c.HTML(http.StatusOK, "tutup_tahun/content_oob", h.context(c))
}

// Proses menutup tahun ajaran aktif.
func (h *TutupTahunHandler) Proses(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Belum ada tahun ajaran aktif."}}`)
		h.refresh(c)
		return
	}
	namaBaru := strings.TrimSpace(c.PostForm("nama_baru"))
	if namaBaru == "" {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Nama tahun ajaran baru wajib diisi."}}`)
		h.refresh(c)
		return
	}
	tanggal := util.ParseDate(c.PostForm("tanggal"), time.Now())
	userID := ctxUserID(c)

	saldo := SummaryForTA(h.DB, ta.ID).Saldo
	items, totalPiutang := h.piutangPlan(ta.ID)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// TA baru aktif; semua TA lain non-aktif; TA lama ditutup.
		baru := models.TahunAjaran{Nama: namaBaru, Aktif: true}
		if err := tx.Create(&baru).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.TahunAjaran{}).Where("id <> ?", baru.ID).Update("aktif", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.TahunAjaran{}).Where("id = ?", ta.ID).Update("ditutup", true).Error; err != nil {
			return err
		}

		tutup := models.TutupTahun{
			TahunAjaranLamaID: ta.ID, TahunAjaranBaruID: baru.ID, Tanggal: tanggal,
			SaldoDibawa: saldo, TotalPiutang: totalPiutang, JumlahPiutang: len(items),
			Keterangan: c.PostForm("keterangan"), UserID: userID,
		}
		if err := tx.Create(&tutup).Error; err != nil {
			return err
		}

		// Saldo kas dibawa ke TA baru sebagai "Saldo Awal".
		if saldo > 0 {
			var jenis models.JenisPemasukan
			if err := tx.Where("kode = ?", models.KodeJenisSaldoAwal).First(&jenis).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.KasPemasukan{
				TahunAjaranID: baru.ID, JenisPemasukanID: jenis.ID, Tanggal: tanggal,
				Jumlah: saldo, Keterangan: "Saldo awal dari " + ta.Nama,
				UserID: userID, TutupTahunID: &tutup.ID,
			}).Error; err != nil {
				return err
			}
		}

		// Sisa tunggakan → piutang (1 baris per tagihan, berjejak sumber).
		for _, it := range items {
			p := models.Piutang{
				SiswaID:           it.SiswaID,
				TahunAjaranAsalID: ta.ID,
				TutupTahunID:      &tutup.ID,
				Nominal:           it.Nominal,
				Status:            models.TagihanBelum,
				Keterangan:        it.Keterangan,
				SumberTipe:        it.SumberTipe,
				SumberID:          &it.SumberID,
			}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Gagal menutup tahun."}}`)
	} else {
		msg := "Tahun ajaran ditutup. TA baru '" + namaBaru + "' aktif. Saldo dibawa " + util.Rupiah(saldo) + ", piutang " + util.Rupiah(totalPiutang) + "."
		c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"`+msg+`"}}`)
	}
	h.refresh(c)
}

// Batal membatalkan tutup tahun (hanya bila TA baru masih bersih).
func (h *TutupTahunHandler) Batal(c *gin.Context) {
	ta, hasTA := h.activeTA()
	if !hasTA {
		h.refresh(c)
		return
	}
	var tutup models.TutupTahun
	if h.DB.Where("tahun_ajaran_baru_id = ?", ta.ID).First(&tutup).Error != nil {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tahun ajaran aktif bukan hasil tutup tahun."}}`)
		h.refresh(c)
		return
	}

	// Guard: TA baru tidak boleh sudah punya transaksi lain / piutang terbayar.
	var piutangIDs []uint64
	h.DB.Model(&models.Piutang{}).Where("tutup_tahun_id = ?", tutup.ID).Pluck("id", &piutangIDs)
	var bayarPiutang int64
	if len(piutangIDs) > 0 {
		h.DB.Model(&models.PiutangPembayaran{}).Where("piutang_id IN ?", piutangIDs).Count(&bayarPiutang)
	}
	var kasLain, keluarLain int64
	h.DB.Model(&models.KasPemasukan{}).Where("tahun_ajaran_id = ? AND tutup_tahun_id IS NULL", ta.ID).Count(&kasLain)
	h.DB.Model(&models.KasPengeluaran{}).Where("tahun_ajaran_id = ?", ta.ID).Count(&keluarLain)
	if bayarPiutang > 0 || kasLain > 0 || keluarLain > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dibatalkan: tahun baru sudah ada transaksi/pembayaran piutang."}}`)
		h.refresh(c)
		return
	}

	lamaID := tutup.TahunAjaranLamaID
	baruID := tutup.TahunAjaranBaruID
	h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tutup_tahun_id = ?", tutup.ID).Delete(&models.Piutang{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tutup_tahun_id = ?", tutup.ID).Delete(&models.KasPemasukan{}).Error; err != nil {
			return err
		}
		// Aktifkan lagi TA lama, buka kuncinya.
		if err := tx.Model(&models.TahunAjaran{}).Where("id = ?", lamaID).Updates(map[string]interface{}{"aktif": true, "ditutup": false}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.TutupTahun{}, tutup.ID).Error; err != nil {
			return err
		}
		// Hapus TA baru (kosong).
		return tx.Delete(&models.TahunAjaran{}, baruID).Error
	})
	c.Header("HX-Trigger", `{"toast":{"type":"success","msg":"Tutup tahun dibatalkan; tahun ajaran lama aktif kembali."}}`)
	h.refresh(c)
}
