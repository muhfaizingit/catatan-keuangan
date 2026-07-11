package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/middleware"
	"teras-keuangan/models"
	"teras-keuangan/util"
)

// DashboardHandler menangani halaman ringkasan utama.
type DashboardHandler struct {
	DB *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{DB: db}
}

// Index menampilkan dashboard dengan ringkasan kas tahun ajaran aktif.
func (h *DashboardHandler) Index(c *gin.Context) {
	var ta models.TahunAjaran
	hasTA := h.DB.Where("aktif = ?", true).First(&ta).Error == nil

	var summary KasSummary
	var pemasukanBulan, pengeluaranBulan int64
	bulan := int(time.Now().Month())

	if hasTA {
		summary = SummaryForTA(h.DB, ta.ID)
		h.DB.Model(&models.KasPemasukan{}).
			Where("tahun_ajaran_id = ? AND MONTH(tanggal) = ?", ta.ID, bulan).
			Select("COALESCE(SUM(jumlah),0)").Scan(&pemasukanBulan)
		h.DB.Model(&models.KasPengeluaran{}).
			Where("tahun_ajaran_id = ? AND MONTH(tanggal) = ?", ta.ID, bulan).
			Select("COALESCE(SUM(jumlah),0)").Scan(&pengeluaranBulan)
	}

	c.HTML(http.StatusOK, "dashboard/index", gin.H{
		"Title":            "Dashboard",
		"ActiveMenu":       "dashboard",
		"User":             currentUser(c),
		"HasTA":            hasTA,
		"TANama":           ta.Nama,
		"NamaBulan":        util.NamaBulan(bulan),
		"Saldo":            summary.Saldo,
		"TotalPemasukan":   summary.TotalPemasukan,
		"TotalPengeluaran": summary.TotalPengeluaran,
		"PemasukanBulan":   pemasukanBulan,
		"PengeluaranBulan": pengeluaranBulan,
	})
}

// currentUser merangkum data user dari context untuk dipakai di template.
func currentUser(c *gin.Context) gin.H {
	nama, _ := c.Get(middleware.CtxNama)
	email, _ := c.Get(middleware.CtxEmail)
	role, _ := c.Get(middleware.CtxRole)
	return gin.H{
		"Nama":  nama,
		"Email": email,
		"Role":  role,
	}
}
