#!/bin/bash

# Test 4: Execute SQL
# This test executes various SQL operations

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 4: Execute SQL"
echo "========================================="
echo ""
echo "Creating products table..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "CREATE TABLE IF NOT EXISTS products (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, price REAL, stock INTEGER DEFAULT 0)"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Inserting products..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "INSERT INTO products (name, price, stock) VALUES (?, ?, ?)",
    "args": ["Laptop", 999.99, 10]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "INSERT INTO products (name, price, stock) VALUES (?, ?, ?)",
    "args": ["Mouse", 29.99, 50]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Updating product stock..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "UPDATE products SET stock = stock - 1 WHERE name = ?",
    "args": ["Laptop"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Querying products..."
echo ""

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "testdb",
    "sql": "SELECT * FROM products"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 4 Complete"
echo "========================================="

