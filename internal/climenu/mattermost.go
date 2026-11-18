package climenu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
)

func (u *UI) mattermostOverride(label string, current *bool) *bool {
	value := "наследовать"
	if current != nil {
		value = "выключено"
		if *current {
			value = "включено"
		}
	}
	u.Printf("%s: %s\n", label, value)
	switch u.Menu(label, []string{"Наследовать", "Включить", "Выключить"}) {
	case 1:
		return nil
	case 2:
		v := true
		return &v
	case 3:
		v := false
		return &v
	}
	return current
}
func (m *Menu) editMattermost() error {
	return m.withHub(func(ctx context.Context, hub *settings.Hub, cfg *config.Config, store storage.Driver) error {
		level := m.ui.Menu("Mattermost", []string{"Запуск API и хоста плагинов (сервер)", "Настройки сервера по умолчанию", "Настройки тенанта", "Настройки домена"})
		if level == 0 {
			return nil
		}
		if level == 1 {
			var cur config.MattermostConfig
			if err := m.loadSection(hub, "mattermost", &cur); err != nil {
				return err
			}
			cur.Enabled = m.ui.PromptYesNo("API Mattermost", cur.Enabled)
			cur.PluginsEnabled = m.ui.PromptYesNo("Хост серверных плагинов", cur.PluginsEnabled)
			cur.PluginDirectory = m.ui.Prompt("Каталог пакетов плагинов (пусто = автоматически)", cur.PluginDirectory)
			if !m.ui.PromptYesNo("Сохранить? Требуется перезапуск.", true) {
				return nil
			}
			return m.putSection(ctx, hub, "mattermost", cur)
		}
		scope := mmsettings.Scope{}
		if level >= 3 {
			tenants, err := store.ListTenants(ctx)
			if err != nil {
				return err
			}
			labels := []string{}
			for _, t := range tenants {
				labels = append(labels, t.Name)
			}
			if len(labels) == 0 {
				return fmt.Errorf("нет тенантов")
			}
			n := m.ui.Menu("Тенант", labels)
			if n == 0 {
				return nil
			}
			scope.TenantID = tenants[n-1].ID
			if level == 4 {
				domains, err := store.ListDomainsByTenant(ctx, scope.TenantID)
				if err != nil {
					return err
				}
				labels = []string{}
				for _, d := range domains {
					labels = append(labels, d.Name)
				}
				if len(labels) == 0 {
					return fmt.Errorf("нет доменов")
				}
				n = m.ui.Menu("Домен", labels)
				if n == 0 {
					return nil
				}
				scope.DomainID = domains[n-1].ID
			}
		}
		service := &mmsettings.Service{Store: store, Defaults: mmsettings.Defaults{Enabled: cfg.Mattermost.Enabled, PluginsEnabled: cfg.Mattermost.PluginsEnabled, SiteName: "Tayga Chat"}}
		value, err := service.Read(ctx, scope)
		if err != nil {
			return err
		}
		effective, err := service.Resolve(ctx, scope)
		if err != nil {
			return err
		}
		m.ui.Printf("Область: %s. Итог: API=%v, плагины=%v, имя=%s\n", scope.Key(), effective.Enabled, effective.PluginsEnabled, effective.SiteName)
		value.Enabled = m.ui.mattermostOverride("Доступ к Mattermost", value.Enabled)
		value.PluginsEnabled = m.ui.mattermostOverride("Серверные плагины", value.PluginsEnabled)
		name := ""
		if value.SiteName != nil {
			name = *value.SiteName
		}
		inherit := m.ui.PromptYesNo("Наследовать название сервера", value.SiteName == nil)
		if inherit {
			value.SiteName = nil
		} else {
			next := m.ui.Prompt("Название", name)
			value.SiteName = &next
		}
		safe, _ := json.Marshal(mmsettings.Redact(value.Plugins))
		m.ui.Printf("Параметры плагинов этой области (секреты скрыты): %s\n", safe)
		raw := m.ui.Prompt("Плагины JSON (пусто = без изменений; {} = наследовать всё)", "")
		if strings.TrimSpace(raw) != "" {
			if err = json.Unmarshal([]byte(raw), &value.Plugins); err != nil {
				return err
			}
		}
		if !m.ui.PromptYesNo("Сохранить настройки выбранной области?", true) {
			return nil
		}
		if err = service.Save(ctx, scope, value); err != nil {
			return err
		}
		m.ui.Println("Настройки Mattermost сохранены. Области других тенантов не изменены.")
		return nil
	})
}
