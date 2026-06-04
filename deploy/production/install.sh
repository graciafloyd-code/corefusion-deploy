#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/corefusion}"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. Install Docker first, then rerun this script."
  exit 1
fi

mkdir -p "$APP_DIR"
cp -a docker-compose.yml Caddyfile .env mysql-init "$APP_DIR"/

if [ -f image/new-api-corefusion-latest.tar ]; then
  docker load -i image/new-api-corefusion-latest.tar
fi

cd "$APP_DIR"
docker compose up -d
docker compose ps

echo
echo "CoreFusion is starting. DNS must point supchuang.com to this server before HTTPS can issue."
