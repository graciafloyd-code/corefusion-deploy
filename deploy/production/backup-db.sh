#!/usr/bin/env bash
set -euo pipefail

source ./.env
mkdir -p backups
ts="$(date +%Y%m%d-%H%M%S)"

docker exec new-api-mysql mysqldump \
  --default-character-set=utf8mb4 \
  -uroot \
  -p"${MYSQL_ROOT_PASSWORD}" \
  "${MYSQL_DATABASE}" > "backups/newapi-${ts}.sql"

echo "Wrote backups/newapi-${ts}.sql"
