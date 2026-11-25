#!/bin/sh
set -eu
for directory in /var/lib/tayga /var/lib/tayga/data /var/lib/tayga/maildir /var/lib/tayga/certs /var/log/tayga; do
    if [ ! -d "$directory" ]; then
        install -d -m 0750 -o tayga -g tayga "$directory"
    fi
done
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload
    # Upgrade an already running service; a fresh installation stays stopped.
    systemctl try-restart tayga.service
fi
printf '%s\n' 'Tayga: set a PostgreSQL DSN in /etc/tayga/tayga.yaml or run /usr/lib/tayga-mail/postgres-bootstrap after starting local PostgreSQL.'
