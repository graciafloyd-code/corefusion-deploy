#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/corefusion}"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. Install Docker first, then rerun this script."
  exit 1
fi

mkdir -p "$APP_DIR"
SOURCE_DIR="$(pwd)"
if [ "$SOURCE_DIR" != "$APP_DIR" ]; then
  cp -a docker-compose.yml Caddyfile .env mysql-init "$APP_DIR"/
  if [ -d build ]; then
    rm -rf "$APP_DIR/build"
    cp -a build "$APP_DIR"/
  fi
  if [ -d image ]; then
    rm -rf "$APP_DIR/image"
    cp -a image "$APP_DIR"/
  fi
fi

if [ -f "$APP_DIR/build/corefusion-new-api" ] && [ -f "$APP_DIR/build/Dockerfile" ]; then
  clean_context="$(mktemp -d)"
  cp "$APP_DIR/build/corefusion-new-api" "$clean_context/corefusion-new-api"
  cp "$APP_DIR/build/Dockerfile" "$clean_context/Dockerfile"
  docker build --no-cache -f "$clean_context/Dockerfile" -t new-api-corefusion:latest "$clean_context"
  rm -rf "$clean_context"
elif [ -f "$APP_DIR/image/new-api-corefusion-latest.tar" ]; then
  docker load -i "$APP_DIR/image/new-api-corefusion-latest.tar"
fi

cd "$APP_DIR"
mkdir -p app-index
docker compose up -d
for attempt in $(seq 1 20); do
  if docker exec corefusion-caddy wget -T 5 -qO /tmp/app-index.html http://new-api:3000/sign-in; then
    docker cp corefusion-caddy:/tmp/app-index.html app-index/index.html
    break
  fi
  sleep 2
done

if [ ! -s app-index/index.html ]; then
  echo "Failed to generate app-index/index.html from new-api."
  exit 1
fi

main_js="$(grep -o '/static/js/index\.[^"]*\.js' app-index/index.html | head -n 1 || true)"
if [ -n "$main_js" ] && ! grep -q 'rel="preload" as="script"' app-index/index.html; then
  sed -i "0,/<meta charset=\"UTF-8\" \\/>/s#<meta charset=\"UTF-8\" />#<meta charset=\"UTF-8\" />\\n    <link rel=\"preload\" as=\"script\" href=\"$main_js\" fetchpriority=\"high\">#" app-index/index.html
fi
sed -i 's/const BOOT_TIMEOUT_MS = 30000/const BOOT_TIMEOUT_MS = 90000/' app-index/index.html
docker compose restart caddy
docker compose ps

echo
echo "CoreFusion is starting. DNS must point supchuang.com to this server before HTTPS can issue."
