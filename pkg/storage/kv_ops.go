package storage

import (
	"database/sql"
	"fmt"

	"go.uber.org/zap"
)

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
