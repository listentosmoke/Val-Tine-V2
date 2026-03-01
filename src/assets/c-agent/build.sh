#!/bin/bash
# NodePulse Agent Builder Script
set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║           NodePulse C Agent Builder                        ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

cd "$(dirname "$0")"

# Build available targets
echo "Building agent stubs..."
make all

echo ""
echo "=== SHA256 Checksums ==="
make checksums

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✅ Done! Upload stubs/* to the agent-stubs storage bucket."
echo "════════════════════════════════════════════════════════════"
