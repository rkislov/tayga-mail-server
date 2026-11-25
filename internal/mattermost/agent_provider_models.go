package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (p *agentProvider) retainKey(stored []agentProvider) {
	if p.APIKey != "" {
		return
	}
	for _, previous := range stored {
		if previous.ID == p.ID && previous.URL == p.URL && previous.Kind == p.Kind {
			p.APIKey = previous.APIKey
			return
		}
	}
}

func newAgentProviderClient(p agentProvider, timeout time.Duration) (*http.Client, error) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if relay := os.Getenv("TAYGA_OPENAI_RELAY"); p.Kind == "openai" && relay != "" {
		transport, err := openAITransport(relay)
		if err != nil {
			return nil, err
		}
		client.Transport = transport
	}
	return client, nil
}

// Choose a fixed destination or one supplied by the server operator. In
// particular, the draft URL from the browser is never used to make a request.
func agentProviderCatalogEndpoint(p agentProvider) string {
	base := strings.TrimRight(p.URL, "/")
	suffix := "/models"
	switch p.Kind {
	case "openai":
		if base == "https://api.openai.com/v1" {
			return "https://api.openai.com/v1/models"
		}
		return ""
	case "deepseek":
		if base == "https://api.deepseek.com" || base == "https://api.deepseek.com/v1" {
			return "https://api.deepseek.com/models"
		}
		return ""
	case "ollama":
		base = strings.TrimSuffix(base, "/api")
		suffix = "/api/tags"
		switch base {
		case "http://127.0.0.1:11434", "http://localhost:11434":
			return "http://127.0.0.1:11434/api/tags"
		case "http://[::1]:11434":
			return "http://[::1]:11434/api/tags"
		}
	}
	for _, configured := range strings.Split(os.Getenv("TAYGA_AGENT_MODEL_CATALOG_URLS"), ",") {
		approved := strings.TrimRight(strings.TrimSpace(configured), "/")
		if p.Kind == "ollama" {
			approved = strings.TrimSuffix(approved, "/api")
		}
		if approved != "" && base == approved && validAgentProviderEndpoint(agentProvider{Kind: p.Kind, URL: approved}) {
			return approved + suffix
		}
	}
	return ""
}

func (s *API) agentProviderModels(w http.ResponseWriter, r *http.Request, u *storage.User) {
	if r.Method != http.MethodPost {
		wrongMethod(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var p agentProvider
	if !decode(w, r, &p) {
		return
	}
	if (p.ID != "" && !validAgentID(p.ID)) || len(p.APIKey) > 8192 || len(p.URL) > 2048 || !validAgentProviderEndpoint(p) {
		fail(w, 400, "tms.agents.provider_settings", "Проверьте адрес API и тип провайдера.")
		return
	}
	endpoint := agentProviderCatalogEndpoint(p)
	if endpoint == "" {
		fail(w, 403, "tms.agents.provider_catalog_url", "Администратор сервера должен разрешить этот URL для каталога моделей. Идентификатор можно ввести вручную.")
		return
	}
	if p.ID != "" && p.APIKey == "" {
		c := emptyAgentConfig()
		if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
			s.failure(w, err)
			return
		}
		p.retainKey(c.Providers)
	}
	if (p.Kind == "openai" || p.Kind == "deepseek") && strings.TrimSpace(p.APIKey) == "" {
		fail(w, 400, "tms.agents.provider_key_required", "Введите API-ключ провайдера.")
		return
	}
	models, err := listAgentProviderModels(r.Context(), p, endpoint)
	if err != nil {
		message := "Не удалось получить список моделей. Проверьте адрес API и доступность провайдера."
		var rejection agentProviderError
		if errors.As(err, &rejection) {
			message = rejection.message()
			if rejection.status == http.StatusNotFound {
				message = "Провайдер не вернул каталог моделей (HTTP 404). Проверьте базовый URL или введите идентификатор модели вручную."
			}
		}
		fail(w, 502, "tms.agents.provider_models", message)
		return
	}
	write(w, 200, map[string]any{"models": models})
}

func listAgentProviderModels(ctx context.Context, p agentProvider, endpoint string) ([]string, error) {
	const maxResponse = 2 << 20
	client, err := newAgentProviderClient(p, 12*time.Second)
	if err != nil {
		return nil, err
	}
	if client.Transport != nil {
		defer client.CloseIdleConnections()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, readAgentProviderError(res)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return nil, errors.New("invalid provider catalog size")
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Model string `json:"model"`
			Name  string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	models := []string{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && len(id) <= 200 && (p.APIKey == "" || !strings.Contains(id, p.APIKey)) {
			models = append(models, id)
		}
	}
	if p.Kind == "ollama" && response.Models != nil {
		for _, entry := range response.Models {
			if entry.Model != "" {
				add(entry.Model)
			} else {
				add(entry.Name)
			}
		}
	} else if p.Kind != "ollama" && response.Data != nil {
		for _, entry := range response.Data {
			add(entry.ID)
		}
	} else {
		return nil, errors.New("provider catalog missing")
	}
	slices.Sort(models)
	return slices.Compact(models), nil
}
