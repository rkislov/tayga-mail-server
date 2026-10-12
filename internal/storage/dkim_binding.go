package storage

import (
	"context"
	"encoding/json"
)

// DKIMBinding stores only metadata; the private PEM stays in a mode-0600 file.
type DKIMBinding struct {
	Domain   string `json:"domain"`
	Selector string `json:"selector"`
	KeyFile  string `json:"key_file"`
	DNSValue string `json:"dns_value"`
	Enabled  bool   `json:"enabled"`
}

func ReadDKIMBinding(ctx context.Context, store Driver, domain string) (*DKIMBinding, error) {
	raw, ok, err := store.GetSetting(ctx, "dkim.domain."+domain)
	if err != nil || !ok {
		return nil, err
	}
	var binding DKIMBinding
	if err := json.Unmarshal([]byte(raw), &binding); err != nil {
		return nil, err
	}
	return &binding, nil
}
