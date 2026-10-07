package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// Hub merges bootstrap YAML with DB settings and exposes a live config pointer.
type Hub struct {
	store     storage.Driver
	bootstrap *config.Config

	live atomic.Pointer[config.Config]

	mu              sync.Mutex
	restartRequired bool
}

func NewHub(store storage.Driver, bootstrap *config.Config) *Hub {
	h := &Hub{store: store, bootstrap: cloneConfig(bootstrap)}
	h.live.Store(cloneConfig(bootstrap))
	return h
}

// Load merges DB settings onto bootstrap and stores the result.
func (h *Hub) Load(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("nil hub")
	}
	cfg, err := h.merge(ctx)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("merged config: %w", err)
	}
	h.live.Store(cfg)
	return nil
}

func (h *Hub) Config() *config.Config {
	if h == nil {
		return nil
	}
	return h.live.Load()
}

func (h *Hub) RestartRequired() bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.restartRequired
}

func (h *Hub) markRestart(section string) {
	if RequiresRestart(section) {
		h.mu.Lock()
		h.restartRequired = true
		h.mu.Unlock()
	}
}

func (h *Hub) merge(ctx context.Context) (*config.Config, error) {
	cfg := cloneConfig(h.bootstrap)
	preserveBootstrap(h.bootstrap, cfg)
	rows, err := h.store.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	for _, section := range EditableSections {
		raw, ok := rows[section]
		if !ok || raw == "" {
			continue
		}
		ptr, err := getSectionPtr(cfg, section)
		if err != nil {
			return nil, err
		}
		if err := fromJSONBytes([]byte(raw), ptr); err != nil {
			return nil, fmt.Errorf("settings %s: %w", section, err)
		}
	}
	preserveBootstrap(h.bootstrap, cfg)
	return cfg, nil
}

// GetSection returns a redacted JSON object for one section.
func (h *Hub) GetSection(section string) (json.RawMessage, error) {
	if !isEditable(section) {
		return nil, fmt.Errorf("unknown or locked section %q", section)
	}
	cfg := cloneConfig(h.Config())
	redactConfig(cfg)
	ptr, err := getSectionPtr(cfg, section)
	if err != nil {
		return nil, err
	}
	m, err := toJSONMap(ptr)
	if err != nil {
		return nil, err
	}
	return mapToJSON(m)
}

// ListAll returns redacted sections plus meta.
func (h *Hub) ListAll() (map[string]any, error) {
	cfg := cloneConfig(h.Config())
	redactConfig(cfg)
	sections := map[string]any{}
	for _, name := range EditableSections {
		ptr, err := getSectionPtr(cfg, name)
		if err != nil {
			return nil, err
		}
		m, err := toJSONMap(ptr)
		if err != nil {
			return nil, err
		}
		sections[name] = m
	}
	return map[string]any{
		"sections":         EditableSections,
		"settings":         sections,
		"restart_required": h.RestartRequired(),
		"locked":           []string{"storage", "mailstore.root"},
	}, nil
}

// PutSection validates and persists a section, then reloads the live config.
func (h *Hub) PutSection(ctx context.Context, section string, body []byte) error {
	if !isEditable(section) {
		return fmt.Errorf("unknown or locked section %q", section)
	}
	cur := h.Config()
	next := cloneConfig(cur)
	preserveBootstrap(h.bootstrap, next)

	ptr, err := getSectionPtr(next, section)
	if err != nil {
		return err
	}
	if err := fromJSONBytes(body, ptr); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	preserveSecrets(cur, next)
	preserveBootstrap(h.bootstrap, next)

	if err := next.Validate(); err != nil {
		return err
	}

	// Persist only this section (from next, with real secrets).
	secPtr, _ := getSectionPtr(next, section)
	m, err := toJSONMap(secPtr)
	if err != nil {
		return err
	}
	raw, err := mapToJSON(m)
	if err != nil {
		return err
	}
	if err := h.store.PutSetting(ctx, section, string(raw)); err != nil {
		return err
	}

	if err := h.Load(ctx); err != nil {
		return err
	}
	h.markRestart(section)
	return nil
}
