#!/usr/bin/env bash
# clear-ports.sh
# Safely terminate any processes listening on specified TCP ports.
# Usage:
#   ./clear-ports.sh                # clears 4001, 5001, 7001 by default
#   ./clear-ports.sh 4001 5001 7001 # clears the specified ports

set -euo pipefail

# Collect ports from args or use defaults
PORTS=("$@")
if [ ${#PORTS[@]} -eq 0 ]; then
  PORTS=(4001 4002 5001 5002 7001 7002)
fi

echo "Gracefully terminating listeners on: ${PORTS[*]}"
for p in "${PORTS[@]}"; do
  PIDS=$(lsof -t -n -P -iTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)
  if [ -n "$PIDS" ]; then
    echo "Port $p -> PIDs: $PIDS (SIGTERM)"
    # shellcheck disable=SC2086
    kill -TERM $PIDS 2>/dev/null || true
  else
    echo "Port $p -> no listeners"
  fi
done

sleep 1

echo "Force killing any remaining listeners..."
for p in "${PORTS[@]}"; do
  PIDS=$(lsof -t -n -P -iTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)
  if [ -n "$PIDS" ]; then
    echo "Port $p -> PIDs: $PIDS (SIGKILL)"
    # shellcheck disable=SC2086
    kill -9 $PIDS 2>/dev/null || true
  else
    echo "Port $p -> none remaining"
  fi
done

echo "\nVerification (should be empty):"
for p in "${PORTS[@]}"; do
  echo "--- Port $p ---"
  lsof -n -P -iTCP:"$p" -sTCP:LISTEN 2>/dev/null || true
  echo
done

