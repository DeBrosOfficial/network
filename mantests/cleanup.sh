#!/bin/bash

# Cleanup Script
# This script helps clean up test databases
# NOTE: There's no DELETE endpoint yet, so this is informational

API_KEY="ak_L1zF6g7Np1dSRyy-zp_cXFfA:default"
GATEWAY_URL="http://localhost:8080"

echo "========================================="
echo "Cleanup Information"
echo "========================================="
echo ""
echo "Test databases created:"
echo "  - testdb"
echo "  - users_db"
echo "  - products_db"
echo "  - orders_db"
echo "  - hibernate_test_db"
echo "  - stress_test_db_1 through stress_test_db_10"
echo ""
echo "To clean up:"
echo "1. Stop all nodes"
echo "2. Remove data directories:"
echo "   rm -rf data/bootstrap/testapp_*"
echo "   rm -rf data/node/testapp_*"
echo "   rm -rf data/node2/testapp_*"
echo "3. Restart nodes"
echo ""
echo "Or to keep data but test fresh:"
echo "  - Use different database names"
echo "  - Use DROP TABLE statements"
echo ""
echo "========================================="

