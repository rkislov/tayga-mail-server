# Кластер Tayga на одной площадке

Схема: два сервера `mail-a` и `mail-b`, один эксклюзивно монтируемый LUN, Pacemaker/Corosync, независимый qnetd и аппаратный fencing. Работает один экземпляр Tayga. Перед выполнением прочитайте [границы гарантий и состав данных](ha.md). Это инструкция для стенда и подготовки внедрения, а не протокол пройденных аварийных испытаний.

## 1. Подготовить инфраструктуру

Пример адресов: узлы `10.20.0.11/24`, `10.20.0.12/24`, сервисный VIP `10.20.0.10/24` на `ens18`; отдельная сеть Corosync; qnetd в независимом домене отказа. Замените адреса, интерфейсы, WWID и публичное имя `mail.example.com`. VIP должен быть свободен. DNS, MX, SPF, PTR, NAT и firewall должны вести к сервисному адресу; ответный SMTP-трафик должен иметь согласованный внешний IP.

Одинаковые ОС, UID/GID `tayga`, версии бинарника и системных пакетов на обоих узлах. Базовый пример рассчитан на Ubuntu 24.04 LTS; для другой версии сначала проверить синтаксис установленного `pcs`, resource agents и поддержку СХД. Установить на двух узлах:

```sh
sudo apt install pacemaker corosync pcs corosync-qdevice resource-agents fence-agents multipath-tools curl
sudo systemctl enable --now pcsd
sudo passwd hacluster
```

Пароль вводится интерактивно. На независимом арбитре установить `pcs corosync-qnetd`, запустить pcsd и подготовить пользователя для `pcs host auth`. Разрешить только кластерным адресам необходимые порты: pcsd TCP 2224, qnetd TCP 5403, Corosync согласно `corosync.conf` и BMC согласно выбранному fence-agent. Синхронизировать часы; имена узлов должны одинаково разрешаться на всех машинах.

## 2. Подготовить общее хранилище и данные

1. Настроить multipath по инструкции СХД, проверить потерю каждого пути. Использовать стабильный `/dev/disk/by-id/dm-uuid-mpath-<WWID>` или подтверждённый UUID файловой системы, а не `/dev/sdb`.
2. Обычная ext4/XFS монтируется **только на одном узле**. Не добавлять автомонтирование в fstab; не включать GFS2 и не открывать SQLite по NFS/SMB. На пустом новом томе создать файловую систему после проверки WWID. **Не форматировать существующий LUN.**
3. Для переноса работающего сервера назначить окно, остановить Tayga и все процессы, использующие его БД/файлы. Сделать полный backup вне LUN. После остановки скопировать весь `/var/lib/tayga` с сохранением владельцев, ACL и xattr (`rsync -aHAX --numeric-ids`) на временную точку нового тома. Включить SQLite WAL/SHM, если они существуют; не копировать отдельно работающий файл SQLite.
4. Проверить все пути из YAML и БД: Maildir, файлы, версии/корзину, secrets.key, TLS, DKIM, журналы. Каталоги вне общего тома перенести с корректировкой конфигурации либо документировать их отдельную репликацию. Сверить контрольные суммы и права. Не менять UID пользователей приложения, ключи OMEMO/WebAuthn, hostname или базовые URL при переносе.
5. На корне тома создать файл `.ha-volume-id` с точной строкой `tayga-prod-01` и доступом на чтение для tayga. После проверки размонтировать том. Создать пустой `/var/lib/tayga` на обоих узлах; до запуска кластера там не должно оставаться второго набора рабочих данных.

Задайте переменную **реальным проверенным** путём устройства в административной оболочке, где будут команды pcs:

```sh
TAYGA_HA_DEVICE='/dev/disk/by-id/dm-uuid-mpath-REPLACE_WITH_WWID'
test -b "$TAYGA_HA_DEVICE"
```

Для существующего сервера сохранить его SQLite. Для нового PostgreSQL-развёртывания см. отдельный раздел ниже. Перенос БД через встроенный `backup/restore` не является полной миграцией всего состояния продукта.

## 3. Подготовить службу Tayga

На оба узла установить одинаковый бинарник `/opt/tayga/0.9.30/tayga-mail` и проверить SHA256. Для другой версии заменить путь в unit на обоих узлах. Установить [unit](../deploy/ha/tayga-ha.service) в `/etc/systemd/system/` и [wait-ready.sh](../deploy/ha/wait-ready.sh) как исполняемый `/usr/local/libexec/tayga-ha-wait-ready`. Исправить `TAYGA_PUBLIC_HOST` на имя сертификата.

```sh
sudo systemctl daemon-reload
sudo systemd-analyze verify /etc/systemd/system/tayga-ha.service
```

Unit проверяет mountpoint и маркер, затем HTTPS `/readyz` через loopback с проверкой сертификата. Tayga должна слушать 443 на всех адресах: VIP появляется после её готовности. Для другого TLS-порта, reverse proxy или частного УЦ адаптировать readiness-проверку и доверие УЦ, не отключать проверку TLS.

Сохранить действующую конфигурацию в `/var/lib/tayga/config/tayga.yaml`, отключить встроенный `ha.mode` (`none`) и seed. Проверить эффективные настройки из БД. [SQLite-шаблон](../deploy/ha/tayga-sqlite.yaml) — ориентир для пустого стенда, не замена рабочего YAML. Обычную `tayga.service` остановить и отключить в согласованное окно. `tayga-ha.service` **не включать через systemctl enable**: её запускает только Pacemaker.

## 4. Создать кластер и защиту от split-brain

Команды относятся к **новому** кластеру. Сначала проверить `pcs --version` и справку локальной версии. Не выполнять `cluster setup` поверх работающей конфигурации.

```sh
sudo pcs host auth mail-a mail-b
sudo pcs cluster setup tayga-ha mail-a addr=10.20.0.11 mail-b addr=10.20.0.12
sudo pcs cluster start --all
sudo pcs cluster enable --all
sudo pcs property set maintenance-mode=true
sudo pcs property set stonith-enabled=true no-quorum-policy=stop
```

Для двух независимых heartbeat-сетей настроить второй link Corosync по установленной версии pcs; не публиковать heartbeat в интернет. На арбитре подготовить qnetd (`pcs qdevice setup model net --enable --start`). На кластере аутентифицировать арбитр через pcsd и добавить устройство:

```sh
sudo pcs host auth quorum.example.internal
sudo pcs quorum device add model net host=quorum.example.internal algorithm=ffsplit
sudo pcs quorum status
sudo pcs quorum device status
```

Qdevice предоставляет решающий голос, **но не отключает старый сервер**. Настроить STONITH для обоих узлов: BMC/IPMI, API гипервизора или другой поддерживаемый агент с независимым каналом управления. Перед созданием ресурса прочитать `pcs stonith describe <agent>`; идентификаторы питания должны точно соответствовать `mail-a` и `mail-b`. Секреты не помещать в репозиторий и shell history. После настройки испытать отключение каждого узла командой `pcs stonith fence <node>` на стенде, проверить реальное обесточивание/изоляцию, затем восстановить узел.

**Не продолжать без работающего fencing.** Не использовать `stonith-enabled=false`, `no-quorum-policy=ignore`, фиктивный fence-agent или ручное подтверждение отключения живого узла. Настройки голоса и two_node, сформированные pcs при добавлении qdevice, проверить через `corosync-quorumtool -s`; не смешивать несовместимые ручные настройки кворума.

## 5. Создать упорядоченную группу ресурсов

Для SQLite:

```sh
sudo pcs resource defaults update resource-stickiness=100
sudo pcs resource create tayga-fs ocf:heartbeat:Filesystem \
  device="$TAYGA_HA_DEVICE" directory=/var/lib/tayga fstype=ext4 \
  op monitor interval=20s timeout=40s --group tayga-stack
sudo pcs resource create tayga-app systemd:tayga-ha \
  op start timeout=270s op stop timeout=90s op monitor interval=20s timeout=30s \
  --group tayga-stack
sudo pcs resource create tayga-vip ocf:heartbeat:IPaddr2 \
  ip=10.20.0.10 cidr_netmask=24 nic=ens18 \
  op monitor interval=10s --group tayga-stack
```

Если создана XFS, указать `fstype=xfs`. Порядок группы: FS → Tayga → VIP, остановка — обратная. Stickiness предотвращает ненужный автоматический возврат на восстановившийся узел. До выхода из maintenance проверить `pcs status --full`, ограничения, fencing, сертификат, права и резервную копию. Затем:

```sh
sudo pcs property set maintenance-mode=false
sudo pcs status --full
```

На пассивном узле `/var/lib/tayga` не смонтирован, `tayga-ha` не работает. На активном доступны VIP и `/readyz`. Monitor systemd проверяет состояние процесса; зависание протокола обнаруживать отдельными HTTPS/SMTP/IMAP/FlowSync/XMPP пробами. Самодельный внешний watchdog не должен параллельно с Pacemaker запускать/переключать службы.

## PostgreSQL: только новый экземпляр

Установить PostgreSQL 16 на оба узла, отключить его обычный autostart. Создать **один** PGDATA `/var/lib/tayga/postgres/16` на общем томе с владельцем postgres, правами 0700, используя `initdb` соответствующей версии. PostgreSQL конфигурация и pg_hba.conf также находятся на томе; UNIX socket `/var/run/postgresql` существует локально на обоих узлах. Задать `listen_addresses=''`, `unix_socket_directories='/var/run/postgresql'`, локальную peer-аутентификацию. Создать роль `tayga LOGIN` и БД `tayga OWNER tayga`. Пароли/сетевой trust не нужны. Инициализацию выполнить один раз в обслуживании на единственном смонтировавшем узле, затем остановить БД и размонтировать том.

Вместо SQLite-конфигурации использовать [PostgreSQL-шаблон](../deploy/ha/tayga-postgres.yaml). Между FS и Tayga в группу добавить ресурс `ocf:heartbeat:pgsql` с `pgdata=/var/lib/tayga/postgres/16`, `pgctl=/usr/lib/postgresql/16/bin/pg_ctl`, `psql=/usr/bin/psql`, `rep_mode=none`, `socketdir=/var/run/postgresql` и отдельными timeout start/stop/monitor по измерениям стенда. Перед созданием прочитать `pcs resource describe ocf:heartbeat:pgsql` установленной версии. Итоговый порядок: FS → PostgreSQL → Tayga → VIP.

Здесь не используется Patroni или streaming replication: общий PGDATA перемещается вместе с полным состоянием приложения. Нельзя запускать второй PostgreSQL на том же PGDATA. Не отключать `fsync`/`full_page_writes` ради производительности. [Документация PostgreSQL](https://www.postgresql.org/docs/16/high-availability.html).

## 6. Проверка, переключение, backup

- Записать тестовое письмо с вложением, файл и его версию, событие с участником, XMPP-сообщение. Проверить вход и состояния клиентов до переключения.
- Планово: `pcs node standby mail-a`, дождаться переноса группы, проверить данные и протоколы; затем `pcs node unstandby mail-a`. Снять необязательные временные ограничения. Не монтировать том вручную на резервном узле.
- Испытать потерю процесса, питания активного узла, каждого пути СХД, heartbeat, qdevice. Проверить, что до запуска на новом узле старый гарантированно остановлен/изолирован. Записать фактические RTO и потери/дубли, а не только статус pcs.
- Снять полный согласованный backup вне общего тома: остановить группу, убедиться в завершении всех писателей, сделать snapshot всего тома, возобновить группу, выгрузить snapshot в независимое хранилище. Для online backup нужна отдельная проверенная процедура согласования БД и файлов. Snapshot одного дискового массива не защищает от его потери или логического удаления.
- Восстановление сначала проверить в изолированной сети с выключенными внешней доставкой, fetchmail, расписаниями и push. Не подключать восстановленный клон к рабочим клиентам. Сохранять учётные данные, ключи и идентификаторы, проверять счётчики и содержимое.
- Обновлять оба бинарника последовательно в обслуживании, сохраняя возможность отката. Не переключать между несовместимыми схемами БД. При сомнении остановить кластер и восстановить согласованный backup, а не смешивать старую БД с новыми файлами.

[Pacemaker fencing](https://clusterlabs.org/projects/pacemaker/doc/3.0/Pacemaker_Explained/html/fencing.html), [pcs](https://manpages.debian.org/bookworm/pcs/pcs.8.en.html), [Corosync qdevice](https://manpages.debian.org/bookworm/corosync-qdevice/corosync-qdevice.8.en.html).
