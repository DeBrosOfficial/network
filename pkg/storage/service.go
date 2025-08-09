package storage

import (
	"database/sql"
	"sync"

	"go.uber.org/zap"
)

// Service provides distributed storage functionality using RQLite
type Service struct {
	logger *zap.Logger
	db     *sql.DB
	mu     sync.RWMutex
}

// NewService creates a new storage service backed by RQLite
func NewService(db *sql.DB, logger *zap.Logger) (*Service, error) {
	service := &Service{
		logger: logger,
		db:     db,
	}

	return service, nil
}

// Close closes the storage service
func (s *Service) Close() error {
	// The database connection is managed elsewhere
	s.logger.Info("Storage service closed")
	return nil
}
