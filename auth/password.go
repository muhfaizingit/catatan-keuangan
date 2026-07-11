package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword menghasilkan hash bcrypt dari password plain.
func HashPassword(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword membandingkan password plain dengan hash tersimpan.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
