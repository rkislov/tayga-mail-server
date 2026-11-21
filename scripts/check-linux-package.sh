#!/bin/sh
# Run only in a disposable Linux container/VM, as root.
set -eu
package=$1
family=$2
export DEBIAN_FRONTEND=noninteractive
case "$family" in
  deb)
    apt-get update
    apt-get install -y --no-install-recommends "$package" curl procps
    ;;
  alt)
    apt-get update
    apt-get install -y "$package" curl procps su
    ;;
  rpm)
    dnf install -y "$package" curl procps-ng
    ;;
  *) exit 2;;
esac
/usr/bin/tayga-mail version
getent passwd tayga
su -s /bin/sh tayga -c 'test -r /etc/tayga/tayga.yaml && test -w /var/lib/tayga && test -w /var/log/tayga'
test "$(stat -c %a /etc/tayga/tayga.yaml)" = 640
test "$(stat -c %U /etc/tayga/tayga.yaml)" = root
test "$(stat -c %G /etc/tayga/tayga.yaml)" = tayga
test "$(stat -c %a /var/lib/tayga)" = 750
systemd-analyze verify /usr/lib/systemd/system/tayga.service
logrotate --debug /etc/logrotate.d/tayga-mail
# Configure an isolated instance with high ports, no seeded account or outbound mail.
cat > /tmp/tayga-package-test.yaml <<'YAML'
server:
  hostname: localhost
storage:
  driver: sqlite
  sqlite:
    path: /var/lib/tayga/data/package-test.db
mailstore:
  root: /var/lib/tayga/maildir
seed:
  enabled: false
http:
  listen: 127.0.0.1:18080
  tls_listen: ''
  public_url: http://localhost:18080
  redirect_http_to_tls: false
  admins: []
smtp:
  mx: 127.0.0.1:18025
  submission: 127.0.0.1:18587
  smtps: 127.0.0.1:18465
imap:
  listen: 127.0.0.1:18143
  imaps: 127.0.0.1:18993
pop3:
  listen: 127.0.0.1:18110
  pop3s: 127.0.0.1:18995
managesieve:
  listen: 127.0.0.1:18419
xmpp:
  enabled: false
tls:
  auto_generate: true
  cert_file: /var/lib/tayga/certs/server.crt
  key_file: /var/lib/tayga/certs/server.key
  certs_dir: /var/lib/tayga/certs/store
log:
  level: info
  format: json
  file: /var/log/tayga/tayga.log
YAML
if [ -d /run/systemd/system ]; then
  if systemctl is-active --quiet tayga.service; then echo 'Fresh install unexpectedly started service';exit 1;fi
  cp /tmp/tayga-package-test.yaml /etc/tayga/tayga.yaml
  systemctl start tayga.service
  trap 'systemctl stop tayga.service' EXIT
else
  su -s /bin/sh tayga -c 'exec /usr/bin/tayga-mail -config /tmp/tayga-package-test.yaml' > /tmp/tayga-package-test.out 2>&1 &
  runner=$!
  trap 'kill "$runner" 2>/dev/null || true' EXIT
fi
ready=false
for attempt in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:18080/readyz >/dev/null 2>&1; then ready=true;break;fi
  sleep 1
done
if [ "$ready" != true ]; then cat /tmp/tayga-package-test.out 2>/dev/null || journalctl -u tayga.service --no-pager;exit 1;fi
pid=$(pgrep -u tayga -x tayga-mail)
logrotate --force --state /tmp/tayga-logrotate.state /etc/logrotate.d/tayga-mail
test -s /var/log/tayga/tayga.log.1
if [ ! -d /run/systemd/system ]; then kill -HUP "$pid";fi
reopened=false
for attempt in $(seq 1 30); do
  if su -s /bin/sh tayga -c "ls -l /proc/$pid/fd" | grep -q '/var/log/tayga/tayga.log$'; then reopened=true;break;fi
  sleep 1
done
test "$reopened" = true
kill -0 "$pid"
curl -fsS http://127.0.0.1:18080/readyz >/dev/null
if [ ! -d /run/systemd/system ]; then
  kill -TERM "$pid"
  wait "$runner"
  trap - EXIT
fi
# Preserve administrator edits and mail during a reinstall/upgrade transaction.
printf '\n# package-test-local-setting\n' >> /etc/tayga/tayga.yaml
printf 'keep-mail\n' > /var/lib/tayga/maildir/package-test-marker
case "$family" in
 deb) dpkg -i "$package" ;;
 alt|rpm) rpm -Uvh --replacepkgs "$package" ;;
esac
grep -q package-test-local-setting /etc/tayga/tayga.yaml
if [ -d /run/systemd/system ]; then
  for attempt in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:18080/readyz >/dev/null 2>&1;then break;fi
    sleep 1
  done
  curl -fsS http://127.0.0.1:18080/readyz >/dev/null
  test "$(systemctl show -p MainPID --value tayga.service)" != "$pid"
  systemctl stop tayga.service
  dpkg -i "$package"
  if systemctl is-active --quiet tayga.service;then echo 'Stopped service unexpectedly started';exit 1;fi
fi
case "$family" in
 deb) dpkg --remove tayga-mail ;;
 alt|rpm) rpm -e tayga-mail ;;
esac
test -f /var/lib/tayga/maildir/package-test-marker
test -f /var/lib/tayga/data/package-test.db
test -f /var/log/tayga/tayga.log.1
getent passwd tayga >/dev/null
if [ "$family" = deb ]; then dpkg --purge tayga-mail; fi
test -f /var/lib/tayga/maildir/package-test-marker
trap - EXIT
printf '%s\n' 'PASS: native install, ownership, service syntax, process readiness, live log rotation, config preservation, removal and retained data.'
