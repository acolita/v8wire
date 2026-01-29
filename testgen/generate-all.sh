#!/bin/bash
# Generate V8 serialization fixtures for all supported Node.js versions
# This ensures backwards compatibility across V8 wire format versions 13-15

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/../testdata/fixtures"

echo "========================================"
echo "V8 Multi-Version Fixture Generator"
echo "========================================"
echo ""

# Create version-specific output directories
mkdir -p "$TESTDATA_DIR/v13"
mkdir -p "$TESTDATA_DIR/v14"
mkdir -p "$TESTDATA_DIR/v15"

# Change to testgen directory for docker-compose
cd "$SCRIPT_DIR"

echo "Building Docker images..."
docker compose build

echo ""
echo "--- Generating v13 fixtures (Node.js 18.x) ---"
docker compose run --rm node18

echo ""
echo "--- Generating v14 fixtures (Node.js 20.x) ---"
docker compose run --rm node20

echo ""
echo "--- Generating v15 fixtures (Node.js 22.x) ---"
docker compose run --rm node22

echo ""
echo "========================================"
echo "Fixture generation complete!"
echo ""
echo "Generated fixtures:"
echo "  - $TESTDATA_DIR/v13/ (Node.js 18.x, V8 format v13)"
echo "  - $TESTDATA_DIR/v14/ (Node.js 20.x, V8 format v14)"
echo "  - $TESTDATA_DIR/v15/ (Node.js 22.x, V8 format v15)"
echo ""

# Count fixtures per version
for version in v13 v14 v15; do
  count=$(find "$TESTDATA_DIR/$version" -name "*.bin" 2>/dev/null | wc -l)
  echo "  $version: $count fixtures"
done

echo ""
echo "Run 'go test -v ./...' to verify compatibility."
