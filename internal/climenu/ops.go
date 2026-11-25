package climenu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func (m *Menu) loadCfg() (*config.Config, error) {
	return config.Load(m.cfgPath)
}

func (m *Menu) openStore(ctx context.Context) (storage.Driver, *config.Config, error) {
	cfg, err := m.loadCfg()
	if err != nil {
		return nil, nil, err
	}
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return nil, nil, err
	}
	return store, cfg, nil
}

func (m *Menu) doBackup() error {
	cfg, err := m.loadCfg()
	if err != nil {
		return err
	}
	out := m.ui.Prompt("Файл .tar.gz", fmt.Sprintf("tayga-backup-%s.tar.gz", time.Now().Format("20060102-1504")))
	includeMail := m.ui.PromptYesNo("Включить maildir?", true)
	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := backup.WriteTarGz(ctx, store, backup.Options{
		IncludeMail: includeMail,
		MailRoot:    cfg.Mailstore.Root,
	}, f); err != nil {
		return err
	}
	m.ui.Printf("OK → %s\n", out)
	return nil
}

func (m *Menu) doRestore() error {
	cfg, err := m.loadCfg()
	if err != nil {
		return err
	}
	in := m.ui.Prompt("Файл .tar.gz", "")
	if in == "" {
		return fmt.Errorf("нужен путь к архиву")
	}
	includeMail := m.ui.PromptYesNo("Восстановить maildir?", true)
	skip := m.ui.PromptYesNo("Пропускать существующих пользователей?", false)
	if !m.ui.PromptYesNo("Продолжить restore в "+m.cfgPath+"?", false) {
		m.ui.Println("Отменено.")
		return nil
	}
	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	rep, err := backup.RestoreTarGz(ctx, store, f, backup.RestoreOptions{
		MailRoot:     cfg.Mailstore.Root,
		IncludeMail:  includeMail,
		SkipExisting: skip,
	})
	if err != nil {
		return err
	}
	m.ui.Printf("restore ok: tenants=%d domains=%d users=%d skipped=%d scripts=%d mail=%d\n",
		rep.TenantsCreated, rep.DomainsCreated, rep.UsersCreated, rep.UsersSkipped, rep.ScriptsRestored, rep.MailFiles)
	return nil
}

func (m *Menu) transferStorage() error {
	u := m.ui
	u.Println("Полный офлайн-перенос SQLite → PostgreSQL. Остановите исходный сервер.")
	sourcePath := u.Prompt("Исходный config", m.cfgPath)
	targetPath := u.Prompt("Готовый приватный config PostgreSQL (пустая БД)", strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))+"-postgres.yaml")
	reportPath := u.Prompt("Новый файл отчёта сверки", targetPath+".migration-report.json")
	source, err := config.Load(sourcePath)
	if err != nil {
		return err
	}
	target, err := config.Load(targetPath)
	if err != nil {
		return err
	}
	if source.Storage.Driver != "sqlite" || target.Storage.Driver != "postgres" {
		return fmt.Errorf("полный перенос поддерживает только SQLite → PostgreSQL; логический backup не является полной миграцией")
	}
	if source.Mailstore.Root != target.Mailstore.Root || source.MattermostAttachmentsPath() != target.MattermostAttachmentsPath() {
		return fmt.Errorf("сохраните mailstore.root и mattermost.attachment_directory в целевом config")
	}
	if !u.PromptYesNo("Перенести все таблицы в пустую БД и проверить содержимое? Активный config не изменится", false) {
		return nil
	}
	reportFile, err := os.OpenFile(reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer reportFile.Close()
	ctx := context.Background()
	driver, err := storage.OpenOffline(ctx, target.Storage)
	if err != nil {
		return fmt.Errorf("не удалось открыть целевую PostgreSQL")
	}
	defer driver.Close()
	report, transferErr := storage.TransferSQLiteToPostgres(ctx, source.Storage.SQLite.Path, driver.(*storage.Store), func(table storage.TransferTable) {
		u.Printf("проверено %s → %s: %d строк\n", table.Source, table.Target, table.Rows)
	})
	if err = json.NewEncoder(reportFile).Encode(report); err != nil {
		return err
	}
	if err = reportFile.Sync(); err != nil {
		return err
	}
	if transferErr != nil {
		return transferErr
	}
	u.Println("Все таблицы перенесены и проверены. Исходная SQLite сохранена. Переключите службу на целевой config отдельно.")
	return nil
}
