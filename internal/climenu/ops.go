package climenu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/settings"
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

func (m *Menu) editSettingsSection() error {
	ctx := context.Background()
	store, cfg, err := m.openStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	hub := settings.NewHub(store, cfg)
	if err := hub.Load(ctx); err != nil {
		return err
	}
	sections := settings.EditableSections
	labels := make([]string, len(sections))
	copy(labels, sections)
	n := m.ui.Menu("Секция настроек (DB)", labels)
	if n == 0 {
		return nil
	}
	name := sections[n-1]
	raw, err := hub.GetSection(name)
	if err != nil {
		return err
	}
	m.ui.Println("--- текущее JSON (редактирование: вставьте новое JSON одной строкой или оставьте пустым) ---")
	m.ui.Println(string(raw))
	next := m.ui.Prompt("Новое JSON (пусто = без изменений)", "")
	if strings.TrimSpace(next) == "" {
		m.ui.Println("Без изменений.")
		return nil
	}
	if err := hub.PutSection(ctx, name, []byte(next)); err != nil {
		return err
	}
	m.ui.Printf("Секция %s сохранена. restart_required=%v\n", name, hub.RestartRequired())
	return nil
}

func (m *Menu) transferStorage() error {
	u := m.ui
	u.Println("Перенос данных между storage (sqlite ↔ postgres) через backup/restore.")
	u.Println("Остановите tayga-mail на исходном и целевом хосте перед переносом.")
	srcCfgPath := u.Prompt("Исходный config", m.cfgPath)
	dstDriver := strings.ToLower(u.Prompt("Целевой драйвер (sqlite/postgres)", "postgres"))
	var dstStorage config.StorageConfig
	var dstMail string
	var dstCfgPath string
	srcCfg, err := config.Load(srcCfgPath)
	if err != nil {
		return err
	}
	dstMail = u.Prompt("Целевой mailstore.root", srcCfg.Mailstore.Root)
	switch dstDriver {
	case "postgres", "postgresql":
		dstStorage.Driver = "postgres"
		dstStorage.Postgres.DSN = u.Prompt("Целевой Postgres DSN", "postgres://tayga:tayga@localhost:5432/tayga?sslmode=disable")
		dstCfgPath = u.Prompt("Записать новый config как", strings.TrimSuffix(srcCfgPath, filepath.Ext(srcCfgPath))+"-postgres.yaml")
	default:
		dstStorage.Driver = "sqlite"
		dstStorage.SQLite.Path = u.Prompt("Целевой sqlite path", "./data/tayga-pg-migrated.db")
		dstCfgPath = u.Prompt("Записать новый config как", strings.TrimSuffix(srcCfgPath, filepath.Ext(srcCfgPath))+"-sqlite.yaml")
	}
	if !u.PromptYesNo("Выполнить перенос сейчас?", false) {
		u.Println("Отменено.")
		return nil
	}

	ctx := context.Background()
	src, err := storage.Open(ctx, srcCfg.Storage)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "tayga-xfer-*.tar.gz")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	u.Println("Экспорт…")
	if err := backup.WriteTarGz(ctx, src, backup.Options{
		IncludeMail: true,
		MailRoot:    srcCfg.Mailstore.Root,
	}, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	dst, err := storage.Open(ctx, dstStorage)
	if err != nil {
		return fmt.Errorf("dest: %w", err)
	}
	defer dst.Close()

	f, err := os.Open(tmpName)
	if err != nil {
		return err
	}
	defer f.Close()
	u.Println("Импорт…")
	rep, err := backup.RestoreTarGz(ctx, dst, f, backup.RestoreOptions{
		MailRoot:     dstMail,
		IncludeMail:  true,
		SkipExisting: false,
	})
	if err != nil {
		return err
	}
	u.Printf("перенесено: tenants=%d domains=%d users=%d mail=%d\n",
		rep.TenantsCreated, rep.DomainsCreated, rep.UsersCreated, rep.MailFiles)

	// write target bootstrap YAML based on source with new storage
	out := *srcCfg
	out.Storage = dstStorage
	out.Mailstore.Root = dstMail
	if err := writeConfigYAML(dstCfgPath, &out); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	u.Printf("Новый config: %s\n", dstCfgPath)
	u.Println("Дальше: tayga-mail -config " + dstCfgPath)
	m.cfgPath = dstCfgPath
	return nil
}
