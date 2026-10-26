#!/usr/bin/env bash
# Online backup of the SQLite DB with VACUUM INTO (safe while the app is running in WAL mode).
# Keeps the newest $BACKUP_KEEP files. Run by bigwaves-backup.service, and by deploy.sh before
# every release since the new binary migrates the schema on start.
set -euo pipefail

db="${DB_PATH:?DB_PATH is not set}"
dir="${BACKUP_DIR:-/var/lib/bigwaves/backups}"
keep="${BACKUP_KEEP:-14}"

mkdir -p "$dir"
out="$dir/bigwaves-$(TZ=Asia/Ho_Chi_Minh date +%Y%m%d-%H%M%S).db"

sqlite3 "$db" "VACUUM INTO '$out'"
sqlite3 "$out" "PRAGMA integrity_check" | grep -qx ok || { echo "integrity_check failed: $out" >&2; exit 1; }
gzip -f "$out"
echo "backup: $out.gz ($(du -h "$out.gz" | cut -f1))"

ls -1t "$dir"/bigwaves-*.db.gz | tail -n +"$((keep + 1))" | xargs -r rm -f --
