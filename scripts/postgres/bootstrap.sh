#!/bin/sh
# Provision one local Tayga database. PostgreSQL must already be running.
set -eu
umask 077

config=${1:-/etc/tayga/tayga.yaml}
case $config in
  /*) ;;
  *) config=$(pwd)/$config ;;
esac
[ "$(id -u)" -eq 0 ] || { echo 'Run as root on the PostgreSQL host.' >&2; exit 1; }
[ -f "$config" ] && [ ! -L "$config" ] || { echo 'Expected a regular bootstrap YAML file.' >&2; exit 1; }
[ "$(grep -cx '  driver: postgres' "$config" || true)" = 1 ] &&
[ "$(grep -cx '    dsn: ""' "$config" || true)" = 1 ] ||
  { echo 'Expected the PostgreSQL template with an empty DSN. Existing configurations are preserved.' >&2; exit 1; }

command -v psql >/dev/null || { echo 'Install and start the local PostgreSQL service first.' >&2; exit 1; }
# Optional socket and port select an alternate local instance.
if [ -n "${TAYGA_PG_SOCKET:-}" ]; then
  case $TAYGA_PG_SOCKET in /*) ;; *) echo 'Expected an absolute PostgreSQL socket directory.' >&2; exit 1;; esac
fi
if command -v runuser >/dev/null; then
  postgres() { runuser -u postgres -- env -i PATH="$PATH" PGHOST="${TAYGA_PG_SOCKET:-}" PGPORT="${TAYGA_PG_PORT:-}" psql -XqAt -v ON_ERROR_STOP=1 -d postgres "$@"; }
else
  postgres() { sudo -u postgres env -i PATH="$PATH" PGHOST="${TAYGA_PG_SOCKET:-}" PGPORT="${TAYGA_PG_PORT:-}" psql -XqAt -v ON_ERROR_STOP=1 -d postgres "$@"; }
fi
version=$(postgres -c 'SHOW server_version_num')
[ "$version" -ge 130000 ] || { echo 'PostgreSQL 13 or later is required.' >&2; exit 1; }
existing=$(postgres -c "SELECT count(*) FROM pg_roles WHERE rolname='tayga'; SELECT count(*) FROM pg_database WHERE datname='tayga';")
[ "$existing" = "$(printf '0\n0')" ] || { echo 'The Tayga role or database already exists. Use its existing DSN.' >&2; exit 1; }

# Size only the login's settings. Account for machine resources and cgroup v2
# limits when the database and installer run inside the same sandbox.
ram_mib=$(awk '/^MemTotal:/{print int($2/1024)}' /proc/meminfo)
cpus=$(getconf _NPROCESSORS_ONLN)
relative=$(awk -F: '$1=="0"{print $3}' /proc/self/cgroup)
group="/sys/fs/cgroup${relative%/}"
while [ -d "$group" ]; do
  if [ -r "$group/memory.max" ]; then
    read -r memory_cap < "$group/memory.max"
    case "$memory_cap" in
      max|'') ;;
      *) cap_mib=$((memory_cap/1048576)); if [ "$cap_mib" -lt "$ram_mib" ]; then ram_mib=$cap_mib; fi ;;
    esac
  fi
  if [ -r "$group/cpu.max" ]; then
    read -r quota period < "$group/cpu.max"
    if [ "$quota" != max ]; then
      cpu_cap=$(( (quota + period - 1) / period ))
      if [ "$cpu_cap" -lt "$cpus" ]; then cpus=$cpu_cap; fi
    fi
  fi
  [ "$group" != /sys/fs/cgroup ] || break
  group=${group%/*}
done
[ "$ram_mib" -ge 256 ] && [ "$cpus" -ge 1 ] ||
  { echo 'Local bootstrap needs at least 256 MiB of visible memory and one CPU.' >&2; exit 1; }
clamp() { if [ "$1" -lt "$2" ]; then echo "$2"; elif [ "$1" -gt "$3" ]; then echo "$3"; else echo "$1"; fi; }
work=$(clamp "$((ram_mib/512))" 1 16)
maintenance=$(clamp "$((ram_mib/32))" 16 192)
cache=$(clamp "$((ram_mib*2/5))" 96 24576)
parallel=$(clamp "$((cpus/4))" 0 2)
password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
[ "${#password}" -eq 64 ]
db_port=$(postgres -c 'SHOW port')
tmp=$(mktemp "${config}.new.XXXXXX")
trap 'rm -f "$tmp"' EXIT
# Write the generated password via stdin and keep the service config private.
printf 's|^    dsn: ""$|    dsn: "postgres://tayga:%s@127.0.0.1:%s/tayga?sslmode=disable"|\n' "$password" "$db_port" |
  sed -f - "$config" > "$tmp"
chown --reference="$config" "$tmp"
chmod --reference="$config" "$tmp"
postgres <<SQL
SET log_statement='none';
SET log_min_error_statement='panic';
SET password_encryption='scram-sha-256';
CREATE ROLE tayga LOGIN PASSWORD '$password' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
SQL
if ! postgres -c 'CREATE DATABASE tayga OWNER tayga'; then
  postgres -c 'DROP ROLE tayga'
  exit 1
fi
if ! postgres <<SQL
BEGIN;
REVOKE CONNECT ON DATABASE tayga FROM PUBLIC;
ALTER ROLE tayga IN DATABASE tayga SET work_mem = '${work}MB';
ALTER ROLE tayga IN DATABASE tayga SET maintenance_work_mem = '${maintenance}MB';
ALTER ROLE tayga IN DATABASE tayga SET effective_cache_size = '${cache}MB';
ALTER ROLE tayga IN DATABASE tayga SET max_parallel_workers_per_gather = $parallel;
COMMIT;
SQL
then
  postgres -c 'DROP DATABASE tayga'
  postgres -c 'DROP ROLE tayga'
  exit 1
fi
if ! PGPASSWORD="$password" psql -XqAt -w -h 127.0.0.1 -p "$db_port" -U tayga -d tayga -c 'SELECT 1' >/dev/null; then
  postgres -c 'DROP DATABASE tayga'
  postgres -c 'DROP ROLE tayga'
  echo 'Password authentication to local PostgreSQL failed. Check PostgreSQL loopback and pg_hba.conf.' >&2
  exit 1
fi
mv "$tmp" "$config"
printf 'PostgreSQL ready. RAM %s MiB, CPU %s. work_mem %s MB, maintenance_work_mem %s MB, effective_cache_size %s MB, parallel workers %s.\n' "$ram_mib" "$cpus" "$work" "$maintenance" "$cache" "$parallel"
