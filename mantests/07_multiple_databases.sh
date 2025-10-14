#!/bin/bash

# Test 7: Multiple Databases
# This test creates and uses multiple isolated databases

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

# Helper function to make curl requests with proper JSON parsing
make_request() {
  local method="$1"
  local url="$2"
  local data="$3"
  
  RESPONSE=$(curl -X "$method" "$url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    ${data:+-d "$data"} \
    -w "\nHTTP_STATUS:%{http_code}" \
    -s)
  
  HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
  JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
  echo "$JSON_RESPONSE" | jq '.'
  echo "HTTP Status: $HTTP_STATUS"
}

echo "========================================="
echo "Test 7: Multiple Databases"
echo "========================================="
echo ""

# Create users database
echo "Creating 'users_db' with users table..."
make_request "POST" "${GATEWAY_URL}/v1/database/create-table" '{
    "database": "users_db",
    "schema": "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"
  }'

echo ""

# Insert into users database
echo "Inserting into users_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/exec" '{
    "database": "users_db",
    "sql": "INSERT INTO users (name) VALUES (?)",
    "args": ["User from users_db"]
  }'

echo ""

# Create products database
echo "Creating 'products_db' with products table..."
make_request "POST" "${GATEWAY_URL}/v1/database/create-table" '{
    "database": "products_db",
    "schema": "CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL)"
  }'

echo ""

# Insert into products database
echo "Inserting into products_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/exec" '{
    "database": "products_db",
    "sql": "INSERT INTO products (name, price) VALUES (?, ?)",
    "args": ["Product from products_db", 99.99]
  }'

echo ""

# Create orders database
echo "Creating 'orders_db' with orders table..."
make_request "POST" "${GATEWAY_URL}/v1/database/create-table" '{
    "database": "orders_db",
    "schema": "CREATE TABLE orders (id INTEGER PRIMARY KEY, order_number TEXT)"
  }'

echo ""

# Insert into orders database
echo "Inserting into orders_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/exec" '{
    "database": "orders_db",
    "sql": "INSERT INTO orders (order_number) VALUES (?)",
    "args": ["ORD-001"]
  }'

echo ""
echo "========================================="
echo "Verifying Data Isolation"
echo "========================================="
echo ""

# Query each database
echo "Querying users_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/query" '{
    "database": "users_db",
    "sql": "SELECT * FROM users"
  }'

echo ""

echo "Querying products_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/query" '{
    "database": "products_db",
    "sql": "SELECT * FROM products"
  }'

echo ""

echo "Querying orders_db..."
make_request "POST" "${GATEWAY_URL}/v1/database/query" '{
    "database": "orders_db",
    "sql": "SELECT * FROM orders"
  }'

echo ""
echo "========================================="
echo "Test 7 Complete"
echo "Expected: Each database contains only its own data"
echo "========================================="

