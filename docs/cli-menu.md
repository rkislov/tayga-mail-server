# Консольное меню (`tayga-mail menu`)

Интерактивный CLI для первичной настройки, backup/restore и переноса хранилища.

```bash
tayga-mail menu -config /etc/tayga/tayga.yaml
# синонимы: setup, console
```

## Пункты

| Пункт | Действие |
|-------|----------|
| Первичная настройка | wizard → bootstrap YAML (hostname, sqlite/postgres, seed, TLS paths) |
| Показать bootstrap | краткое резюме текущего `-config` |
| Сменить путь к config | рабочий файл для остальных операций |
| Backup | `backup.WriteTarGz` (с maildir по желанию) |
| Restore | `backup.RestoreTarGz` в текущий storage/maildir |
| Перенос БД / хранилища | экспорт из источника → импорт в sqlite или postgres → новый YAML |
| Настройки сервера | GET/PUT секции DB (`settings` hub), JSON одной строкой |
| О версии | `version.String()` |

Перед переносом sqlite↔postgres **остановите** `tayga-mail`. После переноса запускайте с новым config.

См. также: [setup.md](./setup.md), [ha.md](./ha.md), [migration.md](./migration.md) (пользовательский IMAP/DAV импорт).
