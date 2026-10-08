package climenu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"gopkg.in/yaml.v3"
)

func (m *Menu) editSettingsSection() error {
	n := m.ui.Menu("Настройки сервера", []string{
		"SMTP / исходящая",
		"Spam / Rspamd",
		"Антивирус / ICAP",
		"SIEM (syslog CEF)",
		"Логирование",
		"CoS (классы обслуживания)",
		"Другая секция (JSON)",
	})
	switch n {
	case 0:
		return nil
	case 1:
		return m.editSMTPForm()
	case 2:
		return m.editSpamForm()
	case 3:
		return m.editScanForm()
	case 4:
		return m.editSIEMForm()
	case 5:
		return m.editLogForm()
	case 6:
		return m.editCoS()
	case 7:
		return m.editSettingsJSON()
	}
	return nil
}

func (m *Menu) withHub(fn func(ctx context.Context, hub *settings.Hub, cfg *config.Config, store storage.Driver) error) error {
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
	return fn(ctx, hub, hub.Config(), store)
}

func (m *Menu) putSection(ctx context.Context, hub *settings.Hub, name string, v any) error {
	raw, err := sectionToJSON(v)
	if err != nil {
		return err
	}
	if err := hub.PutSection(ctx, name, raw); err != nil {
		return err
	}
	m.ui.Printf("Секция %s сохранена. restart_required=%v\n", name, hub.RestartRequired())
	return nil
}

func (m *Menu) loadSection(hub *settings.Hub, name string, dst any) error {
	raw, err := hub.GetSection(name)
	if err != nil {
		return err
	}
	return sectionFromJSON(raw, dst)
}

// sectionToJSON encodes via YAML tags so duration/field names match settings hub.
func sectionToJSON(v any) ([]byte, error) {
	y, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(y, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return json.Marshal(m)
}

func sectionFromJSON(data []byte, v any) error {
	var n any
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	y, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(y, v)
}

func (m *Menu) editSMTPForm() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, _ storage.Driver) error {
		var cur config.SMTPConfig
		if err := m.loadSection(hub, "smtp", &cur); err != nil {
			return err
		}
		u := m.ui
		u.Println("=== SMTP / исходящая ===")
		cur.OutboundDirect = u.PromptYesNo("Прямая доставка (MX)", cur.OutboundDirect)
		cur.Relay.Host = u.Prompt("Relay host (пусто = выкл)", cur.Relay.Host)
		cur.Relay.Username = u.Prompt("Relay username", cur.Relay.Username)
		if pass := u.Prompt("Relay password (пусто = не менять)", ""); pass != "" {
			cur.Relay.Password = pass
		}
		cur.Relay.DisableSTARTTLS = u.PromptYesNo("Disable STARTTLS", cur.Relay.DisableSTARTTLS)
		cur.Queue.Enabled = u.PromptYesNo("Включить очередь", cur.Queue.Enabled)
		cur.Queue.Workers = promptInt(u, "Queue workers", cur.Queue.Workers, 1)
		cur.Queue.MaxAttempts = promptInt(u, "Max attempts", cur.Queue.MaxAttempts, 8)
		if !u.PromptYesNo("Сохранить?", true) {
			u.Println("Отменено.")
			return nil
		}
		return m.putSection(ctx, hub, "smtp", cur)
	})
}

func (m *Menu) editSpamForm() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, _ storage.Driver) error {
		var cur config.SpamConfig
		if err := m.loadSection(hub, "spam", &cur); err != nil {
			return err
		}
		u := m.ui
		u.Println("=== Spam / Rspamd ===")
		cur.Enabled = u.PromptYesNo("Включить", cur.Enabled)
		cur.Backend = u.Prompt("Backend (rspamd/none)", orDef(cur.Backend, "rspamd"))
		cur.URL = u.Prompt("URL", orDef(cur.URL, "http://127.0.0.1:11333"))
		if pass := u.Prompt("Password (пусто = не менять)", ""); pass != "" {
			cur.Password = pass
		}
		cur.Folder = u.Prompt("Junk folder", orDef(cur.Folder, "Junk"))
		cur.FailOpen = u.PromptYesNo("Fail open", cur.FailOpen)
		cur.FollowRspamd = u.PromptYesNo("Follow Rspamd actions", cur.FollowRspamd)
		if !u.PromptYesNo("Сохранить?", true) {
			u.Println("Отменено.")
			return nil
		}
		return m.putSection(ctx, hub, "spam", cur)
	})
}

func (m *Menu) editScanForm() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, _ storage.Driver) error {
		var cur config.ScanConfig
		if err := m.loadSection(hub, "scan", &cur); err != nil {
			return err
		}
		u := m.ui
		u.Println("=== Антивирус / ICAP ===")
		cur.Enabled = u.PromptYesNo("Включить", cur.Enabled)
		cur.Backend = u.Prompt("Backend (clamav/exec/icap/none)", orDef(cur.Backend, "clamav"))
		cur.Action = u.Prompt("Action (quarantine/reject/tag)", orDef(cur.Action, "quarantine"))
		cur.QuarantineFolder = u.Prompt("Quarantine folder", orDef(cur.QuarantineFolder, "Quarantine"))
		cur.FailOpen = u.PromptYesNo("Fail open", cur.FailOpen)
		switch strings.ToLower(cur.Backend) {
		case "exec":
			cmd := strings.Join(cur.Exec.Command, " ")
			if cmd == "" {
				cmd = "clamdscan --fdpass --no-summary -"
			}
			raw := u.Prompt("Command (space-separated)", cmd)
			cur.Exec.Command = strings.Fields(raw)
		case "icap":
			cur.ICAP.URL = u.Prompt("ICAP URL", orDef(cur.ICAP.URL, "icap://127.0.0.1:1344/reqmod"))
		default:
			cur.ClamAV.Address = u.Prompt("clamd address", orDef(cur.ClamAV.Address, "127.0.0.1:3310"))
		}
		if !u.PromptYesNo("Сохранить?", true) {
			u.Println("Отменено.")
			return nil
		}
		return m.putSection(ctx, hub, "scan", cur)
	})
}

func (m *Menu) editSIEMForm() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, _ storage.Driver) error {
		var cur config.SIEMConfig
		if err := m.loadSection(hub, "siem", &cur); err != nil {
			return err
		}
		u := m.ui
		u.Println("=== SIEM (syslog CEF) ===")
		cur.Enabled = u.PromptYesNo("Включить экспорт в SIEM", cur.Enabled)
		cur.Protocol = u.Prompt("Protocol (udp/tcp/tls)", orDef(cur.Protocol, "udp"))
		cur.Address = u.Prompt("Address host:port", orDef(cur.Address, "127.0.0.1:514"))
		cur.Facility = u.Prompt("Facility", orDef(cur.Facility, "local0"))
		cur.Format = u.Prompt("Format", orDef(cur.Format, "cef"))
		cur.Vendor = u.Prompt("Vendor", orDef(cur.Vendor, "Tayga"))
		cur.Product = u.Prompt("Product", orDef(cur.Product, "TaygaMail"))
		cur.Hostname = u.Prompt("Hostname (пусто = server.hostname)", cur.Hostname)
		cur.TLSSkipVerify = u.PromptYesNo("TLS skip verify (только lab)", cur.TLSSkipVerify)
		cur.QueueSize = promptInt(u, "Queue size", cur.QueueSize, 256)
		if !u.PromptYesNo("Сохранить?", true) {
			u.Println("Отменено.")
			return nil
		}
		return m.putSection(ctx, hub, "siem", cur)
	})
}

func (m *Menu) editLogForm() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, live *config.Config, _ storage.Driver) error {
		var cur config.LogConfig
		if err := m.loadSection(hub, "log", &cur); err != nil {
			return err
		}
		u := m.ui
		u.Println("=== Логирование ===")
		u.Println("Файл логов обязателен. После сохранения нужен перезапуск.")
		cur.Level = u.Prompt("Level (debug/info/warn/error)", orDef(cur.Level, "info"))
		cur.Format = u.Prompt("Format (json/text)", orDef(cur.Format, "json"))
		cur.File = u.Prompt("File path (обязательно)", orDef(cur.File, live.Log.File))
		if strings.TrimSpace(cur.File) == "" {
			return fmt.Errorf("log.file обязателен")
		}
		if !u.PromptYesNo("Сохранить?", true) {
			u.Println("Отменено.")
			return nil
		}
		return m.putSection(ctx, hub, "log", cur)
	})
}

func (m *Menu) editSettingsJSON() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, _ storage.Driver) error {
		sections := settings.EditableSections
		labels := make([]string, len(sections))
		copy(labels, sections)
		n := m.ui.Menu("Секция настроек (JSON)", labels)
		if n == 0 {
			return nil
		}
		name := sections[n-1]
		raw, err := hub.GetSection(name)
		if err != nil {
			return err
		}
		m.ui.Println("--- текущее JSON ---")
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
	})
}

func (m *Menu) editCoS() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, _ *config.Config, store storage.Driver) error {
		tenants, err := store.ListTenants(ctx)
		if err != nil {
			return err
		}
		if len(tenants) == 0 {
			return fmt.Errorf("нет тенантов")
		}
		labels := make([]string, len(tenants))
		for i, t := range tenants {
			labels[i] = t.Name + " (" + t.ID + ")"
		}
		n := m.ui.Menu("Тенант для CoS", labels)
		if n == 0 {
			return nil
		}
		tenant := tenants[n-1]
		for {
			list, err := store.ListServiceClasses(ctx, tenant.ID)
			if err != nil {
				return err
			}
			items := []string{"Создать новый класс"}
			for _, sc := range list {
				items = append(items, sc.Name+" ("+sc.ID+")")
			}
			choice := m.ui.Menu("CoS · "+tenant.Name, items)
			if choice == 0 {
				return nil
			}
			if choice == 1 {
				if err := m.saveCoS(ctx, store, tenant.ID, nil); err != nil {
					m.ui.Printf("Ошибка: %v\n", err)
				}
				continue
			}
			sc := list[choice-2]
			act := m.ui.Menu("Класс "+sc.Name, []string{"Редактировать", "Удалить"})
			switch act {
			case 0:
				continue
			case 1:
				if err := m.saveCoS(ctx, store, tenant.ID, sc); err != nil {
					m.ui.Printf("Ошибка: %v\n", err)
				}
			case 2:
				if m.ui.PromptYesNo("Удалить "+sc.Name+"?", false) {
					if err := store.DeleteServiceClass(ctx, tenant.ID, sc.ID); err != nil {
						m.ui.Printf("Ошибка: %v\n", err)
					} else {
						m.ui.Println("Удалено.")
					}
				}
			}
		}
	})
}

func (m *Menu) saveCoS(ctx context.Context, store storage.Driver, tenantID string, sc *storage.ServiceClass) error {
	u := m.ui
	name := ""
	cfgJSON := `{
  "quota_bytes": 0,
  "max_mail_size": 26214400,
  "large_attach_bytes": 10485760,
  "features": {"files": true, "dav": true, "migration": true}
}`
	if sc != nil {
		name = sc.Name
		if sc.Config != "" {
			cfgJSON = sc.Config
		}
	}
	name = u.Prompt("Имя класса", name)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("имя обязательно")
	}
	u.Println("Config JSON (одна строка или текущий многострочный — введите '.' на пустой строке для конца):")
	u.Println(cfgJSON)
	next := u.Prompt("Новый JSON (пусто = оставить)", "")
	if strings.TrimSpace(next) != "" {
		cfgJSON = next
	}
	if !json.Valid([]byte(cfgJSON)) {
		return fmt.Errorf("невалидный JSON")
	}
	if sc == nil {
		_, err := store.CreateServiceClass(ctx, &storage.ServiceClass{
			TenantID: tenantID,
			Name:     name,
			Config:   cfgJSON,
		})
		if err != nil {
			return err
		}
		u.Println("Создано.")
		return nil
	}
	sc.Name = name
	sc.Config = cfgJSON
	if err := store.UpdateServiceClass(ctx, sc); err != nil {
		return err
	}
	u.Println("Обновлено.")
	return nil
}

func promptInt(u *UI, label string, cur, def int) int {
	if cur <= 0 {
		cur = def
	}
	raw := u.Prompt(label, strconv.Itoa(cur))
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return cur
	}
	return n
}

func orDef(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
