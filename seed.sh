#!/usr/bin/env bash
# One-command demo seeding: categories + approved shops + products.
# Requires Postgres up (docker compose up -d). Idempotent.
#   ./seed.sh              (inside nix-shell)
#   nix-shell --run ./seed.sh
set -euo pipefail
cd "$(dirname "$0")"
cd backend-go && go run ./cmd/api seed
