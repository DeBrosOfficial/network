#!/bin/bash

# Test 1: Create Table
# This test creates a new table in the "testdb" database

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 1: Create Table"
echo "========================================="
echo ""
echo "Creating 'users' table in 'testdb'..."
echo ""

# Make the request and capture both response and status
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/create-table" \
  -H "X-API-Key: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "schema": "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, email TEXT UNIQUE, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)

# Extract HTTP status
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
# Extract JSON response (everything before HTTP_STATUS)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

# Display the JSON response formatted
echo "$JSON_RESPONSE" | jq '.'

# Display HTTP status
echo ""
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 1 Complete"
echo "========================================="

