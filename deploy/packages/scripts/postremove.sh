#!/bin/sh
set -eu
case "${1:-}" in
    remove|purge|0)
        if [ -d /run/systemd/system ]; then systemctl daemon-reload; fi
        ;;
esac
# Keep the service account, /var/lib/tayga and /var/log/tayga, including on purge.
