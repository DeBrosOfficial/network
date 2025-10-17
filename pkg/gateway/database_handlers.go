package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DeBrosOfficial/network/pkg/logging"
	"go.uber.org/zap"
)

// Database request/response types

type ExecRequest struct {
	Database string        `json:"database"`
	SQL      string        `json:"sql"`
	Args     []interface{} `json:"args,omitempty"`
}

type ExecResponse struct {
	RowsAffected int64  `json:"rows_affected"`
	LastInsertID int64  `json:"last_insert_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

type QueryRequest struct {
	Database string        `json:"database"`
	SQL      string        `json:"sql"`
	Args     []interface{} `json:"args,omitempty"`
}

type QueryResponse struct {
	Items []map[string]interface{} `json:"items"`
	Count int                      `json:"count"`
	Error string                   `json:"error,omitempty"`
}

type TransactionRequest struct {
	Database string   `json:"database"`
	Queries  []string `json:"queries"`
}

type TransactionResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type CreateTableRequest struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

type DropTableRequest struct {
	Database  string `json:"database"`
	TableName string `json:"table_name"`
}

type SchemaResponse struct {
	Tables []TableSchema `json:"tables"`
	Error  string        `json:"error,omitempty"`
}

type TableSchema struct {
	Name      string   `json:"name"`
	CreateSQL string   `json:"create_sql"`
	Columns   []string `json:"columns,omitempty"`
}

// Database handlers

// databaseExecHandler handles SQL execution (INSERT, UPDATE, DELETE, DDL)
func (g *Gateway) databaseExecHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "Invalid request body"})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "database field is required"})
		return
	}

	if req.SQL == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "sql field is required"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(req.Database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// For simplicity, we'll use Query and check if it's a write operation
	// In production, you'd want to detect write vs read and route accordingly
	result, err := db.Query(ctx, req.SQL, req.Args...)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Query execution failed",
			zap.String("database", req.Database),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: err.Error()})
		return
	}

	// For exec operations, return affected rows
	g.respondJSON(w, http.StatusOK, ExecResponse{
		RowsAffected: result.Count,
	})
}

// databaseQueryHandler handles SELECT queries
func (g *Gateway) databaseQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, QueryResponse{Error: "Invalid request body"})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, QueryResponse{Error: "database field is required"})
		return
	}

	if req.SQL == "" {
		g.respondJSON(w, http.StatusBadRequest, QueryResponse{Error: "sql field is required"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(req.Database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, QueryResponse{Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := db.Query(ctx, req.SQL, req.Args...)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Query execution failed",
			zap.String("database", req.Database),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, QueryResponse{Error: err.Error()})
		return
	}

	// Convert result to map format
	items := make([]map[string]interface{}, len(result.Rows))
	for i, row := range result.Rows {
		item := make(map[string]interface{})
		for j, col := range result.Columns {
			if j < len(row) {
				item[col] = row[j]
			}
		}
		items[i] = item
	}

	g.respondJSON(w, http.StatusOK, QueryResponse{
		Items: items,
		Count: len(items),
	})
}

// databaseTransactionHandler handles atomic transactions
func (g *Gateway) databaseTransactionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, TransactionResponse{Success: false, Error: "Invalid request body"})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, TransactionResponse{Success: false, Error: "database field is required"})
		return
	}

	if len(req.Queries) == 0 {
		g.respondJSON(w, http.StatusBadRequest, TransactionResponse{Success: false, Error: "queries field is required and must not be empty"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(req.Database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, TransactionResponse{Success: false, Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	err = db.Transaction(ctx, req.Queries)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Transaction failed",
			zap.String("database", req.Database),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, TransactionResponse{Success: false, Error: err.Error()})
		return
	}

	g.respondJSON(w, http.StatusOK, TransactionResponse{Success: true})
}

// databaseSchemaHandler returns database schema information
func (g *Gateway) databaseSchemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Support both GET with query param and POST with JSON body
	var database string
	if r.Method == http.MethodPost {
		var req struct {
			Database string `json:"database"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			g.respondJSON(w, http.StatusBadRequest, SchemaResponse{Error: "Invalid request body"})
			return
		}
		database = req.Database
	} else {
		database = r.URL.Query().Get("database")
	}

	if database == "" {
		g.respondJSON(w, http.StatusBadRequest, SchemaResponse{Error: "database parameter is required"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, SchemaResponse{Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	schemaInfo, err := db.GetSchema(ctx)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get schema",
			zap.String("database", database),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, SchemaResponse{Error: err.Error()})
		return
	}

	// Convert to response format
	tables := make([]TableSchema, len(schemaInfo.Tables))
	for i, table := range schemaInfo.Tables {
		columns := make([]string, len(table.Columns))
		for j, col := range table.Columns {
			columns[j] = col.Name
		}
		tables[i] = TableSchema{
			Name:    table.Name,
			Columns: columns,
		}
	}

	g.respondJSON(w, http.StatusOK, SchemaResponse{Tables: tables})
}

// databaseCreateTableHandler creates a new table
func (g *Gateway) databaseCreateTableHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "Invalid request body"})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "database field is required"})
		return
	}

	if req.Schema == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "schema field is required"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(req.Database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err = db.CreateTable(ctx, req.Schema)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to create table",
			zap.String("database", req.Database),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: err.Error()})
		return
	}

	g.respondJSON(w, http.StatusOK, ExecResponse{RowsAffected: 0})
}

// databaseDropTableHandler drops a table
func (g *Gateway) databaseDropTableHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DropTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "Invalid request body"})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "database field is required"})
		return
	}

	if req.TableName == "" {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "table_name field is required"})
		return
	}

	// Validate table name (basic SQL injection prevention)
	if !isValidIdentifier(req.TableName) {
		g.respondJSON(w, http.StatusBadRequest, ExecResponse{Error: "invalid table name"})
		return
	}

	// Get database client
	db, err := g.client.Database().Database(req.Database)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to get database client", zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: fmt.Sprintf("Failed to access database: %v", err)})
		return
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err = db.DropTable(ctx, req.TableName)
	if err != nil {
		g.logger.ComponentError(logging.ComponentDatabase, "Failed to drop table",
			zap.String("database", req.Database),
			zap.String("table", req.TableName),
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, ExecResponse{Error: err.Error()})
		return
	}

	g.respondJSON(w, http.StatusOK, ExecResponse{RowsAffected: 0})
}

// databaseListHandler lists all available databases for the current app
func (g *Gateway) databaseListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: This would require the ClusterManager to expose a list of databases
	// For now, return a placeholder
	g.respondJSON(w, http.StatusOK, map[string]interface{}{
		"databases": []string{},
		"message":   "Database listing not yet implemented - query metadata store directly",
	})
}

// Helper functions

func (g *Gateway) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "Failed to encode JSON response", zap.Error(err))
	}
}

func isValidIdentifier(name string) bool {
	if len(name) == 0 || len(name) > 128 {
		return false
	}
	// Only allow alphanumeric, underscore, and hyphen
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return false
		}
	}
	// Don't start with number
	firstRune := []rune(name)[0]
	if firstRune >= '0' && firstRune <= '9' {
		return false
	}
	// Avoid SQL keywords
	upperName := strings.ToUpper(name)
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TABLE", "DATABASE", "INDEX"}
	for _, keyword := range sqlKeywords {
		if upperName == keyword {
			return false
		}
	}
	return true
}
