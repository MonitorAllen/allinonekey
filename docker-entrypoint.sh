#!/bin/sh
set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
DATA_DIR="${ALLINONEKEY_DATA_DIR:-/app/data}"

mkdir -p "$DATA_DIR"

if [ "$(id -u)" = "0" ]; then
  chown -R "$PUID:$PGID" "$DATA_DIR"
  exec su-exec "$PUID:$PGID" "$@"
fi

exec "$@"
