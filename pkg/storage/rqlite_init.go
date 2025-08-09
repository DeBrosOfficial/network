package storage

import (
	"fmt"
)

// initTables creates the necessary tables for key-value storage
func (s *Service) initTables() error {
	// Create storage table with namespace support
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS kv_storage (
			namespace TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (namespace, key)
		)
	`

	// Create index for faster queries
	createIndexSQL := `
		CREATE INDEX IF NOT EXISTS idx_kv_storage_namespace_key 
		ON kv_storage(namespace, key)
	`

	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create storage table: %w", err)
	}

	if _, err := s.db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("failed to create storage index: %w", err)
	}

	s.logger.Info("Storage tables initialized successfully")
	return nil
}
