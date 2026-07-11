package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CookieName adalah nama cookie tempat token JWT disimpan.
const CookieName = "tk_session"

// TokenTTL menentukan masa berlaku token.
const TokenTTL = 12 * time.Hour

// Claims adalah payload JWT aplikasi.
type Claims struct {
	UserID uint64 `json:"uid"`
	Nama   string `json:"nama"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken membuat token JWT yang ditandatangani.
func GenerateToken(secret string, userID uint64, nama, email, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Nama:   nama,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			Subject:   email,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken memvalidasi token dan mengembalikan claims-nya.
func ParseToken(secret, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode signing tidak valid")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}
