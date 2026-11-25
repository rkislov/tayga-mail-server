#!/usr/bin/env bash
# Private, native SearXNG service for TMS agent tools. No public reverse proxy.
set -euo pipefail
[[ $(id -u) == 0 ]] || { echo 'Run as root'; exit 1; }
base=/opt/tms-search
revision=a7163d091037cae586c444b05f22ecca162bb492
[[ ! -e "$base" && ! -e /etc/systemd/system/tms-search.service ]] || { echo 'Existing installation: inspect it before updating'; exit 1; }
export DEBIAN_FRONTEND=noninteractive
python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",18788)); s.close()'
apt-get install -y python3-venv git ca-certificates
getent passwd tms-search >/dev/null || useradd --system --home-dir /var/lib/tms-search --create-home --shell /usr/sbin/nologin tms-search
install -d -m755 "$base"
git init -q "$base/source"
git -C "$base/source" remote add origin https://github.com/searxng/searxng.git
git -C "$base/source" fetch -q --depth=1 origin "$revision"
git -C "$base/source" checkout -q --detach FETCH_HEAD
[[ $(git -C "$base/source" rev-parse HEAD) == "$revision" ]]
python3 -m venv "$base/venv"
"$base/venv/bin/pip" install --disable-pip-version-check --no-cache-dir -r "$base/source/requirements.txt" gunicorn setuptools wheel
"$base/venv/bin/pip" install --disable-pip-version-check --no-cache-dir --use-pep517 --no-build-isolation --no-deps -e "$base/source"
python3 - <<'PY'
import secrets,os
from pathlib import Path
p=Path('/opt/tms-search/settings.yml')
p.write_text('''use_default_settings: true
general:
  instance_name: TMS private search
  debug: false
search:
  formats: [json]
  safe_search: 0
  autocomplete: ''
server:
  bind_address: 127.0.0.1
  port: 18788
  limiter: false
  image_proxy: false
  public_instance: false
  secret_key: '''+secrets.token_hex(32)+'''
outgoing:
  request_timeout: 8.0
''')
os.chmod(p,0o640)
PY
chown root:tms-search "$base/settings.yml"
cat >/etc/systemd/system/tms-search.service <<'UNIT'
[Unit]
Description=Private SearXNG for TMS agents
After=network-online.target
Wants=network-online.target
[Service]
User=tms-search
Group=tms-search
WorkingDirectory=/opt/tms-search/source
Environment=SEARXNG_SETTINGS_PATH=/opt/tms-search/settings.yml
Environment=SEARXNG_BIND_ADDRESS=127.0.0.1
Environment=SEARXNG_PORT=18788
Environment=SEARXNG_DEBUG=false
Environment=PYTHONDONTWRITEBYTECODE=1
Environment=SEARXNG_DATA_PATH=/var/lib/tms-search
ExecStart=/opt/tms-search/venv/bin/gunicorn --bind 127.0.0.1:18788 --workers 1 --threads 4 --timeout 30 --access-logfile /dev/null searx.webapp:app
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/tms-search
MemoryMax=512M
TasksMax=128
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now tms-search
apt-get clean
echo 'SearXNG started on http://127.0.0.1:18788; verify JSON search before enabling a domain.'
