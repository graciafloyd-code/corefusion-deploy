# DNS and SSH Prerequisites

## DNS

Set these DNS records at the domain provider:

```text
supchuang.com      A      72.61.114.187
www.supchuang.com  A      72.61.114.187
```

Remove or replace the current A records pointing to:

```text
13.248.243.5
76.223.105.230
```

## SSH

The current local public key is:

```text
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA7t5RwbopXfWU1jvC3gB/Ghqdy9c+0Q0sQY8rEVUTNT wuquan@localhost
```

Add it to the VPS root user's authorized keys:

```bash
mkdir -p /root/.ssh
chmod 700 /root/.ssh
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA7t5RwbopXfWU1jvC3gB/Ghqdy9c+0Q0sQY8rEVUTNT wuquan@localhost' >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

Then deploy from this machine:

```bash
cd /Users/wuquan/new-api
chmod +x deploy-to-vps.sh
./deploy-to-vps.sh
```
