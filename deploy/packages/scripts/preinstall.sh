#!/bin/sh
set -eu
# A manually installed unit would shadow the packaged unit and still run the old
# executable/configuration. Never silently migrate an existing production layout.
if [ -e /etc/systemd/system/tayga.service ] || [ -L /etc/systemd/system/tayga.service ] || [ -e /var/lib/tayga/bin/tayga-mail ]; then
    echo 'Tayga: legacy installation detected. Follow docs/linux-packages.md before switching to packages.' >&2
    exit 1
fi
if ! getent group tayga >/dev/null; then
    groupadd --system tayga
fi
if ! getent passwd tayga >/dev/null; then
    useradd --system --gid tayga --home-dir /var/lib/tayga --no-create-home --shell /bin/false tayga
fi
# Existing accounts and data ownership are deliberately left unchanged.
