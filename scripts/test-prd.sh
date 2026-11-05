#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Every embedded connection starts an isolated PostgreSQL WASM database and
# applies all migrations. Never reuse DATABASE_URL or contact the live database.
npm ci --prefix scripts/prd-db --ignore-scripts --no-audit --no-fund
export REEF_DATA_TEST_DATABASE_URL='postgres://test@127.0.0.1/test?sslmode=disable'
export REEF_PGLITE_RUNNER="$PWD/scripts/prd-db/server.mjs"
go test ./... -count=1 -timeout=120s
node --test scripts/prd-db/*.test.mjs web/admin/tests/*.test.mjs web/site/map-clusters.test.mjs web/public/reef-check/tests/*.cjs
