#!/usr/bin/env bash
set -euo pipefail

VPS_HOST="${VPS_HOST:-72.61.114.187}"
VPS_USER="${VPS_USER:-root}"
REMOTE_DIR="${REMOTE_DIR:-/opt/corefusion}"
BUNDLE="${BUNDLE:-corefusion-production-deploy.tar.gz}"

if [ ! -f "$BUNDLE" ]; then
  echo "Bundle not found: $BUNDLE"
  echo "Run from /Users/wuquan/new-api after generating the production bundle."
  exit 1
fi

echo "Uploading $BUNDLE to ${VPS_USER}@${VPS_HOST}..."
scp "$BUNDLE" "${VPS_USER}@${VPS_HOST}:/tmp/${BUNDLE}"

echo "Installing on VPS..."
ssh "${VPS_USER}@${VPS_HOST}" bash -s <<EOF
set -euo pipefail
mkdir -p "$REMOTE_DIR"
tar -xzf "/tmp/$BUNDLE" -C /tmp
cp -a /tmp/production/. "$REMOTE_DIR"/
cd "$REMOTE_DIR"
chmod +x install.sh backup-db.sh
./install.sh
EOF

echo
echo "Done. Open https://supchuang.com after DNS resolves to $VPS_HOST."
