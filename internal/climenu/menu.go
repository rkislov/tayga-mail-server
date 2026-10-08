package climenu

import (
	"fmt"

	"github.com/tayga/tms/internal/version"
)

// Menu is the interactive ops console.
type Menu struct {
	ui      *UI
	cfgPath string
}

// Run starts the top-level console loop. cfgPath is the default -config path.
func Run(cfgPath string) error {
	if cfgPath == "" {
		cfgPath = "configs/tayga.example.yaml"
	}
	m := &Menu{ui: NewUI(), cfgPath: cfgPath}
	m.ui.Printf("Tayga Mail console · %s\n", version.String())
	m.ui.Printf("config: %s\n", m.cfgPath)

	for {
		n := m.ui.Menu("Главное меню", []string{
			"Первичная настройка (wizard → YAML)",
			"Показать текущий bootstrap",
			"Сменить путь к config",
			"Backup (.tar.gz)",
			"Restore из .tar.gz",
			"Перенос БД / хранилища (sqlite ↔ postgres)",
			"Настройки сервера (SMTP / spam / AV / SIEM / log / CoS)",
			"О версии",
		})
		var err error
		switch n {
		case 0:
			m.ui.Println("Выход.")
			return nil
		case 1:
			err = m.wizardSetup()
		case 2:
			err = m.showConfigSummary()
		case 3:
			m.cfgPath = m.ui.Prompt("Путь к config", m.cfgPath)
		case 4:
			err = m.doBackup()
		case 5:
			err = m.doRestore()
		case 6:
			err = m.transferStorage()
		case 7:
			err = m.editSettingsSection()
		case 8:
			m.ui.Println(version.String())
		}
		if err != nil {
			m.ui.Printf("Ошибка: %v\n", err)
		}
	}
}

// ErrNotTTY is unused placeholder for future non-interactive detection.
var ErrNotTTY = fmt.Errorf("interactive menu requires a terminal")
