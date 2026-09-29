#!/usr/bin/env bash
# One-command demo seeding: categories + approved shops + products.
# Requires Postgres up (docker compose up -d). Idempotent.
#   ./seed.sh              (inside nix develop)
#   nix develop -c ./seed.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -euo pipefail
cd "$(dirname "$0")"
cd backend && go run ./cmd/api seed
