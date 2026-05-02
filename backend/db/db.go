package db

import (
	"log"
	"prediplay/backend/models"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init opens the SQLite database at path, configures WAL mode for concurrent access,
// and runs AutoMigrate for all models. It terminates the process on any error.
func Init(path string) *gorm.DB {
	// WAL journal mode allows concurrent reads while a write is in progress.
	// busy_timeout tells SQLite to wait up to 5 s instead of returning SQLITE_BUSY
	// immediately when another connection holds a write lock (e.g. the sync goroutine).
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	// SQLite allows only one writer at a time. A single open connection prevents
	// "database is locked" errors when sync and HTTP handlers write concurrently.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(time.Minute)

	if err := db.AutoMigrate(
		&models.League{},
		&models.Team{},
		&models.Player{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database initialized")
	return db
}
