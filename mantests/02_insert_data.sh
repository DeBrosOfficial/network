#!/bin/bash

# Test 2: Insert Data
# This test inserts data into the users table

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 2: Insert Data"
echo "========================================="
echo ""
echo "Inserting users into 'testdb'..."
echo ""

# Insert Alice
echo "Inserting Alice..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "INSERT INTO users (name, email) VALUES (?, ?)",
    "args": ["Alice", "alice@example.com"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""

# Insert Bob
echo "Inserting Bob..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "INSERT INTO users (name, email) VALUES (?, ?)",
    "args": ["Bob", "bob@example.com"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""

# Insert Charlie
echo "Inserting Charlie..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "INSERT INTO users (name, email) VALUES (?, ?)",
    "args": ["Charlie", "charlie@example.com"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 2 Complete"
echo "========================================="

