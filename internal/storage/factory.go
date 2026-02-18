package storage

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/microsoft/go-mssqldb/azuread"
)

// NewDatabase creates a GORM database connection for any supported driver.
// Supported drivers: sqlite, postgres, mysql, sqlserver.
// Applies per-driver optimizations (WAL for SQLite, connection pooling for others).
func NewDatabase(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch strings.ToLower(driver) {
	case "postgres", "postgresql":
		if dsn == "" {
			return nil, fmt.Errorf("DB_DSN is required for postgres driver")
		}
		dialector = postgres.Open(dsn)

	case "sqlserver", "mssql":
		if dsn == "" {
			return nil, fmt.Errorf("DB_DSN is required for sqlserver driver")
		}
		dialector = sqlserver.Open(dsn)

	case "mysql":
		if dsn == "" {
			dsn = "root:admin@tcp(127.0.0.1:3306)/argus?charset=utf8mb4&parseTime=True&loc=Local"
		}
		dialector = mysql.Open(dsn)

	case "sqlite", "":
		if dsn == "" {
			dsn = "argus.db"
		}
		if driver == "" {
			driver = "sqlite"
			log.Println("DB_DRIVER not set, defaulting to sqlite (argus.db)")
		}
		dialector = sqlite.Open(dsn)

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database (%s): %w", driver, err)
	}

	// SQLite pragmas must be set via Exec (glebarez/sqlite doesn't support _pragma DSN params)
	if strings.ToLower(driver) == "sqlite" || driver == "" {
		db.Exec("PRAGMA journal_mode=WAL")
		db.Exec("PRAGMA busy_timeout=5000")
		db.Exec("PRAGMA synchronous=NORMAL")
	}

	// Configure Connection Pool
	sqlDB, err := db.DB()
	if err == nil {
		switch strings.ToLower(driver) {
		case "sqlite", "":
			sqlDB.SetMaxIdleConns(1)
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetConnMaxLifetime(time.Hour)
			log.Printf("📊 SQLite Optimization: MaxOpen=1, WAL Mode=Enabled")
		default:
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetMaxOpenConns(50)
			sqlDB.SetConnMaxLifetime(time.Hour)
			log.Printf("📊 DB Pool Configured: MaxOpen=50, MaxIdle=10, Driver=%s", driver)
		}
	}

	return db, nil
}

// AutoMigrateModels runs GORM auto-migration for all Argus models.
func AutoMigrateModels(db *gorm.DB, driver string) error {
	// Disable FK checks during migration for MySQL
	if strings.ToLower(driver) == "mysql" {
		db.Exec("SET FOREIGN_KEY_CHECKS = 0")
		log.Println("🔓 Disabled foreign key checks for migration")
	}

	if err := db.AutoMigrate(&Trace{}, &Span{}, &Log{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Drop foreign keys that AutoMigrate may have created (MySQL)
	if strings.ToLower(driver) == "mysql" {
		db.Exec("ALTER TABLE spans DROP FOREIGN KEY fk_traces_spans")
		db.Exec("ALTER TABLE logs DROP FOREIGN KEY fk_traces_logs")
		db.Exec("SET FOREIGN_KEY_CHECKS = 1")
		log.Println("🔓 Dropped FK constraints for async ingestion compatibility")
	}

	return nil
}
