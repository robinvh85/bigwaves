#!/usr/bin/env bash
# Build a linux/amd64 release and ship it to the droplet.
#
#   deploy/deploy.sh <ssh-host>            # e.g. root@203.0.113.10 or an ~/.ssh/config alias
#   deploy/deploy.sh <ssh-host> --setup <site>   # first time: also run setup-debian.sh
#
# Steps: make release -> upload binaries + deploy/ -> backup DB -> swap binaries (keeping .prev)
# -> restart -> wait for /healthz. Rolls back to .prev if the new binary does not come up.
set -euo pipefail

host="${1:?usage: deploy.sh <ssh-host> [--setup <site>]}"
shift
setup_site=""
if [[ "${1:-}" == "--setup" ]]; then setup_site="${2:?--setup needs <site>}"; fi

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

echo "==> make release"
make release

echo "==> upload"
# tar over ssh: a fresh Debian droplet has no rsync. --no-xattrs keeps macOS xattrs
# (com.apple.provenance) out of the archive, which GNU tar would warn about.
COPYFILE_DISABLE=1 tar --no-xattrs -cz deploy -C backend/bin bigwaves bwctl \
  | ssh "$host" 'rm -rf /tmp/bigwaves-release && mkdir -p /tmp/bigwaves-release && tar -xz -C /tmp/bigwaves-release'

ssh "$host" SETUP_SITE="$setup_site" 'bash -s' <<'REMOTE'
set -euo pipefail
# Logged in as root on a fresh droplet: sudo may not be installed yet.
if [[ $EUID -eq 0 ]]; then sudo() { "$@"; }; fi
sudo mkdir -p /opt/bigwaves
sudo rm -rf /opt/bigwaves/deploy
sudo cp -a /tmp/bigwaves-release/deploy /opt/bigwaves/deploy

if [[ -n "$SETUP_SITE" ]]; then
  sudo /opt/bigwaves/deploy/setup-debian.sh "$SETUP_SITE"
fi
[[ -f /etc/bigwaves/env ]] || { echo "server not set up: run with --setup <site>" >&2; exit 1; }
if ! sudo grep -q '^AUTH0_CLIENT_SECRET=.' /etc/bigwaves/env; then
  echo "/etc/bigwaves/env has no secrets yet: fill it in, then re-run deploy.sh (without --setup)" >&2
  exit 1
fi

if [[ -f /var/lib/bigwaves/bigwaves.db ]]; then
  echo "==> backup before migrate"
  sudo systemctl start bigwaves-backup.service
fi

echo "==> install"
cd /opt/bigwaves/bin
for b in bigwaves bwctl; do
  [[ -f $b ]] && sudo cp -p $b $b.prev
  sudo install -m 0755 /tmp/bigwaves-release/$b $b.new
  sudo mv -f $b.new $b
done
sudo install -m 0755 /opt/bigwaves/deploy/backup.sh /opt/bigwaves/bin/backup.sh
sudo install -m 0755 /opt/bigwaves/deploy/bwctl /usr/local/bin/bwctl
if ! sudo cmp -s /opt/bigwaves/deploy/systemd/bigwaves.service /etc/systemd/system/bigwaves.service; then
  sudo install -m 0644 /opt/bigwaves/deploy/systemd/bigwaves* /etc/systemd/system/
  sudo systemctl daemon-reload
fi

echo "==> restart"
sudo systemctl restart bigwaves
for i in $(seq 1 30); do
  if curl -fs -o /dev/null http://127.0.0.1:8080/healthz; then
    echo "healthy after ${i}s"
    rm -rf /tmp/bigwaves-release
    exit 0
  fi
  sleep 1
done

echo "!! /healthz did not answer in 30s, rolling back" >&2
sudo journalctl -u bigwaves -n 40 --no-pager >&2 || true
if [[ -f bigwaves.prev ]]; then
  sudo cp -p bigwaves.prev bigwaves && sudo cp -p bwctl.prev bwctl
  sudo systemctl restart bigwaves
  echo "rolled back to previous binary (DB may already be migrated: check backups/)" >&2
fi
exit 1
REMOTE
