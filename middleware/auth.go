package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"teras-keuangan/auth"
)

// Context keys untuk menyimpan data user hasil autentikasi.
const (
	CtxUserID = "userID"
	CtxNama   = "userNama"
	CtxEmail  = "userEmail"
	CtxRole   = "userRole"
)

// Auth memvalidasi cookie JWT pada setiap request terproteksi.
// Bila gagal, request diarahkan ke halaman login.
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie(auth.CookieName)
		if err != nil || tokenStr == "" {
			redirectToLogin(c)
			return
		}

		claims, err := auth.ParseToken(secret, tokenStr)
		if err != nil {
			// Token kedaluwarsa / rusak: bersihkan cookie lalu ke login.
			c.SetCookie(auth.CookieName, "", -1, "/", "", false, true)
			redirectToLogin(c)
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxNama, claims.Nama)
		c.Set(CtxEmail, claims.Email)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

func redirectToLogin(c *gin.Context) {
	// Permintaan HTMX butuh header khusus agar redirect penuh terjadi.
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/login")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Redirect(http.StatusFound, "/login")
	c.Abort()
}
