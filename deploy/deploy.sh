#!/usr/bin/env bash
# Build a linux/amd64 release and ship it to the droplet.
#
#   deploy/deploy.sh deploy@<ip>                 # normal deploy, as the unprivileged deploy user
#   deploy/deploy.sh root@<ip> --setup <site>    # first time only: run setup-debian.sh as root
#
# Steps: make release -> upload binaries + deploy/ -> backup DB -> swap binaries (keeping .prev)
# -> restart -> wait for /healthz. Rolls back to .prev if the new binary does not come up.
#
# The deploy user owns /opt/bigwaves and may only run, through sudo without a password,
#   systemctl restart bigwaves
#   systemctl start bigwaves-backup.service
# (set up by setup-debian.sh). Files outside /opt/bigwaves (systemd units, /usr/local/bin/bwctl)
# are installed only when running as root; otherwise the script says what changed and how to
# install it.
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
  | ssh "$host" 'rm -rf ~/.bigwaves-release && mkdir -p ~/.bigwaves-release && tar -xz -C ~/.bigwaves-release'

ssh "$host" SETUP_SITE="$setup_site" 'bash -s' <<'REMOTE'
set -euo pipefail
rel="$HOME/.bigwaves-release"
is_root=false
# Logged in as root on a fresh droplet: sudo may not be installed yet.
if [[ $EUID -eq 0 ]]; then is_root=true; sudo() { "$@"; }; fi

if [[ -n "$SETUP_SITE" ]]; then
  $is_root || { echo "--setup must run as root (deploy.sh root@<ip> --setup <site>)" >&2; exit 1; }
  mkdir -p /opt/bigwaves
  rm -rf /opt/bigwaves/deploy
  cp -a "$rel/deploy" /opt/bigwaves/deploy
  /opt/bigwaves/deploy/setup-debian.sh "$SETUP_SITE"
fi

systemctl cat bigwaves.service >/dev/null 2>&1 || { echo "server not set up: run deploy.sh root@<ip> --setup <site>" >&2; exit 1; }
# Only root can read the env file; the deploy user cannot (and should not) see the secrets.
if $is_root && ! grep -q '^AUTH0_CLIENT_SECRET=.' /etc/bigwaves/env; then
  echo "/etc/bigwaves/env has no secrets yet: fill it in, then re-run deploy.sh (without --setup)" >&2
  exit 1
fi
if ! $is_root && ! sudo -n -l systemctl restart bigwaves >/dev/null 2>&1; then
  echo "$(id -un) may not run 'sudo systemctl restart bigwaves' without a password: see deploy/README.md" >&2
  exit 1
fi

rm -rf /opt/bigwaves/deploy
cp -a "$rel/deploy" /opt/bigwaves/deploy

echo "==> backup before migrate"
sudo systemctl start bigwaves-backup.service

echo "==> install"
cd /opt/bigwaves/bin
for b in bigwaves bwctl; do
  [[ -f $b ]] && cp -p $b $b.prev
  install -m 0755 "$rel/$b" $b.new
  mv -f $b.new $b
done
install -m 0755 /opt/bigwaves/deploy/backup.sh /opt/bigwaves/bin/backup.sh
# A root deploy must leave /opt/bigwaves writable for the deploy user.
if $is_root && id deploy &>/dev/null; then chown -R deploy:deploy /opt/bigwaves; fi

# Files outside /opt/bigwaves: install when root, otherwise report what is out of date.
stale=()
cmp -s /opt/bigwaves/deploy/bwctl /usr/local/bin/bwctl || stale+=(/usr/local/bin/bwctl)
for u in /opt/bigwaves/deploy/systemd/bigwaves*; do
  cmp -s "$u" "/etc/systemd/system/$(basename "$u")" || stale+=("/etc/systemd/system/$(basename "$u")")
done
if ((${#stale[@]})); then
  if $is_root; then
    install -m 0755 /opt/bigwaves/deploy/bwctl /usr/local/bin/bwctl
    install -m 0644 /opt/bigwaves/deploy/systemd/bigwaves* /etc/systemd/system/
    systemctl daemon-reload
  else
    echo "!! out of date, needs root: ${stale[*]}" >&2
    echo "!! install with: ssh root@<ip> 'install -m 0755 /opt/bigwaves/deploy/bwctl /usr/local/bin/bwctl && install -m 0644 /opt/bigwaves/deploy/systemd/bigwaves* /etc/systemd/system/ && systemctl daemon-reload && systemctl restart bigwaves'" >&2
  fi
fi

echo "==> restart"
sudo systemctl restart bigwaves
for i in $(seq 1 30); do
  if curl -fs -o /dev/null http://127.0.0.1:8080/healthz; then
    echo "healthy after ${i}s"
    rm -rf "$rel"
    exit 0
  fi
  sleep 1
done

echo "!! /healthz did not answer in 30s, rolling back" >&2
journalctl -u bigwaves -n 40 --no-pager >&2 || true
if [[ -f bigwaves.prev ]]; then
  cp -p bigwaves.prev bigwaves && cp -p bwctl.prev bwctl
  sudo systemctl restart bigwaves
  echo "rolled back to previous binary (DB may already be migrated: check backups/)" >&2
fi
exit 1
REMOTE
