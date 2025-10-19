#!/usr/bin/env bash
set -euo pipefail

# Simple colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # reset

echo ""
echo -e "${BLUE}🧪 Running all Go tests for ticket-quality-service...${NC}"
echo ""

start_time=$(date +%s)

# --- Unit tests ---
echo -e "${YELLOW}→ Running unit tests...${NC}"
go test ./internal/... -v -count=1
echo -e "${GREEN}✓ Unit tests passed${NC}"
echo ""

# --- Benchmarks ---
echo -e "${YELLOW}→ Running benchmarks (for profiling)...${NC}"
go test ./internal/service -bench=. -benchmem -run=^$ || true
echo -e "${GREEN}✓ Benchmarks completed${NC}"
echo ""

# --- E2E tests ---
if [ "${1:-}" = "--e2e" ] || [ "${1:-}" = "all" ]; then
  echo -e "${YELLOW}→ Running E2E tests (SQLite + gRPC)...${NC}"
  go test ./tests/e2e -v -tags=e2e -count=1
  echo -e "${GREEN}✓ E2E tests passed${NC}"
  echo ""
else
  echo -e "${BLUE}ℹ️  Skipping E2E tests — use './scripts/test.sh --e2e' to include them.${NC}"
  echo ""
fi

# --- Summary ---
end_time=$(date +%s)
elapsed=$(( end_time - start_time ))

echo -e "${GREEN}✅ All tests finished in ${elapsed}s${NC}"
echo ""
