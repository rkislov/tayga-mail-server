#!/usr/bin/env bash
# Upgrade the binary of an existing Tayga systemd service. No config files are copied.
set -euo pipefail
mode="${1:---check}"
service="${2:-tayga.service}"
if [[ "$mode" != --check && "$mode" != --apply ]]; then
  echo "Использование: sudo bash update-binary.sh [--check|--apply] [tayga.service]" >&2
  exit 2
fi
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { echo "Этот пакет предназначен для Linux x64." >&2; exit 1; }
for cmd in systemctl sha256sum stat readlink install mktemp; do command -v "$cmd" >/dev/null || { echo "Не найдена команда: $cmd" >&2; exit 1; }; done
package_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
new_binary="$package_dir/tayga-mail"
[[ -f "$new_binary" && -f "$package_dir/binary.sha256" ]] || { echo "Запускайте скрипт из распакованного архива рядом с tayga-mail и binary.sha256." >&2; exit 1; }
(cd "$package_dir" && sha256sum -c binary.sha256)
new_version="$("$new_binary" version)"
[[ "$new_version" == 'tayga-mail 0.9.35 '* ]] || { echo "Неожиданная версия нового бинарника: $new_version" >&2; exit 1; }
target="${TAYGA_BINARY:-}"
if [[ -z "$target" ]]; then
  specification="$(systemctl show "$service" --property=ExecStart --value)"
  path_pattern='(^|[[:space:]{])path=([^;[:space:]]+)'
  if [[ "$specification" =~ $path_pattern ]]; then target="${BASH_REMATCH[2]}"; fi
fi
[[ "$target" == /* && -f "$target" ]] || { echo "Не удалось определить бинарник службы. Задайте TAYGA_BINARY=/полный/путь/tayga-mail." >&2; exit 1; }
target="$(readlink -f -- "$target")"
old_version="$("$target" version)"
[[ "$old_version" == 'tayga-mail '* ]] || { echo "ExecStart не указывает на бинарник Tayga. Задайте TAYGA_BINARY явно." >&2; exit 1; }
printf 'Служба: %s\nБинарник: %s\nСейчас: %s\nОбновление: %s\n' "$service" "$target" "$old_version" "$new_version"
if [[ "$mode" == --check ]]; then
  echo "Проверка завершена. Для установки: sudo bash update-binary.sh --apply $service"
  exit 0
fi
[[ "$(id -u)" == 0 ]] || { echo "Для установки запустите скрипт через sudo." >&2; exit 1; }
ready_url="${TAYGA_READY_URL:-https://mail.kislovs.ru/readyz}"
command -v curl >/dev/null || { echo "Для проверки готовности требуется curl." >&2; exit 1; }
was_active=false
if systemctl is-active --quiet "$service"; then was_active=true; fi
backup="$(mktemp "${target}.before-0.9.35.XXXXXXXX")"
cp -p -- "$target" "$backup"
staged="$(mktemp "${target}.new.XXXXXXXX")"
trap 'if [[ -n "${staged:-}" && -f "$staged" ]]; then rm -f -- "$staged"; fi' EXIT
install -o "$(stat -c %u "$target")" -g "$(stat -c %g "$target")" -m "$(stat -c %a "$target")" "$new_binary" "$staged"
mv -f -- "$staged" "$target"
staged=""
printf 'Бинарник установлен. Предыдущая версия: %s\n' "$backup"
if [[ "$was_active" == true ]]; then
  systemctl restart "$service"
  for attempt in {1..30}; do
    if systemctl is-active --quiet "$service" && curl -fsS --max-time 3 "$ready_url" >/dev/null; then
      echo "Служба работает, проверка готовности пройдена. В Thunderbird нажмите «Синхронизировать сейчас»."
      exit 0
    fi
    sleep 1
  done
  echo "Проверка готовности не пройдена. Диагностика: sudo journalctl -u $service -n 80 --no-pager" >&2
  exit 1
fi
echo "Служба была остановлена. Бинарник обновлён; запуск: sudo systemctl start $service"
