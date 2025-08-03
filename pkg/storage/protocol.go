package storage

import (
	"encoding/json"
)

// Storage protocol definitions for distributed storage
const (
	StorageProtocolID = "/network/storage/1.0.0"
)

// Message types for storage operations
type MessageType string

const (
	MessageTypePut    MessageType = "put"
	MessageTypeGet    MessageType = "get"
	MessageTypeDelete MessageType = "delete"
	MessageTypeList   MessageType = "list"
	MessageTypeExists MessageType = "exists"
)

// StorageRequest represents a storage operation request
type StorageRequest struct {
	Type      MessageType `json:"type"`
	Key       string      `json:"key"`
	Value     []byte      `json:"value,omitempty"`
	Prefix    string      `json:"prefix,omitempty"`
	Limit     int         `json:"limit,omitempty"`
	Namespace string      `json:"namespace"`
}

// StorageResponse represents a storage operation response
type StorageResponse struct {
	Success bool     `json:"success"`
	Error   string   `json:"error,omitempty"`
	Value   []byte   `json:"value,omitempty"`
	Keys    []string `json:"keys,omitempty"`
	Exists  bool     `json:"exists,omitempty"`
}

// Marshal serializes a request to JSON
func (r *StorageRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Unmarshal deserializes a request from JSON
func (r *StorageRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, r)
}

// Marshal serializes a response to JSON
func (r *StorageResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Unmarshal deserializes a response from JSON
func (r *StorageResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, r)
}
