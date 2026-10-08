package settings

import (
	"embed"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/tayga/tms/internal/config"
)

//go:embed field_help.json
var helpFiles embed.FS

type FieldSchema struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Kind        string        `json:"kind"`
	Fields      []FieldSchema `json:"fields,omitempty"`
	Item        *FieldSchema  `json:"item,omitempty"`
	Options     []string      `json:"options,omitempty"`
	ReadOnly    bool          `json:"read_only,omitempty"`
	Secret      bool          `json:"secret,omitempty"`
	Default     any           `json:"default,omitempty"`
}

type fieldHelp struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

var fieldCatalog = func() map[string]fieldHelp {
	raw, err := helpFiles.ReadFile("field_help.json")
	if err != nil {
		panic(err)
	}
	var catalog map[string]fieldHelp
	if err := json.Unmarshal(raw, &catalog); err != nil {
		panic(err)
	}
	return catalog
}()

// ConfigSchema follows the actual Go config, including fields of empty maps and lists.
// Storage and mailstore.root remain visible, but cannot be changed through this API.
func ConfigSchema() map[string]FieldSchema {
	out := map[string]FieldSchema{}
	v := reflect.ValueOf(config.Default()).Elem()
	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Tag.Get("yaml")
		out[key] = schemaField(key, key, v.Field(i), key == "storage")
	}
	return out
}

func schemaField(key, path string, value reflect.Value, locked bool) FieldSchema {
	help := fieldCatalog[key]
	if specific, ok := fieldCatalog[path]; ok {
		help = specific
	}
	f := FieldSchema{Key: key, Label: help.Label, Description: help.Description, ReadOnly: locked || path == "mailstore.root"}
	if f.Label == "" {
		f.Label = key
	}
	if value.Type() == reflect.TypeOf(time.Duration(0)) {
		f.Kind = "duration"
		f.Default = time.Duration(value.Int()).String()
		return f
	}
	switch value.Kind() {
	case reflect.Struct:
		f.Kind = "object"
		for i := 0; i < value.NumField(); i++ {
			childKey := value.Type().Field(i).Tag.Get("yaml")
			if childKey == "" || childKey == "-" {
				continue
			}
			f.Fields = append(f.Fields, schemaField(childKey, path+"."+childKey, value.Field(i), f.ReadOnly))
		}
	case reflect.Map:
		f.Kind = "map"
		item := schemaField(key, path+".*", reflect.Zero(value.Type().Elem()), f.ReadOnly)
		f.Item = &item
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.String {
			f.Kind = "strings"
		} else {
			f.Kind = "array"
			item := schemaField(key, path+".*", reflect.Zero(value.Type().Elem()), f.ReadOnly)
			f.Item = &item
		}
	case reflect.Bool:
		f.Kind = "boolean"
		f.Default = value.Bool()
	case reflect.Int, reflect.Int64:
		f.Kind = "integer"
		f.Default = value.Int()
	case reflect.Float64:
		f.Kind = "number"
		f.Default = value.Float()
	default:
		f.Kind = "string"
		f.Default = value.String()
	}
	f.Secret = key == "password" || key == "secret" || key == "secret_key" || key == "access_key" || key == "client_secret" || key == "bind_password" || key == "secrets_key" || key == "dsn"
	if f.Secret {
		f.Default = ""
	}
	f.Options = schemaOptions(path)
	return f
}

func schemaOptions(path string) []string {
	switch path {
	case "spam.backend":
		return []string{"none", "rspamd"}
	case "scan.backend":
		return []string{"none", "clamav", "exec", "icap"}
	case "scan.action":
		return []string{"reject", "quarantine", "tag"}
	case "dmarc.action":
		return []string{"tag", "reject", "follow"}
	case "ha.mode":
		return []string{"none", "active_standby", "sticky"}
	case "ha.fence":
		return []string{"mx", "writers"}
	case "smtp.mta_sts.publish.mode":
		return []string{"none", "testing", "enforce"}
	case "ldap.domains.*.groups.mode":
		return []string{"off", "memberof", "search"}
	case "ldap.domains.*.groups.nested_mode":
		return []string{"walk", "chain"}
	case "siem.protocol":
		return []string{"udp", "tcp", "tls"}
	case "siem.format":
		return []string{"cef"}
	case "log.level":
		return []string{"debug", "info", "warn", "error"}
	case "log.format":
		return []string{"json", "text"}
	case "storage.driver":
		return []string{"sqlite", "postgres"}
	}
	if strings.HasSuffix(path, ".action") {
		return []string{"tag", "reject"}
	}
	return nil
}
