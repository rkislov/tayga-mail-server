package mattermost

import (
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
)

func (s *API) patchDomainPluginConfiguration(w http.ResponseWriter, r *http.Request, scope mmsettings.Scope, path string) bool {
	if r.Method != http.MethodPatch {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/plugins/"), "/")
	if len(parts) != 2 || parts[1] != "configuration" {
		return false
	}
	if scope.DomainID == "" {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	if _, err := s.plugins.Manifest(parts[0]); err != nil {
		fail(w, 404, "tms.plugin.not_installed", "Plugin package is not installed")
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var config struct {
		Set     map[string]any `json:"set"`
		Inherit []string       `json:"inherit"`
	}
	if !decode(w, r, &config) {
		return true
	}
	if err := s.policy.PatchPluginConfig(r.Context(), scope, parts[0], config.Set, config.Inherit); err != nil {
		fail(w, 400, "tms.plugin.configuration_failed", "Invalid configuration for the selected domain")
		return true
	}
	write(w, 200, map[string]string{"status": "OK"})
	return true
}
