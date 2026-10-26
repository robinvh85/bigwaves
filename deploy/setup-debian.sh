#!/usr/bin/env bash
# One-time (idempotent) setup of a fresh Debian 12/13 droplet for bigwaves.
#
#   sudo /opt/bigwaves/deploy/setup-debian.sh <site>     # e.g. bigwaves.example.com
#
# deploy.sh uploads this directory to /opt/bigwaves/deploy first. Safe to re-run.
set -euo pipefail

site="${1:?usage: setup-debian.sh <site, e.g. bigwaves.example.com>}"
here="$(cd "$(dirname "$0")" && pwd)"
swap_size="${SWAP_SIZE:-1G}"

[[ $EUID -eq 0 ]] || { echo "run as root" >&2; exit 1; }
. /etc/os-release
[[ "$ID" == debian ]] || echo "warning: tested on Debian, this is $PRETTY_NAME" >&2

log() { printf '\n==> %s\n' "$*"; }

log "Timezone"
timedatectl set-timezone Asia/Ho_Chi_Minh

log "Packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -y -q --no-install-recommends \
  sudo sqlite3 curl ca-certificates gnupg debian-keyring debian-archive-keyring apt-transport-https \
  unattended-upgrades

log "Unattended security upgrades"
cat > /etc/apt/apt.conf.d/20auto-upgrades <<'CONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
CONF

log "Swap ($swap_size)"
if ! swapon --show=NAME --noheadings | grep -q .; then
  fallocate -l "$swap_size" /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
cat > /etc/sysctl.d/90-bigwaves.conf <<'CONF'
vm.swappiness = 10
vm.vfs_cache_pressure = 50
CONF
sysctl -q --system

log "journald size cap"
mkdir -p /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/90-bigwaves.conf <<'CONF'
[Journal]
SystemMaxUse=200M
MaxRetentionSec=1month
CONF
systemctl restart systemd-journald

log "User and directories"
id bigwaves &>/dev/null || useradd --system --home-dir /var/lib/bigwaves --shell /usr/sbin/nologin bigwaves
install -d -o root     -g root     -m 0755 /opt/bigwaves /opt/bigwaves/bin
install -d -o bigwaves -g bigwaves -m 0750 /var/lib/bigwaves /var/lib/bigwaves/backups
install -d -o root     -g bigwaves -m 0750 /etc/bigwaves
if [[ ! -f /etc/bigwaves/env ]]; then
  sed -e 's|^APP_ENV=.*|APP_ENV=production|' \
      -e 's|^HTTP_ADDR=.*|HTTP_ADDR=127.0.0.1:8080|' \
      -e "s|^APP_URL=.*|APP_URL=https://$site|" \
      -e 's|^DB_PATH=.*|DB_PATH=/var/lib/bigwaves/bigwaves.db|' \
      "$here/.env.example" > /etc/bigwaves/env
  echo "created /etc/bigwaves/env — fill in the secrets before starting the service"
fi
chown root:bigwaves /etc/bigwaves/env
chmod 0640 /etc/bigwaves/env

install -m 0755 "$here/backup.sh" /opt/bigwaves/bin/backup.sh
install -m 0755 "$here/bwctl" /usr/local/bin/bwctl

log "systemd units"
install -m 0644 "$here"/systemd/bigwaves.service \
                "$here"/systemd/bigwaves-backup.service \
                "$here"/systemd/bigwaves-backup.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable bigwaves.service bigwaves-backup.timer
systemctl start bigwaves-backup.timer

log "Caddy"
if ! command -v caddy &>/dev/null; then
  curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/gpg.key \
    | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt \
    > /etc/apt/sources.list.d/caddy-stable.list
  apt-get update -q
  apt-get install -y -q caddy
fi
sed "s|__SITE__|$site|" "$here/Caddyfile" > /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl enable caddy
systemctl reload-or-restart caddy

log "Done"
cat <<MSG
Next:
  1. Edit /etc/bigwaves/env (Auth0, Binance, Discord...). Keep TRADING_ENABLED=false until cutover.
  2. Point DNS for $site to this droplet, open 80/443 in the DO Cloud Firewall.
  3. From your machine: deploy/deploy.sh <ssh-host>
MSG
