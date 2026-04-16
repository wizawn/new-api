#!/bin/sh
set -eu

# Keep a headed browser display available for Playwright when headless=false.
DISPLAY_VALUE="${DISPLAY:-:99}"
export DISPLAY="${DISPLAY_VALUE}"
DISPLAY_NUM="${DISPLAY_VALUE#:}"
LOCK_FILE="/tmp/.X${DISPLAY_NUM}-lock"
SOCKET_FILE="/tmp/.X11-unix/X${DISPLAY_NUM}"

if command -v Xvfb >/dev/null 2>&1; then
  if ! pgrep -x Xvfb >/dev/null 2>&1; then
    # Remove stale lock/socket files so headed Playwright can start reliably after container restarts.
    [ -f "${LOCK_FILE}" ] && rm -f "${LOCK_FILE}"
    [ -S "${SOCKET_FILE}" ] && rm -f "${SOCKET_FILE}"
    Xvfb "${DISPLAY_VALUE}" -screen 0 1920x1080x24 -ac +extension RANDR >/tmp/xvfb.log 2>&1 &
    sleep 0.5
    if ! pgrep -x Xvfb >/dev/null 2>&1; then
      cat /tmp/xvfb.log >&2 || true
      exit 1
    fi
  fi
fi

exec /new-api "$@"
