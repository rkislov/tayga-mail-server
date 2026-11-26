#!/usr/bin/env bash
# Two-stage upgrade: encrypted, consistent snapshot; verified package installation.
# Run verify-full on the backup before --apply. See docs/admin-guide.md.
set -euo pipefail
umask 077
mode="${1:---check}"
service="${2:-tayga.service}"
case "$mode" in --check|--backup|--apply) ;; *) echo 'Usage: update-binary.sh [--check|--backup|--apply] [tayga.service]' >&2; exit 2;; esac
[[ "$(uname -s)" == Linux ]] || { echo 'This updater requires Linux/systemd.' >&2; exit 1; }
for command in systemctl sha256sum stat readlink install mktemp curl; do command -v "$command" >/dev/null || { echo "Missing command: $command" >&2; exit 1; }; done
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
new_binary="${TAYGA_NEW_BINARY:-$script_dir/../../tayga-mail}"
new_binary="$(readlink -f -- "$new_binary")"
[[ -f "$new_binary" && -x "$new_binary" ]] || { echo 'Set TAYGA_NEW_BINARY to the extracted release binary.' >&2; exit 1; }
new_version="$("$new_binary" version)"
[[ "$new_version" == 'tayga-mail 1.3.3 '* ]] || { echo "Expected 1.3.3, received: $new_version" >&2; exit 1; }
root="$(readlink -m -- "${TAYGA_ROOT:-/var/lib/tayga}")"
config="${TAYGA_CONFIG:-$root/config/tayga.yaml}"
[[ "$root" != / && "$root" == /* ]] || { echo 'Set a specific installation root.' >&2; exit 1; }
target="${TAYGA_BINARY:-}"
if [[ -z "$target" ]]; then
 specification="$(systemctl show "$service" --property=ExecStart --value)"
 pattern='(^|[[:space:]{])path=([^;[:space:]]+)'
 if [[ "$specification" =~ $pattern ]]; then target="${BASH_REMATCH[2]}"; fi
fi
[[ "$target" == /* && -f "$target" ]] || { echo 'Set TAYGA_BINARY to the service executable.' >&2; exit 1; }
target="$(readlink -f -- "$target")"
case "$target" in "$root"/*) ;; *) echo 'Service binary must be inside TAYGA_ROOT.' >&2; exit 1;; esac
old_version="$("$target" version)"
[[ "$old_version" == 'tayga-mail '* ]] || { echo 'ExecStart does not reference Tayga.' >&2; exit 1; }
actual_sha="$(sha256sum "$new_binary" | cut -d ' ' -f 1)"
if [[ -n "${TAYGA_EXPECTED_SHA256:-}" && "$actual_sha" != "$TAYGA_EXPECTED_SHA256" ]]; then echo 'Binary SHA-256 mismatch.' >&2; exit 1; fi
printf 'Service: %s\nInstalled: %s\nNew: %s\nNew SHA-256: %s\n' "$service" "$old_version" "$new_version" "$actual_sha"
if [[ "$mode" == --check ]]; then echo 'Preflight complete. Create and verify a full backup before --apply.'; exit 0; fi
[[ "$(id -u)" == 0 ]] || { echo 'Run --backup/--apply using sudo.' >&2; exit 1; }
[[ -f "$config" ]] || { echo 'Configuration file missing.' >&2; exit 1; }
was_active=false
if systemctl is-active --quiet "$service"; then was_active=true; fi
staged=''
stopped=false
cleanup() {
 result=$?
 [[ -z "$staged" || ! -f "$staged" ]] || rm -f -- "$staged"
 if [[ "$stopped" == true && "$was_active" == true ]]; then systemctl start "$service" || true; fi
 exit "$result"
}
trap cleanup EXIT
backup_file="${TAYGA_BACKUP_FILE:-/var/backups/tayga/pre-1.3.3-$(date -u +%Y%m%d-%H%M%S).tar.gz.age}"
if [[ "$mode" == --backup ]]; then
 [[ -n "${TAYGA_RECIPIENT:-}" ]] || { echo 'Set TAYGA_RECIPIENT to an age public recipient; keep the private identity off this server.' >&2; exit 1; }
 mkdir -p -- "$(dirname -- "$backup_file")"
 chmod 700 -- "$(dirname -- "$backup_file")"
 root_bytes="$(du -sb -- "$root" | cut -f1)"
 free_bytes="$(df -PB1 -- "$(dirname -- "$backup_file")" | awk 'NR==2 {print $4}')"
 [[ "$free_bytes" -gt $((root_bytes + 268435456)) ]] || { echo 'Insufficient conservative disk reserve for the full snapshot.' >&2; exit 1; }
 systemctl stop "$service"; stopped=true
 "$target" backup-full -root "$root" -config "$config" -out "$backup_file" -recipient "$TAYGA_RECIPIENT"
 sha256sum "$backup_file" > "$backup_file.sha256"
 if [[ "$was_active" == true ]]; then systemctl start "$service"; fi
 stopped=false
 printf 'Backup: %s\n' "$backup_file"
 echo 'Verify the encrypted snapshot on an isolated machine, then use --apply.'
 exit 0
fi
[[ -n "${TAYGA_EXPECTED_SHA256:-}" ]] || { echo 'Set TAYGA_EXPECTED_SHA256 from the validated release.' >&2; exit 1; }
[[ -f "$backup_file" && -n "${TAYGA_BACKUP_SHA256:-}" ]] || { echo 'Set TAYGA_BACKUP_FILE and TAYGA_BACKUP_SHA256 to the full snapshot you verified.' >&2; exit 1; }
[[ "$(sha256sum "$backup_file" | cut -d ' ' -f 1)" == "$TAYGA_BACKUP_SHA256" ]] || { echo 'Backup SHA-256 mismatch.' >&2; exit 1; }
ready_url="${TAYGA_READY_URL:?Set TAYGA_READY_URL to the installation readiness endpoint}"
ready_args=(--fail --silent --show-error --max-time 4)
if [[ -n "${TAYGA_READY_RESOLVE:-}" ]]; then ready_args+=(--resolve "$TAYGA_READY_RESOLVE"); fi
if [[ -n "${TAYGA_READY_CA:-}" ]]; then ready_args+=(--cacert "$TAYGA_READY_CA"); fi
if [[ "$was_active" == true ]]; then curl "${ready_args[@]}" "$ready_url" >/dev/null; fi
owner="$(stat -c %u "$target")"; group="$(stat -c %g "$target")"; mode_bits="$(stat -c %a "$target")"
staged="$(mktemp "${target}.new.XXXXXXXX")"
install -o "$owner" -g "$group" -m "$mode_bits" "$new_binary" "$staged"
rollback_binary="$(mktemp "${target}.before-1.3.3.XXXXXXXX")"
cp -p -- "$target" "$rollback_binary"
systemctl stop "$service"; stopped=true
mv -f -- "$staged" "$target"; staged=''
if [[ "$was_active" == true ]]; then
 systemctl start "$service"; stopped=false
 for attempt in {1..45}; do
  if systemctl is-active --quiet "$service" && curl "${ready_args[@]}" "$ready_url" >/dev/null 2>&1; then
   "$target" version
   printf 'Ready. Prior binary: %s\nVerified full snapshot: %s\n' "$rollback_binary" "$backup_file"
   exit 0
  fi
  sleep 1
 done
 echo 'Readiness failed. Inspect journalctl before restoring; the database may have migrated. Do not roll back only the binary.' >&2
 exit 1
fi
stopped=false
printf 'Binary installed. Service was stopped and remains stopped. Full snapshot: %s\n' "$backup_file"
