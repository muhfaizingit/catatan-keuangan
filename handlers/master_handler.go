package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/models"
)

// MasterHandler menangani seluruh CRUD master data.
type MasterHandler struct {
	DB *gorm.DB
}

func NewMasterHandler(db *gorm.DB) *MasterHandler {
	return &MasterHandler{DB: db}
}

// ---------- util ----------

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// =====================================================================
// TAHUN AJARAN
// =====================================================================

func (h *MasterHandler) tahunAjaranList() []models.TahunAjaran {
	var list []models.TahunAjaran
	h.DB.Order("nama desc").Find(&list)
	return list
}

func (h *MasterHandler) TahunAjaranIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/tahun_ajaran", gin.H{
		"Title":      "Master · Tahun Ajaran",
		"ActiveMenu": "master",
		"ActiveTab":  "tahun-ajaran",
		"User":       currentUser(c),
		"List":       h.tahunAjaranList(),
	})
}

func (h *MasterHandler) TahunAjaranForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Tahun Ajaran", "Action": "/master/tahun-ajaran", "Edit": false, "Item": models.TahunAjaran{}}
	if id, ok := parseID(c); ok {
		var ta models.TahunAjaran
		if err := h.DB.First(&ta, id).Error; err == nil {
			data["Title"] = "Edit Tahun Ajaran"
			data["Action"] = "/master/tahun-ajaran/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = ta
		}
	}
	c.HTML(http.StatusOK, "master/tahun_ajaran_form", data)
}

func (h *MasterHandler) tahunAjaranRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/tahun_ajaran_rows_oob", gin.H{"List": h.tahunAjaranList()})
}

func (h *MasterHandler) tahunAjaranFormErr(c *gin.Context, ta models.TahunAjaran, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/tahun_ajaran_form", gin.H{
		"Title": map[bool]string{true: "Edit Tahun Ajaran", false: "Tambah Tahun Ajaran"}[edit],
		"Action": action, "Edit": edit, "Item": ta, "Error": msg,
	})
}

func (h *MasterHandler) TahunAjaranCreate(c *gin.Context) {
	nama := strings.TrimSpace(c.PostForm("nama"))
	aktif := c.PostForm("aktif") == "on"
	ta := models.TahunAjaran{Nama: nama, Aktif: aktif}
	if nama == "" {
		h.tahunAjaranFormErr(c, ta, false, "/master/tahun-ajaran", "Nama tahun ajaran wajib diisi.")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ta).Error; err != nil {
			return err
		}
		if aktif {
			return tx.Model(&models.TahunAjaran{}).Where("id <> ?", ta.ID).Update("aktif", false).Error
		}
		return nil
	}); err != nil {
		h.tahunAjaranFormErr(c, ta, false, "/master/tahun-ajaran", "Gagal menyimpan data.")
		return
	}
	h.tahunAjaranRefresh(c)
}

func (h *MasterHandler) TahunAjaranUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var ta models.TahunAjaran
	if err := h.DB.First(&ta, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/tahun-ajaran/" + c.Param("id")
	nama := strings.TrimSpace(c.PostForm("nama"))
	aktif := c.PostForm("aktif") == "on"
	ta.Nama, ta.Aktif = nama, aktif
	if nama == "" {
		h.tahunAjaranFormErr(c, ta, true, action, "Nama tahun ajaran wajib diisi.")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&ta).Error; err != nil {
			return err
		}
		if aktif {
			return tx.Model(&models.TahunAjaran{}).Where("id <> ?", ta.ID).Update("aktif", false).Error
		}
		return nil
	}); err != nil {
		h.tahunAjaranFormErr(c, ta, true, action, "Gagal menyimpan data.")
		return
	}
	h.tahunAjaranRefresh(c)
}

func (h *MasterHandler) TahunAjaranSetAktif(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	h.DB.Transaction(func(tx *gorm.DB) error {
		tx.Model(&models.TahunAjaran{}).Where("1 = 1").Update("aktif", false)
		return tx.Model(&models.TahunAjaran{}).Where("id = ?", id).Update("aktif", true).Error
	})
	h.tahunAjaranRefresh(c)
}

func (h *MasterHandler) TahunAjaranDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var jumlahKelas int64
	h.DB.Model(&models.Kelas{}).Where("tahun_ajaran_id = ?", id).Count(&jumlahKelas)
	if jumlahKelas > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dihapus: masih ada kelas yang memakai tahun ajaran ini."}}`)
		h.tahunAjaranRefresh(c)
		return
	}
	h.DB.Delete(&models.TahunAjaran{}, id)
	h.tahunAjaranRefresh(c)
}

// =====================================================================
// KELAS
// =====================================================================

func (h *MasterHandler) kelasList() []models.Kelas {
	var list []models.Kelas
	h.DB.Preload("TahunAjaran").Order("id desc").Find(&list)
	return list
}

func (h *MasterHandler) KelasIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/kelas", gin.H{
		"Title":      "Master · Kelas",
		"ActiveMenu": "master",
		"ActiveTab":  "kelas",
		"User":       currentUser(c),
		"List":       h.kelasList(),
	})
}

func (h *MasterHandler) KelasForm(c *gin.Context) {
	data := gin.H{
		"Title": "Tambah Kelas", "Action": "/master/kelas", "Edit": false,
		"Item": models.Kelas{}, "TahunList": h.tahunAjaranList(),
	}
	if id, ok := parseID(c); ok {
		var k models.Kelas
		if err := h.DB.First(&k, id).Error; err == nil {
			data["Title"] = "Edit Kelas"
			data["Action"] = "/master/kelas/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = k
		}
	}
	c.HTML(http.StatusOK, "master/kelas_form", data)
}

func (h *MasterHandler) kelasRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/kelas_rows_oob", gin.H{"List": h.kelasList()})
}

func (h *MasterHandler) kelasFormErr(c *gin.Context, k models.Kelas, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/kelas_form", gin.H{
		"Title":  map[bool]string{true: "Edit Kelas", false: "Tambah Kelas"}[edit],
		"Action": action, "Edit": edit, "Item": k, "Error": msg, "TahunList": h.tahunAjaranList(),
	})
}

func (h *MasterHandler) bindKelas(c *gin.Context) models.Kelas {
	taID, _ := strconv.ParseUint(c.PostForm("tahun_ajaran_id"), 10, 64)
	return models.Kelas{Nama: strings.TrimSpace(c.PostForm("nama")), TahunAjaranID: taID}
}

func (h *MasterHandler) validKelas(k models.Kelas) string {
	if k.Nama == "" {
		return "Nama kelas wajib diisi."
	}
	if k.TahunAjaranID == 0 {
		return "Tahun ajaran wajib dipilih."
	}
	return ""
}

func (h *MasterHandler) KelasCreate(c *gin.Context) {
	k := h.bindKelas(c)
	if msg := h.validKelas(k); msg != "" {
		h.kelasFormErr(c, k, false, "/master/kelas", msg)
		return
	}
	if err := h.DB.Create(&k).Error; err != nil {
		h.kelasFormErr(c, k, false, "/master/kelas", "Gagal menyimpan data.")
		return
	}
	h.kelasRefresh(c)
}

func (h *MasterHandler) KelasUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var k models.Kelas
	if err := h.DB.First(&k, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/kelas/" + c.Param("id")
	in := h.bindKelas(c)
	k.Nama, k.TahunAjaranID = in.Nama, in.TahunAjaranID
	if msg := h.validKelas(k); msg != "" {
		h.kelasFormErr(c, k, true, action, msg)
		return
	}
	if err := h.DB.Save(&k).Error; err != nil {
		h.kelasFormErr(c, k, true, action, "Gagal menyimpan data.")
		return
	}
	h.kelasRefresh(c)
}

func (h *MasterHandler) KelasDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var jumlahSiswa int64
	h.DB.Model(&models.Siswa{}).Where("kelas_id = ?", id).Count(&jumlahSiswa)
	if jumlahSiswa > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dihapus: masih ada siswa di kelas ini."}}`)
		h.kelasRefresh(c)
		return
	}
	h.DB.Delete(&models.Kelas{}, id)
	h.kelasRefresh(c)
}

// =====================================================================
// SISWA
// =====================================================================

func (h *MasterHandler) siswaList() []models.Siswa {
	var list []models.Siswa
	h.DB.Preload("Kelas.TahunAjaran").Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) SiswaIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/siswa", gin.H{
		"Title":      "Master · Siswa",
		"ActiveMenu": "master",
		"ActiveTab":  "siswa",
		"User":       currentUser(c),
		"List":       h.siswaList(),
	})
}

func (h *MasterHandler) SiswaForm(c *gin.Context) {
	data := gin.H{
		"Title": "Tambah Siswa", "Action": "/master/siswa", "Edit": false,
		"Item": models.Siswa{Aktif: true}, "KelasList": h.kelasList(),
	}
	if id, ok := parseID(c); ok {
		var s models.Siswa
		if err := h.DB.First(&s, id).Error; err == nil {
			data["Title"] = "Edit Siswa"
			data["Action"] = "/master/siswa/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = s
		}
	}
	c.HTML(http.StatusOK, "master/siswa_form", data)
}

func (h *MasterHandler) siswaRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/siswa_rows_oob", gin.H{"List": h.siswaList()})
}

func (h *MasterHandler) siswaFormErr(c *gin.Context, s models.Siswa, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/siswa_form", gin.H{
		"Title":  map[bool]string{true: "Edit Siswa", false: "Tambah Siswa"}[edit],
		"Action": action, "Edit": edit, "Item": s, "Error": msg, "KelasList": h.kelasList(),
	})
}

func (h *MasterHandler) bindSiswa(c *gin.Context) models.Siswa {
	s := models.Siswa{
		NIS:   strings.TrimSpace(c.PostForm("nis")),
		Nama:  strings.TrimSpace(c.PostForm("nama")),
		Aktif: c.PostForm("aktif") == "on",
	}
	if v := c.PostForm("kelas_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			s.KelasID = &id
		}
	}
	return s
}

func (h *MasterHandler) SiswaCreate(c *gin.Context) {
	s := h.bindSiswa(c)
	if s.NIS == "" || s.Nama == "" {
		h.siswaFormErr(c, s, false, "/master/siswa", "NIS dan nama wajib diisi.")
		return
	}
	if err := h.DB.Create(&s).Error; err != nil {
		h.siswaFormErr(c, s, false, "/master/siswa", "Gagal menyimpan. Pastikan NIS belum dipakai.")
		return
	}
	h.siswaRefresh(c)
}

func (h *MasterHandler) SiswaUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var s models.Siswa
	if err := h.DB.First(&s, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/siswa/" + c.Param("id")
	in := h.bindSiswa(c)
	s.NIS, s.Nama, s.Aktif, s.KelasID = in.NIS, in.Nama, in.Aktif, in.KelasID
	if s.NIS == "" || s.Nama == "" {
		h.siswaFormErr(c, s, true, action, "NIS dan nama wajib diisi.")
		return
	}
	if err := h.DB.Save(&s).Error; err != nil {
		h.siswaFormErr(c, s, true, action, "Gagal menyimpan. Pastikan NIS belum dipakai.")
		return
	}
	h.siswaRefresh(c)
}

func (h *MasterHandler) SiswaDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	h.DB.Delete(&models.Siswa{}, id)
	h.siswaRefresh(c)
}

// =====================================================================
// JENIS PEMASUKAN
// =====================================================================

func (h *MasterHandler) jenisList() []models.JenisPemasukan {
	var list []models.JenisPemasukan
	h.DB.Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) JenisIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/jenis_pemasukan", gin.H{
		"Title":      "Master · Jenis Pemasukan",
		"ActiveMenu": "master",
		"ActiveTab":  "jenis-pemasukan",
		"User":       currentUser(c),
		"List":       h.jenisList(),
	})
}

func (h *MasterHandler) JenisForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Jenis Pemasukan", "Action": "/master/jenis-pemasukan", "Edit": false, "Item": models.JenisPemasukan{Aktif: true}}
	if id, ok := parseID(c); ok {
		var j models.JenisPemasukan
		if err := h.DB.First(&j, id).Error; err == nil {
			data["Title"] = "Edit Jenis Pemasukan"
			data["Action"] = "/master/jenis-pemasukan/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = j
		}
	}
	c.HTML(http.StatusOK, "master/jenis_pemasukan_form", data)
}

func (h *MasterHandler) jenisRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/jenis_pemasukan_rows_oob", gin.H{"List": h.jenisList()})
}

func (h *MasterHandler) jenisFormErr(c *gin.Context, j models.JenisPemasukan, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/jenis_pemasukan_form", gin.H{
		"Title":  map[bool]string{true: "Edit Jenis Pemasukan", false: "Tambah Jenis Pemasukan"}[edit],
		"Action": action, "Edit": edit, "Item": j, "Error": msg,
	})
}

func (h *MasterHandler) bindJenis(c *gin.Context) models.JenisPemasukan {
	return models.JenisPemasukan{
		Nama:       strings.TrimSpace(c.PostForm("nama")),
		Keterangan: strings.TrimSpace(c.PostForm("keterangan")),
		Aktif:      c.PostForm("aktif") == "on",
	}
}

func (h *MasterHandler) JenisCreate(c *gin.Context) {
	j := h.bindJenis(c)
	if j.Nama == "" {
		h.jenisFormErr(c, j, false, "/master/jenis-pemasukan", "Nama wajib diisi.")
		return
	}
	if err := h.DB.Create(&j).Error; err != nil {
		h.jenisFormErr(c, j, false, "/master/jenis-pemasukan", "Gagal menyimpan data.")
		return
	}
	h.jenisRefresh(c)
}

func (h *MasterHandler) JenisUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var j models.JenisPemasukan
	if err := h.DB.First(&j, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if j.IsSistem() {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Jenis bawaan sistem tidak dapat diubah."}}`)
		h.jenisRefresh(c)
		return
	}
	action := "/master/jenis-pemasukan/" + c.Param("id")
	in := h.bindJenis(c)
	j.Nama, j.Keterangan, j.Aktif = in.Nama, in.Keterangan, in.Aktif
	if j.Nama == "" {
		h.jenisFormErr(c, j, true, action, "Nama wajib diisi.")
		return
	}
	if err := h.DB.Save(&j).Error; err != nil {
		h.jenisFormErr(c, j, true, action, "Gagal menyimpan data.")
		return
	}
	h.jenisRefresh(c)
}

func (h *MasterHandler) JenisDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var j models.JenisPemasukan
	if err := h.DB.First(&j, id).Error; err == nil && j.IsSistem() {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Jenis bawaan sistem tidak dapat dihapus."}}`)
		h.jenisRefresh(c)
		return
	}
	h.DB.Delete(&models.JenisPemasukan{}, id)
	h.jenisRefresh(c)
}

// =====================================================================
// DONATUR
// =====================================================================

func (h *MasterHandler) donaturList() []models.Donatur {
	var list []models.Donatur
	h.DB.Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) DonaturIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/donatur", gin.H{
		"Title":      "Master · Donatur",
		"ActiveMenu": "master",
		"ActiveTab":  "donatur",
		"User":       currentUser(c),
		"List":       h.donaturList(),
	})
}

func (h *MasterHandler) DonaturForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Donatur", "Action": "/master/donatur", "Edit": false, "Item": models.Donatur{Aktif: true}}
	if id, ok := parseID(c); ok {
		var d models.Donatur
		if err := h.DB.First(&d, id).Error; err == nil {
			data["Title"] = "Edit Donatur"
			data["Action"] = "/master/donatur/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = d
		}
	}
	c.HTML(http.StatusOK, "master/donatur_form", data)
}

func (h *MasterHandler) donaturRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/donatur_rows_oob", gin.H{"List": h.donaturList()})
}

func (h *MasterHandler) donaturFormErr(c *gin.Context, d models.Donatur, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/donatur_form", gin.H{
		"Title":  map[bool]string{true: "Edit Donatur", false: "Tambah Donatur"}[edit],
		"Action": action, "Edit": edit, "Item": d, "Error": msg,
	})
}

func (h *MasterHandler) bindDonatur(c *gin.Context) models.Donatur {
	return models.Donatur{
		Nama:       strings.TrimSpace(c.PostForm("nama")),
		Keterangan: strings.TrimSpace(c.PostForm("keterangan")),
		Aktif:      c.PostForm("aktif") == "on",
	}
}

func (h *MasterHandler) DonaturCreate(c *gin.Context) {
	d := h.bindDonatur(c)
	if d.Nama == "" {
		h.donaturFormErr(c, d, false, "/master/donatur", "Nama donatur wajib diisi.")
		return
	}
	if err := h.DB.Create(&d).Error; err != nil {
		h.donaturFormErr(c, d, false, "/master/donatur", "Gagal menyimpan data.")
		return
	}
	h.donaturRefresh(c)
}

func (h *MasterHandler) DonaturUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var d models.Donatur
	if err := h.DB.First(&d, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/donatur/" + c.Param("id")
	in := h.bindDonatur(c)
	d.Nama, d.Keterangan, d.Aktif = in.Nama, in.Keterangan, in.Aktif
	if d.Nama == "" {
		h.donaturFormErr(c, d, true, action, "Nama donatur wajib diisi.")
		return
	}
	if err := h.DB.Save(&d).Error; err != nil {
		h.donaturFormErr(c, d, true, action, "Gagal menyimpan data.")
		return
	}
	h.donaturRefresh(c)
}

func (h *MasterHandler) DonaturDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var pakai int64
	h.DB.Model(&models.DanaBantuan{}).Where("donatur_id = ?", id).Count(&pakai)
	if pakai > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dihapus: donatur sudah punya catatan dana bantuan."}}`)
		h.donaturRefresh(c)
		return
	}
	h.DB.Delete(&models.Donatur{}, id)
	h.donaturRefresh(c)
}

// =====================================================================
// GURU
// =====================================================================

func (h *MasterHandler) guruList() []models.Guru {
	var list []models.Guru
	h.DB.Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) GuruIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/guru", gin.H{
		"Title":      "Master · Guru",
		"ActiveMenu": "master",
		"ActiveTab":  "guru",
		"User":       currentUser(c),
		"List":       h.guruList(),
	})
}

func (h *MasterHandler) GuruForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Guru", "Action": "/master/guru", "Edit": false, "Item": models.Guru{Aktif: true}}
	if id, ok := parseID(c); ok {
		var g models.Guru
		if err := h.DB.First(&g, id).Error; err == nil {
			data["Title"] = "Edit Guru"
			data["Action"] = "/master/guru/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = g
		}
	}
	c.HTML(http.StatusOK, "master/guru_form", data)
}

func (h *MasterHandler) guruRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/guru_rows_oob", gin.H{"List": h.guruList()})
}

func (h *MasterHandler) guruFormErr(c *gin.Context, g models.Guru, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/guru_form", gin.H{
		"Title": map[bool]string{true: "Edit Guru", false: "Tambah Guru"}[edit],
		"Action": action, "Edit": edit, "Item": g, "Error": msg,
	})
}

func (h *MasterHandler) bindGuru(c *gin.Context) models.Guru {
	return models.Guru{
		Nama:  strings.TrimSpace(c.PostForm("nama")),
		Aktif: c.PostForm("aktif") == "on",
	}
}

func (h *MasterHandler) GuruCreate(c *gin.Context) {
	g := h.bindGuru(c)
	if g.Nama == "" {
		h.guruFormErr(c, g, false, "/master/guru", "Nama guru wajib diisi.")
		return
	}
	if err := h.DB.Create(&g).Error; err != nil {
		h.guruFormErr(c, g, false, "/master/guru", "Gagal menyimpan data.")
		return
	}
	h.guruRefresh(c)
}

func (h *MasterHandler) GuruUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var g models.Guru
	if err := h.DB.First(&g, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/guru/" + c.Param("id")
	in := h.bindGuru(c)
	g.Nama, g.Aktif = in.Nama, in.Aktif
	if g.Nama == "" {
		h.guruFormErr(c, g, true, action, "Nama guru wajib diisi.")
		return
	}
	if err := h.DB.Save(&g).Error; err != nil {
		h.guruFormErr(c, g, true, action, "Gagal menyimpan data.")
		return
	}
	h.guruRefresh(c)
}

func (h *MasterHandler) GuruDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var pakai int64
	h.DB.Model(&models.GajiGuru{}).Where("guru_id = ?", id).Count(&pakai)
	if pakai > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dihapus: guru sudah punya catatan gaji."}}`)
		h.guruRefresh(c)
		return
	}
	h.DB.Delete(&models.Guru{}, id)
	h.guruRefresh(c)
}

// =====================================================================
// JENIS TAGIHAN
// =====================================================================

func (h *MasterHandler) jenisTagihanList() []models.JenisTagihan {
	var list []models.JenisTagihan
	h.DB.Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) JenisTagihanIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/jenis_tagihan", gin.H{
		"Title":      "Master · Jenis Tagihan",
		"ActiveMenu": "master",
		"ActiveTab":  "jenis-tagihan",
		"User":       currentUser(c),
		"List":       h.jenisTagihanList(),
	})
}

func (h *MasterHandler) JenisTagihanForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Jenis Tagihan", "Action": "/master/jenis-tagihan", "Edit": false, "Item": models.JenisTagihan{Aktif: true}}
	if id, ok := parseID(c); ok {
		var j models.JenisTagihan
		if err := h.DB.First(&j, id).Error; err == nil {
			data["Title"] = "Edit Jenis Tagihan"
			data["Action"] = "/master/jenis-tagihan/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = j
		}
	}
	c.HTML(http.StatusOK, "master/jenis_tagihan_form", data)
}

func (h *MasterHandler) jenisTagihanRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/jenis_tagihan_rows_oob", gin.H{"List": h.jenisTagihanList()})
}

func (h *MasterHandler) jenisTagihanFormErr(c *gin.Context, j models.JenisTagihan, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/jenis_tagihan_form", gin.H{
		"Title":  map[bool]string{true: "Edit Jenis Tagihan", false: "Tambah Jenis Tagihan"}[edit],
		"Action": action, "Edit": edit, "Item": j, "Error": msg,
	})
}

func (h *MasterHandler) bindJenisTagihan(c *gin.Context) models.JenisTagihan {
	return models.JenisTagihan{
		Nama:       strings.TrimSpace(c.PostForm("nama")),
		Keterangan: strings.TrimSpace(c.PostForm("keterangan")),
		Aktif:      c.PostForm("aktif") == "on",
	}
}

func (h *MasterHandler) JenisTagihanCreate(c *gin.Context) {
	j := h.bindJenisTagihan(c)
	if j.Nama == "" {
		h.jenisTagihanFormErr(c, j, false, "/master/jenis-tagihan", "Nama wajib diisi.")
		return
	}
	if err := h.DB.Create(&j).Error; err != nil {
		h.jenisTagihanFormErr(c, j, false, "/master/jenis-tagihan", "Gagal menyimpan data.")
		return
	}
	h.jenisTagihanRefresh(c)
}

func (h *MasterHandler) JenisTagihanUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var j models.JenisTagihan
	if err := h.DB.First(&j, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/jenis-tagihan/" + c.Param("id")
	in := h.bindJenisTagihan(c)
	j.Nama, j.Keterangan, j.Aktif = in.Nama, in.Keterangan, in.Aktif
	if j.Nama == "" {
		h.jenisTagihanFormErr(c, j, true, action, "Nama wajib diisi.")
		return
	}
	if err := h.DB.Save(&j).Error; err != nil {
		h.jenisTagihanFormErr(c, j, true, action, "Gagal menyimpan data.")
		return
	}
	h.jenisTagihanRefresh(c)
}

func (h *MasterHandler) JenisTagihanDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var pakai int64
	h.DB.Model(&models.Tagihan{}).Where("jenis_tagihan_id = ?", id).Count(&pakai)
	if pakai > 0 {
		c.Header("HX-Trigger", `{"toast":{"type":"error","msg":"Tidak bisa dihapus: jenis ini sudah dipakai pada tagihan."}}`)
		h.jenisTagihanRefresh(c)
		return
	}
	h.DB.Delete(&models.JenisTagihan{}, id)
	h.jenisTagihanRefresh(c)
}

// =====================================================================
// KATEGORI + SUB KATEGORI PENGELUARAN
// =====================================================================

func (h *MasterHandler) kategoriList() []models.KategoriPengeluaran {
	var list []models.KategoriPengeluaran
	h.DB.Preload("SubList", func(db *gorm.DB) *gorm.DB {
		return db.Order("nama asc")
	}).Order("nama asc").Find(&list)
	return list
}

func (h *MasterHandler) KategoriIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "master/kategori", gin.H{
		"Title":      "Master · Kategori Pengeluaran",
		"ActiveMenu": "master",
		"ActiveTab":  "kategori",
		"User":       currentUser(c),
		"List":       h.kategoriList(),
	})
}

func (h *MasterHandler) kategoriRefresh(c *gin.Context) {
	c.HTML(http.StatusOK, "master/kategori_list_oob", gin.H{"List": h.kategoriList()})
}

// --- Kategori ---

func (h *MasterHandler) KategoriForm(c *gin.Context) {
	data := gin.H{"Title": "Tambah Kategori", "Action": "/master/kategori", "Edit": false, "Item": models.KategoriPengeluaran{Aktif: true}}
	if id, ok := parseID(c); ok {
		var k models.KategoriPengeluaran
		if err := h.DB.First(&k, id).Error; err == nil {
			data["Title"] = "Edit Kategori"
			data["Action"] = "/master/kategori/" + c.Param("id")
			data["Edit"] = true
			data["Item"] = k
		}
	}
	c.HTML(http.StatusOK, "master/kategori_form", data)
}

func (h *MasterHandler) kategoriFormErr(c *gin.Context, k models.KategoriPengeluaran, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/kategori_form", gin.H{
		"Title":  map[bool]string{true: "Edit Kategori", false: "Tambah Kategori"}[edit],
		"Action": action, "Edit": edit, "Item": k, "Error": msg,
	})
}

func (h *MasterHandler) KategoriCreate(c *gin.Context) {
	k := models.KategoriPengeluaran{Nama: strings.TrimSpace(c.PostForm("nama")), Aktif: c.PostForm("aktif") == "on"}
	if k.Nama == "" {
		h.kategoriFormErr(c, k, false, "/master/kategori", "Nama kategori wajib diisi.")
		return
	}
	if err := h.DB.Create(&k).Error; err != nil {
		h.kategoriFormErr(c, k, false, "/master/kategori", "Gagal menyimpan data.")
		return
	}
	h.kategoriRefresh(c)
}

func (h *MasterHandler) KategoriUpdate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	var k models.KategoriPengeluaran
	if err := h.DB.First(&k, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/kategori/" + c.Param("id")
	k.Nama = strings.TrimSpace(c.PostForm("nama"))
	k.Aktif = c.PostForm("aktif") == "on"
	if k.Nama == "" {
		h.kategoriFormErr(c, k, true, action, "Nama kategori wajib diisi.")
		return
	}
	if err := h.DB.Save(&k).Error; err != nil {
		h.kategoriFormErr(c, k, true, action, "Gagal menyimpan data.")
		return
	}
	h.kategoriRefresh(c)
}

func (h *MasterHandler) KategoriDelete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	h.DB.Where("kategori_id = ?", id).Delete(&models.SubKategoriPengeluaran{})
	h.DB.Delete(&models.KategoriPengeluaran{}, id)
	h.kategoriRefresh(c)
}

// --- Sub Kategori ---

func (h *MasterHandler) SubForm(c *gin.Context) {
	katID, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	data := gin.H{
		"Title": "Tambah Sub Kategori", "Action": "/master/kategori/" + c.Param("id") + "/sub",
		"Edit": false, "Item": models.SubKategoriPengeluaran{KategoriID: katID},
	}
	if subIDStr := c.Param("subID"); subIDStr != "" {
		var sub models.SubKategoriPengeluaran
		if err := h.DB.First(&sub, subIDStr).Error; err == nil {
			data["Title"] = "Edit Sub Kategori"
			data["Action"] = "/master/kategori/" + c.Param("id") + "/sub/" + subIDStr
			data["Edit"] = true
			data["Item"] = sub
		}
	}
	c.HTML(http.StatusOK, "master/sub_form", data)
}

func (h *MasterHandler) subFormErr(c *gin.Context, sub models.SubKategoriPengeluaran, edit bool, action, msg string) {
	c.HTML(http.StatusOK, "master/sub_form", gin.H{
		"Title":  map[bool]string{true: "Edit Sub Kategori", false: "Tambah Sub Kategori"}[edit],
		"Action": action, "Edit": edit, "Item": sub, "Error": msg,
	})
}

func (h *MasterHandler) SubCreate(c *gin.Context) {
	katID, ok := parseID(c)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	sub := models.SubKategoriPengeluaran{KategoriID: katID, Nama: strings.TrimSpace(c.PostForm("nama")), Aktif: c.PostForm("aktif") == "on"}
	action := "/master/kategori/" + c.Param("id") + "/sub"
	if sub.Nama == "" {
		h.subFormErr(c, sub, false, action, "Nama sub kategori wajib diisi.")
		return
	}
	if err := h.DB.Create(&sub).Error; err != nil {
		h.subFormErr(c, sub, false, action, "Gagal menyimpan data.")
		return
	}
	h.kategoriRefresh(c)
}

func (h *MasterHandler) SubUpdate(c *gin.Context) {
	var sub models.SubKategoriPengeluaran
	if err := h.DB.First(&sub, c.Param("subID")).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	action := "/master/kategori/" + c.Param("id") + "/sub/" + c.Param("subID")
	sub.Nama = strings.TrimSpace(c.PostForm("nama"))
	sub.Aktif = c.PostForm("aktif") == "on"
	if sub.Nama == "" {
		h.subFormErr(c, sub, true, action, "Nama sub kategori wajib diisi.")
		return
	}
	if err := h.DB.Save(&sub).Error; err != nil {
		h.subFormErr(c, sub, true, action, "Gagal menyimpan data.")
		return
	}
	h.kategoriRefresh(c)
}

func (h *MasterHandler) SubDelete(c *gin.Context) {
	h.DB.Delete(&models.SubKategoriPengeluaran{}, c.Param("subID"))
	h.kategoriRefresh(c)
}
