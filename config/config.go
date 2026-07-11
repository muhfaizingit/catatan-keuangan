package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi aplikasi yang dibaca dari environment.
type Config struct {
	DBHost string
	DBPort string
	DBName string
	DBUser string
	DBPass string

	JWTSecret string
	AppPort   string
	AppEnv    string

	SeedAdminEmail    string
	SeedAdminPassword string
	SeedAdminNama     string
}

// Load membaca file .env (jika ada) lalu mengisi Config dari environment.
func Load() *Config {
	// .env bersifat opsional; di production env bisa diset langsung.
	if err := godotenv.Load(); err != nil {
		log.Println("info: .env tidak ditemukan, memakai environment variable yang ada")
	}

	cfg := &Config{
		DBHost: getEnv("DB_HOST", "localhost"),
		DBPort: getEnv("DB_PORT", "3306"),
		DBName: getEnv("DB_NAME", "school_finance"),
		DBUser: getEnv("DB_USER", "root"),
		DBPass: getEnv("DB_PASS", ""),

		JWTSecret: getEnv("JWT_SECRET", "dev-secret-jangan-dipakai-di-production"),
		AppPort:   getEnv("APP_PORT", "8080"),
		AppEnv:    getEnv("APP_ENV", "development"),

		SeedAdminEmail:    getEnv("SEED_ADMIN_EMAIL", "admin@sekolah.test"),
		SeedAdminPassword: getEnv("SEED_ADMIN_PASSWORD", "admin123"),
		SeedAdminNama:     getEnv("SEED_ADMIN_NAMA", "Administrator"),
	}

	return cfg
}

// DSN membentuk Data Source Name untuk koneksi MySQL.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName,
	)
}

// DSNNoDB sama seperti DSN tapi tanpa nama database; dipakai untuk membuat
// database bila belum ada.
func (c *Config) DSNNoDB() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort,
	)
}

// IsProduction menandakan apakah aplikasi berjalan di mode production.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
