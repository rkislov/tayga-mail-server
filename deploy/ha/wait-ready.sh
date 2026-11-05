#!/bin/sh
# Startup readiness only; this does not continuously check storage or fencing.
set -eu
: "${TAYGA_PUBLIC_HOST:?Set the public certificate hostname}"
attempt=0
while [ "$attempt" -lt 40 ]; do
    if curl --fail --silent --show-error --max-time 3 --noproxy '*' \
        --resolve "${TAYGA_PUBLIC_HOST}:443:127.0.0.1" \
        "https://${TAYGA_PUBLIC_HOST}/readyz" >/dev/null 2>&1; then
        exit 0
    fi
    attempt=$((attempt + 1))
    sleep 2
done
echo 'Tayga did not become ready; inspect its journal and storage.' >&2
exit 1
