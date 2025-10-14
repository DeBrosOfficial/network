#!/bin/bash

# Test 3: Query Data
# This test queries data from the users table

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 3: Query Data"
echo "========================================="
echo ""
echo "Querying all users from 'testdb'..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "SELECT * FROM users ORDER BY id"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Querying specific user (Alice)..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "SELECT * FROM users WHERE name = ?",
    "args": ["Alice"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Counting users..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "SELECT COUNT(*) as count FROM users"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 3 Complete"
echo "========================================="

