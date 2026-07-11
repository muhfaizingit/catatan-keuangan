package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole membatasi akses hanya untuk role tertentu.
// Harus dipasang setelah middleware Auth.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		roleStr, _ := role.(string)
		if _, ok := allowed[roleStr]; !ok {
			if c.GetHeader("HX-Request") == "true" {
				c.String(http.StatusForbidden, "Akses ditolak")
				c.Abort()
				return
			}
			c.HTML(http.StatusForbidden, "error/403", gin.H{
				"Title": "Akses Ditolak",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
