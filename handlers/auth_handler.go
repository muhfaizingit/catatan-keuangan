package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"teras-keuangan/auth"
	"teras-keuangan/config"
	"teras-keuangan/models"
)

// AuthHandler menangani login dan logout.
type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{DB: db, Cfg: cfg}
}

// ShowLogin menampilkan halaman login. Bila sudah login, ke dashboard.
func (h *AuthHandler) ShowLogin(c *gin.Context) {
	if tokenStr, err := c.Cookie(auth.CookieName); err == nil && tokenStr != "" {
		if _, err := auth.ParseToken(h.Cfg.JWTSecret, tokenStr); err == nil {
			c.Redirect(http.StatusFound, "/dashboard")
			return
		}
	}
	c.HTML(http.StatusOK, "auth/login", gin.H{
		"Title": "Masuk",
	})
}

// Login memproses kredensial dan menerbitkan cookie JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	email := strings.TrimSpace(strings.ToLower(c.PostForm("email")))
	password := c.PostForm("password")

	render := func(msg string) {
		c.HTML(http.StatusOK, "auth/login_form", gin.H{
			"Error": msg,
			"Email": email,
		})
	}

	if email == "" || password == "" {
		render("Email dan password wajib diisi.")
		return
	}

	var user models.User
	if err := h.DB.Where("email = ?", email).First(&user).Error; err != nil {
		render("Email atau password salah.")
		return
	}
	if !user.Aktif {
		render("Akun Anda non-aktif. Hubungi administrator.")
		return
	}
	if !auth.CheckPassword(user.Password, password) {
		render("Email atau password salah.")
		return
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, user.ID, user.Nama, user.Email, string(user.Role))
	if err != nil {
		render("Terjadi kesalahan internal. Coba lagi.")
		return
	}

	// httpOnly cookie; secure aktif di production (HTTPS).
	c.SetCookie(
		auth.CookieName,
		token,
		int(auth.TokenTTL.Seconds()),
		"/",
		"",
		h.Cfg.IsProduction(),
		true,
	)

	// HTMX: redirect penuh ke dashboard.
	c.Header("HX-Redirect", "/dashboard")
	c.Status(http.StatusOK)
}

// Logout menghapus cookie sesi.
func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie(auth.CookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
