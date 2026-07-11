package config

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDB membuat database bila belum ada lalu membuka koneksi GORM.
func ConnectDB(cfg *Config) (*gorm.DB, error) {
	if err := ensureDatabase(cfg); err != nil {
		return nil, fmt.Errorf("memastikan database ada: %w", err)
	}

	logLevel := logger.Warn
	if cfg.IsProduction() {
		logLevel = logger.Error
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("membuka koneksi gorm: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database gagal: %w", err)
	}

	log.Printf("terhubung ke database '%s' di %s:%s", cfg.DBName, cfg.DBHost, cfg.DBPort)
	return db, nil
}

// ensureDatabase membuat database utama bila belum ada (CREATE DATABASE IF NOT EXISTS).
func ensureDatabase(cfg *Config) error {
	conn, err := sql.Open("mysql", cfg.DSNNoDB())
	if err != nil {
		return err
	}
	defer conn.Close()

	stmt := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		cfg.DBName,
	)
	if _, err := conn.Exec(stmt); err != nil {
		return err
	}
	return nil
}
