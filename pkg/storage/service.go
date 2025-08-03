package storage

import (
	"database/sql"
	"fmt"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
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

	// Initialize storage tables
	if err := service.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage tables: %w", err)
	}

	return service, nil
}

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

// HandleStorageStream handles incoming storage protocol streams
func (s *Service) HandleStorageStream(stream network.Stream) {
	defer stream.Close()

	// Read request
	data, err := io.ReadAll(stream)
	if err != nil {
		s.logger.Error("Failed to read storage request", zap.Error(err))
		return
	}

	var request StorageRequest
	if err := request.Unmarshal(data); err != nil {
		s.logger.Error("Failed to unmarshal storage request", zap.Error(err))
		return
	}

	// Process request
	response := s.processRequest(&request)

	// Send response
	responseData, err := response.Marshal()
	if err != nil {
		s.logger.Error("Failed to marshal storage response", zap.Error(err))
		return
	}

	if _, err := stream.Write(responseData); err != nil {
		s.logger.Error("Failed to write storage response", zap.Error(err))
		return
	}

	s.logger.Debug("Handled storage request",
		zap.String("type", string(request.Type)),
		zap.String("key", request.Key),
		zap.String("namespace", request.Namespace),
		zap.Bool("success", response.Success),
	)
}

// processRequest processes a storage request and returns a response
func (s *Service) processRequest(req *StorageRequest) *StorageResponse {
	switch req.Type {
	case MessageTypePut:
		return s.handlePut(req)
	case MessageTypeGet:
		return s.handleGet(req)
	case MessageTypeDelete:
		return s.handleDelete(req)
	case MessageTypeList:
		return s.handleList(req)
	case MessageTypeExists:
		return s.handleExists(req)
	default:
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown message type: %s", req.Type),
		}
	}
}

// handlePut stores a key-value pair
func (s *Service) handlePut(req *StorageRequest) *StorageResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use REPLACE to handle both insert and update
	query := `
		REPLACE INTO kv_storage (namespace, key, value, updated_at) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err := s.db.Exec(query, req.Namespace, req.Key, req.Value)
	if err != nil {
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to store key: %v", err),
		}
	}

	s.logger.Debug("Stored key", zap.String("key", req.Key), zap.String("namespace", req.Namespace))
	return &StorageResponse{Success: true}
}

// handleGet retrieves a value by key
func (s *Service) handleGet(req *StorageRequest) *StorageResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT value FROM kv_storage WHERE namespace = ? AND key = ?`

	var value []byte
	err := s.db.QueryRow(query, req.Namespace, req.Key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return &StorageResponse{
				Success: false,
				Error:   fmt.Sprintf("key not found: %s", req.Key),
			}
		}
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get key: %v", err),
		}
	}

	return &StorageResponse{
		Success: true,
		Value:   value,
	}
}

// handleDelete removes a key
func (s *Service) handleDelete(req *StorageRequest) *StorageResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM kv_storage WHERE namespace = ? AND key = ?`

	result, err := s.db.Exec(query, req.Namespace, req.Key)
	if err != nil {
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to delete key: %v", err),
		}
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("key not found: %s", req.Key),
		}
	}

	s.logger.Debug("Deleted key", zap.String("key", req.Key), zap.String("namespace", req.Namespace))
	return &StorageResponse{Success: true}
}

// handleList lists keys with a prefix
func (s *Service) handleList(req *StorageRequest) *StorageResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var args []interface{}

	if req.Prefix == "" {
		// List all keys in namespace
		query = `SELECT key FROM kv_storage WHERE namespace = ?`
		args = []interface{}{req.Namespace}
	} else {
		// List keys with prefix
		query = `SELECT key FROM kv_storage WHERE namespace = ? AND key LIKE ?`
		args = []interface{}{req.Namespace, req.Prefix + "%"}
	}

	if req.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, req.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to query keys: %v", err),
		}
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	return &StorageResponse{
		Success: true,
		Keys:    keys,
	}
}

// handleExists checks if a key exists
func (s *Service) handleExists(req *StorageRequest) *StorageResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT 1 FROM kv_storage WHERE namespace = ? AND key = ? LIMIT 1`

	var exists int
	err := s.db.QueryRow(query, req.Namespace, req.Key).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return &StorageResponse{
				Success: true,
				Exists:  false,
			}
		}
		return &StorageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to check key existence: %v", err),
		}
	}

	return &StorageResponse{
		Success: true,
		Exists:  true,
	}
}

// Close closes the storage service
func (s *Service) Close() error {
	// The database connection is managed elsewhere
	s.logger.Info("Storage service closed")
	return nil
}
