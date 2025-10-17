#!/bin/bash

# Test 9: Stress Test - Create Many Databases
# This test creates multiple databases to test capacity and distribution

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 9: Stress Test - Multiple Databases"
echo "========================================="
echo ""
echo "Creating 10 databases with data..."
echo ""

for i in {1..10}; do
  DB_NAME="stress_test_db_${i}"
  echo "Creating ${DB_NAME}..."
  
  # Create table
  RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/create-table" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{
      \"database\": \"${DB_NAME}\",
      \"schema\": \"CREATE TABLE data (id INTEGER PRIMARY KEY, value TEXT, db_number INTEGER)\"
    }" \
    -w "\nHTTP_STATUS:%{http_code}" \
    -s)
  HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
  JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
  echo "$JSON_RESPONSE" | jq -c '.'
  echo "HTTP Status: $HTTP_STATUS"
  
  # Insert data
  RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/exec" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{
      \"database\": \"${DB_NAME}\",
      \"sql\": \"INSERT INTO data (value, db_number) VALUES (?, ?)\",
      \"args\": [\"Data from database ${i}\", ${i}]
    }" \
    -w "\nHTTP_STATUS:%{http_code}" \
    -s)
  HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
  JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
  echo "$JSON_RESPONSE" | jq -c '.'
  echo "HTTP Status: $HTTP_STATUS"
  
  echo ""
  
  # Small delay to avoid overwhelming the system
  sleep 2
done

echo "========================================="
echo "Verifying all databases..."
echo "========================================="
echo ""

SUCCESS_COUNT=0
FAIL_COUNT=0

for i in {1..10}; do
  DB_NAME="stress_test_db_${i}"
  echo "Querying ${DB_NAME}..."
  
  RESPONSE=$(curl -X POST "${GATEWAY_URL}/v1/database/query" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{
      \"database\": \"${DB_NAME}\",
      \"sql\": \"SELECT * FROM data WHERE db_number = ${i}\"
    }" \
    -s)
  
  if echo "$RESPONSE" | jq -e '.rows | length > 0' > /dev/null 2>&1; then
    echo "✓ ${DB_NAME} OK"
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
  else
    echo "✗ ${DB_NAME} FAILED"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

echo ""
echo "========================================="
echo "Test 9 Complete"
echo "Success: ${SUCCESS_COUNT}/10"
echo "Failed: ${FAIL_COUNT}/10"
echo "========================================="

