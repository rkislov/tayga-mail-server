#!/bin/sh
set -eu
# Debian: remove; RPM: 0 on final erase, 1 on removal of an upgraded old version.
case "${1:-}" in
    remove|0)
        if [ -d /run/systemd/system ]; then
            systemctl stop tayga.service
            systemctl disable tayga.service >/dev/null 2>&1 || true
        fi
        ;;
esac
