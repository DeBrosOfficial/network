#!/bin/bash

# Test 6: Get Schema
# This test retrieves the schema of a database

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:6001"

echo "========================================="
echo "Test 6: Get Schema"
echo "========================================="
echo ""
echo "Getting schema for 'testdb'..."
echo ""

RESPONSE=$(curl -X GET "${GATEWAY_URL}/v1/database/schema?database=testdb" \
  -H "Authorization: Bearer ${API_KEY}" \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
JSON_RESPONSE=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')
echo "$JSON_RESPONSE" | jq '.'
echo "HTTP Status: $HTTP_STATUS"

echo ""
echo "========================================="
echo "Test 6 Complete"
echo "========================================="

