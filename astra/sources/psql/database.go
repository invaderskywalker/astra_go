package psql

import (
	"astra/astra/config"
	"astra/astra/sources/psql/models"
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	DB *gorm.DB
}

func NewDatabase(ctx context.Context, cfg config.Config) (*Database, error) {
	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
	)

	fmt.Println("Connecting to database:", connStr)

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info), // Enable SQL logging for debugging
	})
	if err != nil {
		return nil, err
	}

	var currentDB string
	_ = db.Raw("SELECT current_database()").Scan(&currentDB).Error
	fmt.Println("Connected to DB:", currentDB)

	// Auto-migrate models (automatic schema creation)
	err = db.WithContext(ctx).
		AutoMigrate(
			&models.User{},
			&models.ChatMessage{},
			&models.LongTermKnowledge{},
			&models.Note{},
			&models.SessionSummary{},
		)
	fmt.Println("err in migrate", err)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate: %w", err)
	}

	return &Database{DB: db}, nil
}

func (db *Database) Close() {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}
