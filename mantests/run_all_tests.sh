#!/bin/bash

# Run All Tests
# This script runs all manual tests in sequence

echo "========================================="
echo "Running All Manual Tests"
echo "========================================="
echo ""
echo "Prerequisites:"
echo "  - Gateway running on http://localhost:8080"
echo "  - At least 3 nodes running"
echo "  - Nodes have discovered each other"
echo ""
read -p "Press Enter to continue or Ctrl+C to cancel..."
echo ""

# Array of test scripts
TESTS=(
  "01_create_table.sh"
  "02_insert_data.sh"
  "03_query_data.sh"
  "04_execute_sql.sh"
  "05_transaction.sh"
  "06_get_schema.sh"
  "07_multiple_databases.sh"
  "09_stress_test.sh"
)

# Note: Skipping 08_hibernation_test.sh as it requires long wait times

PASSED=0
FAILED=0

for test in "${TESTS[@]}"; do
  echo ""
  echo "========================================="
  echo "Running: $test"
  echo "========================================="
  
  if bash "mantests/$test"; then
    PASSED=$((PASSED + 1))
    echo "✓ $test PASSED"
  else
    FAILED=$((FAILED + 1))
    echo "✗ $test FAILED"
  fi
  
  echo ""
  echo "Waiting 3 seconds before next test..."
  sleep 3
done

echo ""
echo "========================================="
echo "All Tests Complete"
echo "========================================="
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo ""
echo "Note: Test 08 (hibernation) was skipped due to long wait times."
echo "Run it manually if needed: ./mantests/08_hibernation_test.sh"
echo ""
echo "========================================="

