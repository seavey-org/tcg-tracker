package database

import (
	"log"
	"os"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

var DB *gorm.DB

func Initialize(dbPath string) error {
	var err error
	logLevel := logger.Warn
	if v := os.Getenv("GORM_LOG_LEVEL"); v != "" {
		switch strings.ToLower(v) {
		case "silent":
			logLevel = logger.Silent
		case "error":
			logLevel = logger.Error
		case "warn", "warning":
			logLevel = logger.Warn
		case "info":
			logLevel = logger.Info
		}
	}

	dialector := sqlite.Open(dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return err
	}

	log.Println("Database connected successfully")

	// Pre-migration: Clean up any duplicate card_prices before adding unique constraint
	// This handles existing databases that may have duplicates from before the constraint was added
	if err := cleanupDuplicateCardPrices(DB); err != nil {
		log.Printf("Warning: failed to cleanup duplicate card prices: %v", err)
	}

	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&models.Card{},
		&models.CollectionItem{},
		&models.CardPrice{},
		&models.CollectionValueSnapshot{},
	)
	if err != nil {
		return err
	}

	log.Println("Database schema migration completed")

	// Run custom data migrations
	if err := RunMigrations(DB); err != nil {
		log.Printf("Warning: data migrations had issues: %v", err)
	}

	log.Println("Database migration completed")
	return nil
}

func GetDB() *gorm.DB {
	return DB
}
