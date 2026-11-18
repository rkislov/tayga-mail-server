package mmsettings

import "fmt"

const TPNSURL = "https://push-test.mattermost.com"

// Push policy inherits per server, tenant and domain; content is always generic.
type PushOverrides struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	ProxyURL *string `json:"proxy_url,omitempty"`
}
type PushEffective struct {
	ProxyURL           string `json:"proxy_url"`
	Prepared           bool   `json:"prepared"`
	Enabled            bool   `json:"enabled"`
	IncludeMessageText bool   `json:"include_message_text"`
}

func validatePush(v *PushOverrides) error {
	if v != nil && v.ProxyURL != nil && *v.ProxyURL != "" && *v.ProxyURL != TPNSURL {
		return fmt.Errorf("only the HTTPS Mattermost TPNS endpoint is supported for this test")
	}
	return nil
}
