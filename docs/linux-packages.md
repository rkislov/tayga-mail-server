# Linux: пакеты DEB и RPM

Выпуск 1.0.5 добавляет пакеты `tayga-mail_1.0.5_amd64.deb`, `tayga-mail_1.0.5_arm64.deb`, `tayga-mail_1.0.5_amd64.rpm` и `tayga-mail_1.0.5_arm64.rpm`. Бинарный файл статически собран с `CGO_ENABLED=0`; отдельная версия glibc не требуется. Пакеты требуют systemd, logrotate и системные корневые сертификаты. Архитектура RPM внутри пакета — x86_64 / aarch64.

Целевые системы:

- DEB: Ubuntu 20.04, 22.04, 24.04, 26.04 LTS и 26.10; Astra Linux SE 1.7 и 1.8.
- RPM: ALT p10/p11, CentOS Stream 9/10, РЕД ОС 7.3/8.
- Архивы: Linux и macOS amd64/arm64, Windows amd64/arm64, FreeBSD amd64.

Одинаковый пакет используется в нескольких дистрибутивах. Матрица проверки установки находится в workflow `Linux packages`; контейнерная проверка не является сертификацией Astra Linux / РЕД ОС и не проверяет мандатные политики, SELinux или загрузку ядра конкретной ОС. Подробные результаты и ограничения записаны в [заметках выпуска](release-1.0.5.md).

Версии сверены с официальными источниками: [CentOS](https://www.centos.org/download/), [РЕД ОС](https://redos.red-soft.ru/base/update/version-and-term/), [образы Astra](https://registry.astralinux.ru/), [Ubuntu](https://ubuntu.com/about/release-cycle), [ALT Docker images](https://github.com/docker-library/official-images/blob/master/library/alt).

## Новая установка

Скачайте нужный пакет и `checksums.txt` из [выпуска](https://github.com/rkislov/tayga-mail-server/releases/tag/v1.0.5). Проверьте SHA-256 скачанного файла по `checksums.txt` до установки. RPM опубликован без GPG-подписи пакета; контрольные суммы приложены к выпуску. Настройки проверки подписей системных репозиториев менять не нужно.

Ubuntu / Astra (выберите архитектуру своего сервера):

```sh
sudo apt-get install ./tayga-mail_1.0.5_amd64.deb
```

ALT:

```sh
sudo apt-get install ./tayga-mail_1.0.5_amd64.rpm
```

CentOS / РЕД ОС:

```sh
sudo dnf install ./tayga-mail_1.0.5_amd64.rpm
```

При установке создаётся системная учётная запись `tayga`. Служба **не запускается и не включается автоматически** при первой установке. Пароля администратора по умолчанию нет.

Откройте `sudoedit /etc/tayga/tayga.yaml` и задайте:

1. `server.hostname` и `http.public_url` для своего домена.
2. `seed.enabled: true`, свой `seed.domain`, адрес `seed.email` и уникальный `seed.password` для первого запуска.
3. Тот же адрес в списке `http.admins`.
4. Сертификаты TLS и остальные параметры по [руководству установки](setup.md).

Штатный YAML уже задаёт пути пакета. Оставьте их, если не требуется перенос хранилища. После настройки:

```sh
sudo systemctl enable --now tayga.service
sudo systemctl status tayga.service --no-pager
sudo journalctl -u tayga.service -n 50 --no-pager
```

После создания администратора выключите `seed.enabled` и удалите пароль из YAML. Для публичного сервера настройте доверенный TLS-сертификат, DNS/MX и доступность почтовых портов.

## Файлы и права

- `/usr/bin/tayga-mail` — исполняемый файл, root:root.
- `/etc/tayga/tayga.yaml` — bootstrap, root:tayga, 0640; каталог 0750.
- `/var/lib/tayga` — база, письма, сертификаты и остальные данные, tayga:tayga, 0750.
- `/var/log/tayga/tayga.log` — основной журнал, tayga:tayga, 0640.
- `/usr/lib/systemd/system/tayga.service` — служба с отдельным пользователем и `CAP_NET_BIND_SERVICE`.
- `/etc/logrotate.d/tayga-mail` — сохраняемые настройки ротации.

Пакет не меняет firewall, SELinux/AppArmor/мандатные политики и настройки системного journald. Они управляются администратором ОС. Для нестандартных путей добавьте нужные `ReadWritePaths` через `systemctl edit tayga.service`.

## Обновление и удаление

Перед обновлением создайте и проверьте резервную копию по [руководству администратора](admin-guide.md). Установка нового пакета сохраняет изменённые конфигурационные файлы: DEB использует conffiles, RPM — `%config(noreplace)`. Проверяйте `.dpkg-dist` / `.rpmnew` при изменении шаблона.

Если служба работает, обновление перезапускает её. Если она остановлена, пакет её не запускает. При ошибке запуска проверьте `journalctl -u tayga`; после миграции БД восстанавливайте согласованную копию, а не только старый бинарный файл.

Удаление пакета останавливает и отключает службу, но сохраняет пользователя, письма, базу и журналы. `apt purge` может удалить bootstrap; данные в `/var/lib/tayga` сохраняются. Автоматического удаления данных нет.

## Переход с установки из архива

Пакет намеренно останавливает установку, если обнаружен старый `/etc/systemd/system/tayga.service` или `/var/lib/tayga/bin/tayga-mail`: такая служба перекрывает пакетную и продолжает использовать старые пути. Для работающего сервера используйте прежний `deploy/systemd/update-binary.sh` до отдельного планового переноса.

Для перехода: сделайте полную проверенную копию, остановите службу, сохраните старую unit-конфигурацию и бинарный файл вне указанных путей, затем установите пакет. Перенесите bootstrap в `/etc/tayga/tayga.yaml`, сохранив фактические пути БД, maildir и сертификатов. Укажите `/var/log/tayga/tayga.log`, проверьте права и ограничения unit. Выполните `systemctl daemon-reload`, запустите службу и проверьте `/readyz`, авторизацию и почтовые протоколы. Скрипты пакета не выполняют этот перенос автоматически.

## Logrotate

Правило обрабатывает только `/var/log/tayga/tayga.log`: ежедневная ротация, 14 архивов, `maxsize 100M`, сжатие со следующего цикла (`delaycompress`), пропуск пустых и отсутствующих файлов. Размер проверяется при запуске logrotate, а не непрерывно; при большом потоке настройте частоту системного запуска. Существующий системный таймер/cron logrotate используется без добавления второго расписания.

Ротация переименовывает файл и создаёт новый 0640 от tayga. После этого `SIGHUP` отправляется только главному процессу службы. Tayga переоткрывает журнал под блокировкой записи; `copytruncate` не используется. Ошибка переоткрытия оставляет старый дескриптор и выдаёт сообщение в stderr/journald. SIGHUP не перечитывает конфигурацию сервера.

```sh
sudo logrotate --debug /etc/logrotate.d/tayga-mail
sudo logrotate --force /etc/logrotate.d/tayga-mail
sudo systemctl is-active tayga.service
```

Если изменили `log.file`, измените и правило logrotate. Правило пакета не очищает журналы почтовых событий в базе и журнал systemd.

## English

Release 1.0.5 includes static amd64/arm64 DEB and RPM packages. DEB targets Ubuntu 20.04+ and Astra Linux SE 1.7/1.8; RPM targets ALT p10/p11, CentOS Stream 9/10 and RED OS 7.3/8. See the release notes and Linux packages workflow for actual test coverage.

Install with `apt-get install ./package.deb`, ALT `apt-get install ./package.rpm`, or `dnf install ./package.rpm`. Configure `/etc/tayga/tayga.yaml` before enabling `tayga.service`. No default administrator password or automatic first start is provided. Package upgrades retain configuration and restart only an active service. Removal retains accounts, mail, database and logs. Existing archive-based installations require a planned migration; they are rejected rather than silently replaced.

Logs use `/var/log/tayga/tayga.log`, daily rotation, 14 retained files, a 100 MiB threshold checked by the system scheduler, delayed compression and SIGHUP reopening without restarting the service. File permissions are 0640. See the Russian sections above for paths, commands and migration details.
