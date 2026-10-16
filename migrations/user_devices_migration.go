package migrations

import (
	"database/sql"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// MigrationReport holds the outcome of a migration check or execution.
type MigrationReport struct {
	TotalDevices    int64
	ActiveDevices   int64
	MaxTokenLength  int64
	OversizedCount  int64
	DuplicateCount  int64
	ActionTaken     string
	IndexesCreated  []string
	IndexesExisting []string
}

// PreflightCheckUserDevices verifies that production data in user_devices is safe
// for column length optimization to VARCHAR(255) and unique index consolidation.
func PreflightCheckUserDevices(db *gorm.DB) (*MigrationReport, error) {
	report := &MigrationReport{}

	// Determine length function compatible with both MySQL and SQLite test runner
	lenFunc := "CHAR_LENGTH"
	if db.Dialector.Name() == "sqlite" {
		lenFunc = "LENGTH"
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_count,
			COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) as active_count,
			COALESCE(MAX(%s(fcm_token)), 0) as max_len,
			COUNT(CASE WHEN %s(fcm_token) > 255 THEN 1 END) as over255_count
		FROM user_devices
	`, lenFunc, lenFunc)

	row := db.Raw(query).Row()
	if err := row.Scan(&report.TotalDevices, &report.ActiveDevices, &report.MaxTokenLength, &report.OversizedCount); err != nil {
		return nil, fmt.Errorf("preflight check failed reading user_devices metrics: %w", err)
	}

	// Check for duplicate fcm_token values
	rowDup := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT fcm_token FROM user_devices GROUP BY fcm_token HAVING COUNT(*) > 1
		) d
	`).Row()

	if err := rowDup.Scan(&report.DuplicateCount); err != nil {
		return nil, fmt.Errorf("preflight check failed checking duplicate tokens: %w", err)
	}

	return report, nil
}

// MigrateUserDevicesSchema executes the safe, reversible schema optimization:
// 1. Validates preconditions (no tokens > 255 chars, no duplicate tokens).
// 2. Alters fcm_token to VARCHAR(255) NOT NULL.
// 3. Adds composite index idx_user_devices_user_deleted (user_id, deleted_at) if not present.
// 4. Adds composite index idx_user_devices_mobile_deleted (mobile, deleted_at) if not present.
func MigrateUserDevicesSchema(db *gorm.DB, apply bool) (*MigrationReport, error) {
	report, err := PreflightCheckUserDevices(db)
	if err != nil {
		return nil, err
	}

	if report.OversizedCount > 0 {
		return report, fmt.Errorf("preflight safety error: found %d token(s) exceeding 255 characters (max: %d)",
			report.OversizedCount, report.MaxTokenLength)
	}

	if report.DuplicateCount > 0 {
		return report, fmt.Errorf("preflight safety error: found %d duplicate token value(s)", report.DuplicateCount)
	}

	if !apply {
		report.ActionTaken = "DRY RUN: Validation passed. Database is safe for VARCHAR(255) and composite indexes."
		return report, nil
	}

	// Inspect existing indexes to avoid duplicate index creation
	type IndexInfo struct {
		KeyName string
	}
	var existingIndexes []IndexInfo
	if err := db.Raw("SHOW INDEXES FROM user_devices").Scan(&existingIndexes).Error; err != nil {
		return report, fmt.Errorf("failed inspecting existing indexes: %w", err)
	}

	hasIndex := func(name string) bool {
		for _, idx := range existingIndexes {
			if idx.KeyName == name {
				return true
			}
		}
		return false
	}

	// 1. Optimize column size: VARCHAR(255) NOT NULL
	var currentMaxLen sql.NullInt64
	db.Raw(`
		SELECT CHARACTER_MAXIMUM_LENGTH 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_devices' AND COLUMN_NAME = 'fcm_token'
	`).Scan(&currentMaxLen)

	if currentMaxLen.Valid && currentMaxLen.Int64 > 255 {
		log.Printf("[MIGRATION] Modifying user_devices.fcm_token from VARCHAR(%d) to VARCHAR(255)...", currentMaxLen.Int64)
		if err := db.Exec("ALTER TABLE user_devices MODIFY fcm_token VARCHAR(255) NOT NULL").Error; err != nil {
			return report, fmt.Errorf("failed altering fcm_token column to VARCHAR(255): %w", err)
		}
		report.ActionTaken = "Modified fcm_token column to VARCHAR(255) NOT NULL."
	} else {
		report.ActionTaken = "fcm_token column is already VARCHAR(255) or smaller."
	}

	// 2. Add composite index (user_id, deleted_at)
	const idxUserDeleted = "idx_user_devices_user_deleted"
	if !hasIndex(idxUserDeleted) {
		log.Printf("[MIGRATION] Creating composite index %s (user_id, deleted_at)...", idxUserDeleted)
		if err := db.Exec(fmt.Sprintf("CREATE INDEX %s ON user_devices (user_id, deleted_at)", idxUserDeleted)).Error; err != nil {
			return report, fmt.Errorf("failed creating index %s: %w", idxUserDeleted, err)
		}
		report.IndexesCreated = append(report.IndexesCreated, idxUserDeleted)
	} else {
		report.IndexesExisting = append(report.IndexesExisting, idxUserDeleted)
	}

	// 3. Add composite index (mobile, deleted_at)
	const idxMobileDeleted = "idx_user_devices_mobile_deleted"
	if !hasIndex(idxMobileDeleted) {
		log.Printf("[MIGRATION] Creating composite index %s (mobile, deleted_at)...", idxMobileDeleted)
		if err := db.Exec(fmt.Sprintf("CREATE INDEX %s ON user_devices (mobile, deleted_at)", idxMobileDeleted)).Error; err != nil {
			return report, fmt.Errorf("failed creating index %s: %w", idxMobileDeleted, err)
		}
		report.IndexesCreated = append(report.IndexesCreated, idxMobileDeleted)
	} else {
		report.IndexesExisting = append(report.IndexesExisting, idxMobileDeleted)
	}

	return report, nil
}

// RollbackUserDevicesSchema rolls back the composite indexes and reverts fcm_token to VARCHAR(500).
func RollbackUserDevicesSchema(db *gorm.DB) error {
	log.Println("[MIGRATION ROLLBACK] Dropping composite indexes if present...")
	_ = db.Exec("DROP INDEX idx_user_devices_user_deleted ON user_devices").Error
	_ = db.Exec("DROP INDEX idx_user_devices_mobile_deleted ON user_devices").Error

	log.Println("[MIGRATION ROLLBACK] Reverting user_devices.fcm_token to VARCHAR(500)...")
	return db.Exec("ALTER TABLE user_devices MODIFY fcm_token VARCHAR(500) NOT NULL").Error
}
