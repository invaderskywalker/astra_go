package actions

import (
	"testing"

	"astra/astra/utils/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- Helpers ---
func setupTestEnv(t *testing.T) *DataActions {
	logging.InitLogger() // ensures AppLogger isn’t nil
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return NewDataActions(db)
}
