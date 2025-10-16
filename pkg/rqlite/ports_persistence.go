package rqlite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// SavedPortInfo represents the persisted port allocation for a database on this node
type SavedPortInfo struct {
	HTTPPort int    `json:"http_port"`
	RaftPort int    `json:"raft_port"`
	Host     string `json:"host"`
}

// getPortsFilePath returns the path to the ports.json file for a database
func getPortsFilePath(dataDir, dbName string) string {
	return filepath.Join(dataDir, dbName, "ports.json")
}

// LoadSavedPorts loads previously saved port information for a database
// Returns nil if no saved ports exist or if there's an error reading them
func LoadSavedPorts(dataDir, dbName string, logger *zap.Logger) *SavedPortInfo {
	portsFile := getPortsFilePath(dataDir, dbName)

	data, err := os.ReadFile(portsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read saved ports file",
				zap.String("database", dbName),
				zap.String("file", portsFile),
				zap.Error(err))
		}
		return nil
	}

	var savedPorts SavedPortInfo
	if err := json.Unmarshal(data, &savedPorts); err != nil {
		logger.Warn("Failed to parse saved ports file",
			zap.String("database", dbName),
			zap.String("file", portsFile),
			zap.Error(err))
		return nil
	}

	logger.Info("Loaded saved ports for database",
		zap.String("database", dbName),
		zap.Int("http_port", savedPorts.HTTPPort),
		zap.Int("raft_port", savedPorts.RaftPort),
		zap.String("host", savedPorts.Host))

	return &savedPorts
}

// SavePorts persists port allocation for a database to disk
func SavePorts(dataDir, dbName string, ports PortPair, logger *zap.Logger) error {
	// Create directory if it doesn't exist
	dbDir := filepath.Join(dataDir, dbName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	portsFile := getPortsFilePath(dataDir, dbName)

	savedPorts := SavedPortInfo(ports)

	data, err := json.MarshalIndent(savedPorts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ports: %w", err)
	}

	if err := os.WriteFile(portsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write ports file: %w", err)
	}

	logger.Debug("Saved ports for database",
		zap.String("database", dbName),
		zap.Int("http_port", savedPorts.HTTPPort),
		zap.Int("raft_port", savedPorts.RaftPort),
		zap.String("host", savedPorts.Host),
		zap.String("file", portsFile))

	return nil
}

// DeleteSavedPorts removes the saved ports file for a database
func DeleteSavedPorts(dataDir, dbName string, logger *zap.Logger) error {
	portsFile := getPortsFilePath(dataDir, dbName)

	if err := os.Remove(portsFile); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to delete saved ports file",
			zap.String("database", dbName),
			zap.String("file", portsFile),
			zap.Error(err))
		return err
	}

	logger.Debug("Deleted saved ports file",
		zap.String("database", dbName),
		zap.String("file", portsFile))

	return nil
}
