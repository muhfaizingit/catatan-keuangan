package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/middleware"
	"teras-keuangan/models"
	"teras-keuangan/util"
)

// LaporanHandler menangani laporan keuangan (Kas, dll).
type LaporanHandler struct {
	DB *gorm.DB
}

func NewLaporanHandler(db *gorm.DB) *LaporanHandler {
	return &LaporanHandler{DB: db}
}

// barisLaporanKategori adalah satu baris ringkasan per kategori/jenis.
type barisLaporanKategori struct {
	Nama   string
	Jumlah int64
}

// LaporanKas menampilkan laporan kas bulanan untuk satu tahun ajaran + bulan.
// Filter: ?ta=<id>&bulan=<1-12>. Default bulan = bulan berjalan.
func (h *LaporanHandler) LaporanKas(c *gin.Context) {
	now := time.Now()
	taID, _ := strconv.ParseUint(c.Query("ta"), 10, 64)
	if taID == 0 {
		taID = activeTahunAjaranID(h.DB)
	}
	bulan, _ := strconv.Atoi(c.Query("bulan"))
	if bulan < 1 || bulan > 12 {
		bulan = int(now.Month())
	}

	var ta models.TahunAjaran
	hasTA := h.DB.First(&ta, taID).Error == nil

	data := gin.H{
		"Title":      "Laporan · Kas",
		"ActiveMenu": "laporan-kas",
		"User":       currentUser(c),
		"HasTA":      hasTA,
		"SelectedTA": taID,
		"Bulan":      bulan,
		"TahunList":  tahunAjaranAll(h.DB),
		"NamaBulan":  util.NamaBulan(bulan),
		"Tahun":      now.Year(),
	}

	if !hasTA {
		c.HTML(http.StatusOK, "laporan/kas", data)
		return
	}

	// Batas tanggal: awal & akhir bulan terpilih.
	awal := time.Date(now.Year(), time.Month(bulan), 1, 0, 0, 0, 0, time.Local)
	akhir := awal.AddDate(0, 1, 0).Add(-time.Second)

	// Saldo awal = akumulasi semua kas s/d akhir bulan sebelumnya.
	saldoAwal := h.saldoSampai(taID, awal.Add(-time.Second))

	// Pemasukan bulan berjalan (diurut nama jenis untuk laporan).
	var pemasukan []models.KasPemasukan
	h.DB.Preload("JenisPemasukan").
		Where("tahun_ajaran_id = ? AND tanggal >= ? AND tanggal <= ?", taID, awal, akhir).
		Order("tanggal asc, id asc").Find(&pemasukan)

	// Pengeluaran bulan berjalan.
	var pengeluaran []models.KasPengeluaran
	h.DB.Preload("Kategori").Preload("SubKategori").
		Where("tahun_ajaran_id = ? AND tanggal >= ? AND tanggal <= ?", taID, awal, akhir).
		Order("tanggal asc, id asc").Find(&pengeluaran)

	// Ringkasan per jenis pemasukan.
	totalPemasukan, perJenis := h.ringkasPemasukan(pemasukan)
	// Ringkasan per kategori pengeluaran.
	totalPengeluaran, perKategori := h.ringkasPengeluaran(pengeluaran)

	saldoAkhir := saldoAwal + totalPemasukan - totalPengeluaran

	data["NamaTA"] = ta.Nama
	data["SaldoAwal"] = saldoAwal
	data["TotalPemasukan"] = totalPemasukan
	data["TotalPengeluaran"] = totalPengeluaran
	data["SaldoAkhir"] = saldoAkhir
	data["PerJenis"] = perJenis
	data["PerKategori"] = perKategori
	data["Pemasukan"] = pemasukan
	data["Pengeluaran"] = pengeluaran
	data["JumlahTrx"] = len(pemasukan) + len(pengeluaran)

	c.HTML(http.StatusOK, "laporan/kas", data)
}

// CetakKas versi printer-friendly (tanpa chrome aplikasi).
func (h *LaporanHandler) CetakKas(c *gin.Context) {
	// Pakai handler yang sama untuk menghitung data, lalu render template cetak.
	h.LaporanKas(c)
}

// saldoSampai menghitung saldo kas (pemasukan - pengeluaran) s/d tanggal tertentu.
func (h *LaporanHandler) saldoSampai(taID uint64, sampai time.Time) int64 {
	var in, out int64
	h.DB.Model(&models.KasPemasukan{}).
		Where("tahun_ajaran_id = ? AND tanggal <= ?", taID, sampai).
		Select("COALESCE(SUM(jumlah),0)").Scan(&in)
	h.DB.Model(&models.KasPengeluaran{}).
		Where("tahun_ajaran_id = ? AND tanggal <= ?", taID, sampai).
		Select("COALESCE(SUM(jumlah),0)").Scan(&out)
	return in - out
}

// ringkasPemasukan mengelompokkan pemasukan per jenis, mengembalikan total & baris.
func (h *LaporanHandler) ringkasPemasukan(list []models.KasPemasukan) (int64, []barisLaporanKategori) {
	total := int64(0)
	agg := map[string]int64{}
	for _, r := range list {
		nama := "Lainnya"
		if r.JenisPemasukan.ID != 0 {
			nama = r.JenisPemasukan.Nama
		}
		agg[nama] += r.Jumlah
		total += r.Jumlah
	}
	rows := make([]barisLaporanKategori, 0, len(agg))
	for nama, j := range agg {
		rows = append(rows, barisLaporanKategori{Nama: nama, Jumlah: j})
	}
	// Urut nama naik (stabil, sederhana).
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Nama < rows[i].Nama {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return total, rows
}

// ringkasPengeluaran mengelompokkan pengeluaran per kategori, mengembalikan total & baris.
func (h *LaporanHandler) ringkasPengeluaran(list []models.KasPengeluaran) (int64, []barisLaporanKategori) {
	total := int64(0)
	agg := map[string]int64{}
	for _, r := range list {
		nama := "Lainnya"
		if r.Kategori.ID != 0 {
			nama = r.Kategori.Nama
		}
		agg[nama] += r.Jumlah
		total += r.Jumlah
	}
	rows := make([]barisLaporanKategori, 0, len(agg))
	for nama, j := range agg {
		rows = append(rows, barisLaporanKategori{Nama: nama, Jumlah: j})
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Nama < rows[i].Nama {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return total, rows
}

// =====================================================================
// FILTER BERSAMA (kelas + bulan + range tanggal) untuk laporan Tabungan & SPP
// =====================================================================

type laporanFilter struct {
	TaID      uint64
	KelasID   uint64
	Bulan     int
	Dari      time.Time
	Sampai    time.Time
	UseRange  bool
	DariStr   string
	SampaiStr string
}

func (h *LaporanHandler) parseFilter(c *gin.Context) laporanFilter {
	var f laporanFilter
	f.TaID, _ = strconv.ParseUint(c.Query("ta"), 10, 64)
	if f.TaID == 0 {
		f.TaID = activeTahunAjaranID(h.DB)
	}
	f.KelasID, _ = strconv.ParseUint(c.Query("kelas"), 10, 64)
	f.Bulan, _ = strconv.Atoi(c.Query("bulan"))
	f.DariStr = strings.TrimSpace(c.Query("dari"))
	f.SampaiStr = strings.TrimSpace(c.Query("sampai"))

	if t, err := time.ParseInLocation("2006-01-02", f.DariStr, time.Local); err == nil {
		f.Dari = t
		f.UseRange = true
	}
	if t, err := time.ParseInLocation("2006-01-02", f.SampaiStr, time.Local); err == nil {
		f.Sampai = t.AddDate(0, 0, 1).Add(-time.Second) // sampai akhir hari terpilih
		f.UseRange = true
	}
	return f
}

// applyTanggal menambahkan kondisi tanggal ke query sesuai mode aktif
// (range tanggal mengalahkan filter bulan).
func (f laporanFilter) applyTanggal(q *gorm.DB, col string) *gorm.DB {
	if f.UseRange {
		if !f.Dari.IsZero() {
			q = q.Where(col+" >= ?", f.Dari)
		}
		if !f.Sampai.IsZero() {
			q = q.Where(col+" <= ?", f.Sampai)
		}
		return q
	}
	if f.Bulan >= 1 && f.Bulan <= 12 {
		q = q.Where("MONTH("+col+") = ?", f.Bulan)
	}
	return q
}

func (f laporanFilter) periodeLabel() string {
	if f.UseRange {
		d, s := "awal", "sekarang"
		if !f.Dari.IsZero() {
			d = f.Dari.Format("02 Jan 2006")
		}
		if !f.Sampai.IsZero() {
			s = f.Sampai.Format("02 Jan 2006")
		}
		return d + " – " + s
	}
	if f.Bulan >= 1 && f.Bulan <= 12 {
		return util.NamaBulan(f.Bulan)
	}
	return "Semua bulan"
}

func (h *LaporanHandler) kelasByTA(taID uint64) []models.Kelas {
	var list []models.Kelas
	h.DB.Where("tahun_ajaran_id = ?", taID).Order("nama asc").Find(&list)
	return list
}

// filterData menyiapkan field-field yang dipakai oleh filter bar & header.
func (h *LaporanHandler) filterData(f laporanFilter, ta models.TahunAjaran, hasTA bool) gin.H {
	return gin.H{
		"HasTA":         hasTA,
		"NamaTA":        ta.Nama,
		"SelectedTA":    f.TaID,
		"TahunList":     tahunAjaranAll(h.DB),
		"KelasList":     h.kelasByTA(f.TaID),
		"SelectedKelas": f.KelasID,
		"Bulan":         f.Bulan,
		"Dari":          f.DariStr,
		"Sampai":        f.SampaiStr,
		"Periode":       f.periodeLabel(),
	}
}

// =====================================================================
// LAPORAN TABUNGAN
// =====================================================================

func (h *LaporanHandler) LaporanTabungan(c *gin.Context) {
	f := h.parseFilter(c)
	var ta models.TahunAjaran
	hasTA := h.DB.First(&ta, f.TaID).Error == nil

	data := h.filterData(f, ta, hasTA)
	data["Title"] = "Laporan · Tabungan"
	data["ActiveMenu"] = "laporan-tabungan"
	data["User"] = currentUser(c)

	if !hasTA {
		c.HTML(http.StatusOK, "laporan/tabungan", data)
		return
	}

	type tx struct {
		Tanggal    time.Time
		Jumlah     int64
		Keterangan string
		SiswaID    uint64
		SiswaNama  string
		NIS        string
		KelasNama  string
	}

	var setoran, penarikan []tx
	qs := h.DB.Table("tabungan_setoran AS x").
		Select("x.tanggal, x.jumlah_setor AS jumlah, x.keterangan, x.siswa_id, s.nama AS siswa_nama, s.nis AS nis, COALESCE(k.nama,'') AS kelas_nama").
		Joins("JOIN siswa s ON s.id = x.siswa_id").
		Joins("LEFT JOIN kelas k ON k.id = s.kelas_id").
		Where("x.tahun_ajaran_id = ?", f.TaID)
	if f.KelasID != 0 {
		qs = qs.Where("s.kelas_id = ?", f.KelasID)
	}
	qs = f.applyTanggal(qs, "x.tanggal")
	qs.Order("x.tanggal asc, x.id asc").Scan(&setoran)

	qp := h.DB.Table("tabungan_penarikan AS x").
		Select("x.tanggal, x.jumlah AS jumlah, x.keterangan, x.siswa_id, s.nama AS siswa_nama, s.nis AS nis, COALESCE(k.nama,'') AS kelas_nama").
		Joins("JOIN siswa s ON s.id = x.siswa_id").
		Joins("LEFT JOIN kelas k ON k.id = s.kelas_id").
		Where("x.tahun_ajaran_id = ?", f.TaID)
	if f.KelasID != 0 {
		qp = qp.Where("s.kelas_id = ?", f.KelasID)
	}
	qp = f.applyTanggal(qp, "x.tanggal")
	qp.Order("x.tanggal asc, x.id asc").Scan(&penarikan)

	// Rekap per siswa.
	type rekapRow struct {
		Nama, NIS, Kelas  string
		Setor, Tarik, Net int64
	}
	rekapMap := map[uint64]*rekapRow{}
	var totalSetor, totalTarik int64
	for _, s := range setoran {
		totalSetor += s.Jumlah
		r := rekapMap[s.SiswaID]
		if r == nil {
			r = &rekapRow{Nama: s.SiswaNama, NIS: s.NIS, Kelas: s.KelasNama}
			rekapMap[s.SiswaID] = r
		}
		r.Setor += s.Jumlah
	}
	for _, p := range penarikan {
		totalTarik += p.Jumlah
		r := rekapMap[p.SiswaID]
		if r == nil {
			r = &rekapRow{Nama: p.SiswaNama, NIS: p.NIS, Kelas: p.KelasNama}
			rekapMap[p.SiswaID] = r
		}
		r.Tarik += p.Jumlah
	}
	rekap := make([]rekapRow, 0, len(rekapMap))
	for _, r := range rekapMap {
		r.Net = r.Setor - r.Tarik
		rekap = append(rekap, *r)
	}
	sort.Slice(rekap, func(i, j int) bool { return rekap[i].Nama < rekap[j].Nama })

	// Detail transaksi gabungan (urut tanggal).
	type detailRow struct {
		Tanggal                               time.Time
		SiswaNama, NIS, KelasNama, Keterangan string
		Setor, Tarik                          int64
	}
	detail := make([]detailRow, 0, len(setoran)+len(penarikan))
	for _, s := range setoran {
		detail = append(detail, detailRow{s.Tanggal, s.SiswaNama, s.NIS, s.KelasNama, s.Keterangan, s.Jumlah, 0})
	}
	for _, p := range penarikan {
		detail = append(detail, detailRow{p.Tanggal, p.SiswaNama, p.NIS, p.KelasNama, p.Keterangan, 0, p.Jumlah})
	}
	sort.SliceStable(detail, func(i, j int) bool { return detail[i].Tanggal.Before(detail[j].Tanggal) })

	data["TotalSetor"] = totalSetor
	data["TotalTarik"] = totalTarik
	data["Net"] = totalSetor - totalTarik
	data["Rekap"] = rekap
	data["Detail"] = detail
	data["JumlahTrx"] = len(detail)

	c.HTML(http.StatusOK, "laporan/tabungan", data)
}

// =====================================================================
// LAPORAN SPP
// =====================================================================

func (h *LaporanHandler) LaporanSPP(c *gin.Context) {
	f := h.parseFilter(c)
	var ta models.TahunAjaran
	hasTA := h.DB.First(&ta, f.TaID).Error == nil

	data := h.filterData(f, ta, hasTA)
	data["Title"] = "Laporan · SPP"
	data["ActiveMenu"] = "laporan-spp"
	data["User"] = currentUser(c)

	if !hasTA {
		c.HTML(http.StatusOK, "laporan/spp", data)
		return
	}

	type bayarRow struct {
		Tanggal   time.Time
		Jumlah    int64
		Sumber    string
		BulanSpp  int
		SiswaNama string
		NIS       string
		KelasNama string
	}
	var rows []bayarRow
	q := h.DB.Table("spp_pembayaran AS p").
		Select("p.tanggal, p.jumlah_bayar AS jumlah, p.sumber, t.bulan AS bulan_spp, s.nama AS siswa_nama, s.nis AS nis, COALESCE(k.nama,'') AS kelas_nama").
		Joins("JOIN spp_tagihan t ON t.id = p.tagihan_id").
		Joins("JOIN siswa s ON s.id = t.siswa_id").
		Joins("LEFT JOIN kelas k ON k.id = s.kelas_id").
		Where("t.tahun_ajaran_id = ?", f.TaID)
	if f.KelasID != 0 {
		q = q.Where("s.kelas_id = ?", f.KelasID)
	}
	q = f.applyTanggal(q, "p.tanggal")
	q.Order("p.tanggal asc, p.id asc").Scan(&rows)

	var total, tunai, bantuan, tabungan int64
	perKelasMap := map[string]int64{}
	for _, r := range rows {
		total += r.Jumlah
		switch r.Sumber {
		case models.SumberBantuan:
			bantuan += r.Jumlah
		case models.SumberTabungan:
			tabungan += r.Jumlah
		default:
			tunai += r.Jumlah
		}
		nama := r.KelasNama
		if nama == "" {
			nama = "(Tanpa kelas)"
		}
		perKelasMap[nama] += r.Jumlah
	}
	perKelas := make([]barisLaporanKategori, 0, len(perKelasMap))
	for nama, j := range perKelasMap {
		perKelas = append(perKelas, barisLaporanKategori{Nama: nama, Jumlah: j})
	}
	sort.Slice(perKelas, func(i, j int) bool { return perKelas[i].Nama < perKelas[j].Nama })

	data["Total"] = total
	data["Tunai"] = tunai
	data["Bantuan"] = bantuan
	data["Tabungan"] = tabungan
	data["PerKelas"] = perKelas
	data["List"] = rows
	data["JumlahTrx"] = len(rows)

	c.HTML(http.StatusOK, "laporan/spp", data)
}

// roleLaporan: semua role (admin, bendahara, kepala sekolah) boleh lihat laporan.
func canViewLaporan(c *gin.Context) bool {
	role, _ := c.Get(middleware.CtxRole)
	r, _ := role.(string)
	return r == string(models.RoleAdmin) || r == string(models.RoleBendahara) || r == string(models.RoleKepalaSekolah)
}
