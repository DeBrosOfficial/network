#!/bin/bash

# Test 5: Transaction
# This test executes multiple SQL statements in a transaction

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 5: Transaction"
echo "========================================="
echo ""
echo "Executing transaction (insert multiple orders)..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/transaction" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "queries": [
      "CREATE TABLE IF NOT EXISTS orders (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, product_id INTEGER, quantity INTEGER, total REAL)",
      "INSERT INTO orders (user_id, product_id, quantity, total) VALUES (1, 1, 2, 1999.98)",
      "INSERT INTO orders (user_id, product_id, quantity, total) VALUES (2, 2, 5, 149.95)",
      "INSERT INTO orders (user_id, product_id, quantity, total) VALUES (1, 2, 1, 29.99)"
    ]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Querying orders..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "SELECT * FROM orders"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 5 Complete"
echo "========================================="

