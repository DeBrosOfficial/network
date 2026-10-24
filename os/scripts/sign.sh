#!/bin/bash
# Sign an OramaOS update with rootwallet.
#
# Usage:
#   ./scripts/sign.sh output/orama-os-1.0.0-amd64
#   ./scripts/sign.sh --message output/orama-os-1.0.0-amd64
#
# The update payload is <prefix>.rootfs.squashfs, the root filesystem the
# agent writes to its standby partition. The signed message is that file's
# SHA-256 in lowercase hex: exactly what the agent verifies, since it checks
# the downloaded payload against VersionInfo.sha256 and then the EVM
# personal_sign signature over that same hex string
# (os/agent/internal/update/verify.go). The signature goes to <prefix>.sig.
#
# --message prints the message that would be signed and signs nothing; the
# agent's tests use it to hold the two sides to one digest.
set -euo pipefail

PRINT_ONLY=0
if [ "${1:-}" = "--message" ]; then
    PRINT_ONLY=1
    shift
fi

PREFIX="${1:-}"
if [ -z "$PREFIX" ]; then
    echo "Usage: $0 [--message] <artifact-prefix>" >&2
    echo "  e.g.: $0 output/orama-os-1.0.0-amd64" >&2
    exit 1
fi

PAYLOAD="${PREFIX}.rootfs.squashfs"
if [ ! -f "$PAYLOAD" ]; then
    echo "Error: update payload not found: $PAYLOAD" >&2
    echo "Run 'make build' first." >&2
    exit 1
fi

MESSAGE=$(sha256sum "$PAYLOAD" | awk '{print $1}')
if [ "$PRINT_ONLY" = 1 ]; then
    echo "$MESSAGE"
    exit 0
fi
echo "Signing the SHA-256 of $PAYLOAD: $MESSAGE"

# Sign with rootwallet (EVM secp256k1 personal_sign)
if ! command -v rw &>/dev/null; then
    echo "Error: 'rw' (rootwallet CLI) not found in PATH" >&2
    exit 1
fi

if ! SIGNATURE=$(rw sign "$MESSAGE" --chain evm 2>&1); then
    echo "Error: rw sign failed: $SIGNATURE" >&2
    exit 1
fi

SIG_FILE="${PREFIX}.sig"
echo "$SIGNATURE" > "$SIG_FILE"
echo "Signature written: $SIG_FILE"

echo "Verifying signature..."
if ! VERIFY=$(rw verify "$MESSAGE" "$SIGNATURE" --chain evm 2>&1); then
    echo "Error: signature verification failed: $VERIFY" >&2
    exit 1
fi

echo "Signature verified successfully."
echo ""
echo "Artifacts:"
echo "  Update payload: $PAYLOAD (sha256 $MESSAGE)"
echo "  Signature:      $SIG_FILE"
