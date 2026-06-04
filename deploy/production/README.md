# CoreFusion Production Deploy

Domain: https://supchuang.com
VPS: 72.61.114.187
OS: Ubuntu 24.04.4 LTS

## DNS

Before starting Caddy, point these records to `72.61.114.187`:

- `supchuang.com` A `72.61.114.187`
- `www.supchuang.com` A `72.61.114.187`

Current DNS must resolve correctly before Let's Encrypt can issue HTTPS certificates.

See `DNS_AND_SSH.md` for the exact DNS records and SSH public key.

## Server Commands

Install Docker:

```bash
apt update
apt install -y ca-certificates curl git
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo ${UBUNTU_CODENAME:-$VERSION_CODENAME}) stable" > /etc/apt/sources.list.d/docker.list
apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Open firewall:

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
```

Run:

```bash
cd /opt/corefusion
docker compose up -d
docker compose ps
```

Or unpack this bundle on the VPS and run:

```bash
chmod +x install.sh
./install.sh
```

After first boot, update `ServerAddress` in the database/admin panel to:

```text
https://supchuang.com
```

## Bundle Contents

- `docker-compose.yml` — production compose file
- `Caddyfile` — HTTPS reverse proxy for `supchuang.com` and `www.supchuang.com`
- `.env` — production secrets and database credentials
- `mysql-init/01-newapi.sql` — current configured database snapshot, loaded only on first MySQL initialization
- `image/new-api-corefusion-latest.tar` — offline Docker image
- `install.sh` — copy files to `/opt/corefusion`, load image, and start services
- `backup-db.sh` — create database backups after deployment
