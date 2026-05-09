package utils

import (
	"log"
	"time"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB
var SessionStore = make(map[string]uint)

func InitDB() error {
	var err error
	DB, err = gorm.Open(mysql.Open(config.AppConfig.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	cfg := config.AppConfig.MySQL
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)
	}

	log.Printf("Database connected successfully (max_open=%d, max_idle=%d)", cfg.MaxOpenConns, cfg.MaxIdleConns)

	if err := AutoMigrate(); err != nil {
		return err
	}

	log.Println("Database migration completed")
	return nil
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Content{},
		&models.Comment{},
		&models.CommentReport{},
		&models.AuditLog{},
	)
}
