# Manual Testing Scripts

This directory contains manual test scripts for testing the dynamic database clustering feature through the Gateway API.

## Prerequisites

1. **Start the Gateway**:
   ```bash
   cd /Users/penguin/dev/debros/network
   make run-gateway
   ```

2. **Start at least 3 nodes** (in separate terminals):
   ```bash
   # Terminal 1
   make run-node

   # Terminal 2
   make run-node2

   # Terminal 3
   make run-node3
   ```

3. **Set your API key**:
   The scripts use the API key: `ak_L1zF6g7Np1dSRyy-zp_cXFfA:default`

## Test Scripts

### Basic Tests
- `01_create_table.sh` - Create a table in a database
- `02_insert_data.sh` - Insert data into a table
- `03_query_data.sh` - Query data from a table
- `04_execute_sql.sh` - Execute arbitrary SQL
- `05_transaction.sh` - Execute a transaction
- `06_get_schema.sh` - Get database schema

### Advanced Tests
- `07_multiple_databases.sh` - Test multiple isolated databases
- `08_hibernation_test.sh` - Test hibernation and wake-up
- `09_stress_test.sh` - Create many databases

### Utility Scripts
- `cleanup.sh` - Clean up test databases
- `run_all_tests.sh` - Run all tests in sequence

## Usage

Make scripts executable:
```bash
chmod +x mantests/*.sh
```

Run individual test:
```bash
./mantests/01_create_table.sh
```

Run all tests:
```bash
./mantests/run_all_tests.sh
```

## Expected Results

All scripts should return HTTP 200 status codes and appropriate JSON responses. Check the output for:
- Success messages
- Returned data matching expectations
- No errors in the JSON responses

## Troubleshooting

If tests fail:
1. Ensure gateway is running on `http://localhost:8080`
2. Ensure at least 3 nodes are running
3. Check that nodes have discovered each other (wait 10 seconds after startup)
4. Verify API key is valid
5. Check gateway and node logs for errors

