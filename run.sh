#!/usr/bin/env bash
# One-command full stack for NixOS: Database -> Backend -> Frontend.
# Run inside nix develop:  nix develop -c ./run.sh   (flake cung cấp Go 1.27, templ, tailwindcss, air)
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
# Brings up Postgres (docker), waits until healthy, starts the backend in the
# background, waits until :5000 answers, then runs the frontend in the
# foreground. Ctrl+C stops the frontend and tears down the backend.
set -uo pipefail
cd "$(dirname "$0")"

BACKEND_PID=""
cleanup() {
  echo ""
  echo "==> Stopping backend..."
  [ -n "$BACKEND_PID" ] && kill "$BACKEND_PID" 2>/dev/null
  wait "$BACKEND_PID" 2>/dev/null
  echo "==> Done. (Postgres van chay; dung 'docker compose down' de tat)"
}
trap cleanup INT TERM EXIT

echo "==> [1/3] Database (Postgres 16)..."
docker compose up -d || { echo "Docker daemon chua bat? Xem README muc NixOS."; exit 1; }

echo "==> [2/3] Doi Postgres healthy..."
until docker compose exec -T postgres pg_isready -U admin -d kernelstore >/dev/null 2>&1; do
  sleep 1
done
echo "    Postgres OK."

echo "==> Backend -> http://localhost:5000 (log: /tmp/kernelstore-backend.log)"
(cd backend && go build -o /tmp/kernelstore-api ./cmd/api && exec /tmp/kernelstore-api) \
  >/tmp/kernelstore-backend.log 2>&1 &
BACKEND_PID=$!

echo "==> Doi backend san sang..."
for _ in $(seq 1 60); do
  [ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:5000/api/categories 2>/dev/null)" = "200" ] && break
  sleep 2
done
echo "    Backend OK."

echo "==> [3/3] Frontend -> http://localhost:8080 (Ctrl+C de dung tat ca)"
cd frontend && air   # templ generate + tailwind + build, tự reload khi sửa code
