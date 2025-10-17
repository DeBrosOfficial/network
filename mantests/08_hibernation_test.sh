#!/bin/bash

# Test 8: Hibernation and Wake-Up
# This test verifies hibernation and wake-up behavior
# NOTE: This test requires nodes to be configured with a short hibernation timeout

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 8: Hibernation and Wake-Up"
echo "========================================="
echo ""
echo "NOTE: This test requires nodes configured with hibernation_timeout"
echo "      Default is 60 seconds. Adjust wait times accordingly."
echo ""

# Create database and insert data
echo "Creating 'hibernate_test_db' and inserting data..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/create-table" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "hibernate_test_db",
    "schema": "CREATE TABLE test_data (id INTEGER PRIMARY KEY, value TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)"
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
    "database": "hibernate_test_db",
    "sql": "INSERT INTO test_data (value) VALUES (?)",
    "args": ["Initial data before hibernation"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "Data inserted. Database is now active."
echo ""
echo "Waiting for hibernation timeout (default: 60 seconds)..."
echo "You can monitor node logs for 'Database is idle' messages."
echo ""

# Wait for hibernation (adjust based on your config)
HIBERNATION_TIMEOUT=70
for i in $(seq $HIBERNATION_TIMEOUT -10 0); do
  echo -ne "Waiting ${i} seconds...\r"
  sleep 10
done

echo ""
echo ""
echo "Hibernation period elapsed. Database should be hibernating now."
echo ""
echo "Attempting to query (this should trigger wake-up)..."
echo ""

# Measure wake-up time
START_TIME=$(date +%s)

RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "hibernate_test_db",
    "sql": "SELECT * FROM test_data"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

END_TIME=$(date +%s)
WAKE_TIME=$((END_TIME - START_TIME))

echo ""
echo "Query completed in ${WAKE_TIME} seconds"
echo "Expected: < 10 seconds for wake-up"
echo ""

# Verify data persisted
echo "Inserting new data after wake-up..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "hibernate_test_db",
    "sql": "INSERT INTO test_data (value) VALUES (?)",
    "args": ["Data after wake-up"]
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""

echo "Querying all data..."
RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "database": "hibernate_test_db",
    "sql": "SELECT * FROM test_data ORDER BY id"
  }' \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 8 Complete"
echo "Expected: Both records present (data persisted through hibernation)"
echo "========================================="

