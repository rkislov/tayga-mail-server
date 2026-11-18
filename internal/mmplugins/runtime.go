// Package mmplugins hosts Mattermost server plugins through the official RPC SDK.
// The host API is deliberately incremental: unsupported methods fail explicitly.
package mmplugins

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

var ErrInvalid = errors.New("invalid plugin bundle or request")
var pluginID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,189}$`)

const Version = "10.11.0"

type Options struct {
	LoadConfiguration func(string) (map[string]any, error)
	SaveConfiguration func(string, map[string]any) error
	ReadEnabled       func(string) (bool, error)
	WriteEnabled      func(string, bool) error

	TenantID, DomainID, Scope string
	Admins                    []string
	Directory, SiteURL        string
	Store                     storage.Driver
	Chat                      *chat.Service
	Logger                    *slog.Logger
}
type Runtime struct {
	botMu     sync.Mutex
	options   Options
	env       *plugin.Environment
	logger    *mlog.Logger
	mu        sync.Mutex
	kvMu      sync.Mutex
	commandMu sync.RWMutex
	commands  map[string]registeredCommand
	closed    bool
}
type registeredCommand struct {
	PluginID string
	Command  *model.Command
}
type metrics struct{}

func (metrics) ObservePluginHookDuration(string, string, bool, float64) {}
func (metrics) ObservePluginMultiHookIterationDuration(string, float64) {}
func (metrics) ObservePluginMultiHookDuration(float64)                  {}
func (metrics) ObservePluginAPIDuration(string, string, bool, float64)  {}
func New(options Options) (*Runtime, error) {
	if options.Directory == "" || options.Store == nil || options.Chat == nil {
		return nil, ErrInvalid
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if err := os.MkdirAll(options.Directory, 0700); err != nil {
		return nil, err
	}
	web := filepath.Join(options.Directory, ".webapp", chat.ID(options.Scope))
	if err := os.MkdirAll(web, 0700); err != nil {
		return nil, err
	}
	logger, err := mlog.NewLogger()
	if err != nil {
		return nil, err
	}
	r := &Runtime{options: options, logger: logger, commands: map[string]registeredCommand{}}
	r.env, err = plugin.NewEnvironment(func(m *model.Manifest) plugin.API {
		return &hostAPI{unsupportedAPI: &unsupportedAPI{log: options.Logger, pluginID: m.Id}, runtime: r, manifest: m}
	}, nil, options.Directory, web, logger, metrics{})
	if err != nil {
		return nil, err
	}
	available, err := r.env.Available()
	if err != nil {
		return nil, err
	}
	for _, bundle := range available {
		if options.Scope == "catalog" {
			continue
		}
		enabled, err := r.enabled(bundle.Manifest.Id)
		if err != nil {
			r.Close()
			return nil, err
		}
		if enabled {
			if _, err = r.Enable(bundle.Manifest.Id); err != nil {
				options.Logger.Error("Mattermost plugin activation failed", "plugin_id", bundle.Manifest.Id, "scope", options.Scope, "error", err)
			}
		}
	}

	return r, nil
}
func (r *Runtime) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.env.Shutdown()
	_ = r.logger.Shutdown()
}
func (r *Runtime) List() (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	available, err := r.env.Available()
	if err != nil {
		return nil, err
	}
	active, inactive := []*model.Manifest{}, []*model.Manifest{}
	for _, b := range available {
		if r.env.IsActive(b.Manifest.Id) {
			active = append(active, b.Manifest)
		} else {
			inactive = append(inactive, b.Manifest)
		}
	}
	return map[string]any{"active": active, "inactive": inactive}, nil
}
func (r *Runtime) Webapp() []*model.Manifest {
	result := []*model.Manifest{}
	for _, b := range r.env.Active() {
		if b.Manifest.HasWebapp() {
			result = append(result, b.Manifest.ClientManifest())
		}
	}
	return result
}
func (r *Runtime) Statuses() (model.PluginStatuses, error) { return r.env.Statuses() }
func (r *Runtime) Enable(id string) (*model.Manifest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !pluginID.MatchString(id) || r.closed || r.options.Scope == "catalog" {
		return nil, ErrInvalid
	}
	m, err := r.env.GetManifest(id)
	if err != nil {
		return nil, err
	}
	matches, err := m.MeetMinServerVersion(Version)
	if err != nil {
		return nil, err
	}
	if !matches {
		return nil, fmt.Errorf("plugin requires Mattermost %s; host API target is %s", m.MinServerVersion, Version)
	}
	m, _, err = r.env.Activate(id)
	if err != nil {
		return nil, err
	}
	if err = r.writeEnabled(id, true); err != nil {
		r.env.Deactivate(id)
		return nil, err
	}
	return m, nil
}
func (r *Runtime) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !pluginID.MatchString(id) || r.closed {
		return ErrInvalid
	}
	if _, err := r.env.GetManifest(id); err != nil {
		return err
	}
	if err := r.writeEnabled(id, false); err != nil {
		return err
	}
	r.env.Deactivate(id)
	r.commandMu.Lock()
	defer r.commandMu.Unlock()
	for key, c := range r.commands {
		if c.PluginID == id {
			delete(r.commands, key)
		}
	}
	return nil
}
func (r *Runtime) Remove(id string) error {
	if err := r.Disable(id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.env.RemovePlugin(id)
	return os.RemoveAll(filepath.Join(r.options.Directory, id))
}
func (r *Runtime) Install(reader io.Reader, force bool) (*model.Manifest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrInvalid
	}
	temp, err := os.MkdirTemp(r.options.Directory, ".upload-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	extracted := filepath.Join(temp, "bundle")
	if err = os.Mkdir(extracted, 0700); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(io.LimitReader(reader, 100<<20))
	if err != nil {
		return nil, ErrInvalid
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	entries := 0
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		entries++
		total += h.Size
		if entries > 2500 || h.Size < 0 || total > 256<<20 || h.Size > 100<<20 {
			return nil, ErrInvalid
		}
		clean := filepath.Clean(h.Name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(h.Name, "\\") {
			return nil, ErrInvalid
		}
		target := filepath.Join(extracted, clean)
		switch h.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0700); err != nil {
				return nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return nil, err
			}
			mode := os.FileMode(0600)
			if h.Mode&0111 != 0 {
				mode = 0700
			}
			f, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if e != nil {
				return nil, e
			}
			_, e = io.CopyN(f, tr, h.Size)
			closeErr := f.Close()
			if e != nil {
				return nil, e
			}
			if closeErr != nil {
				return nil, closeErr
			}
		default:
			return nil, ErrInvalid
		}
	}
	bundle := extracted
	if _, err = os.Stat(filepath.Join(bundle, "plugin.json")); err != nil {
		entries, e := os.ReadDir(extracted)
		if e != nil || len(entries) != 1 || !entries[0].IsDir() {
			return nil, ErrInvalid
		}
		bundle = filepath.Join(extracted, entries[0].Name())
	}
	raw, err := os.ReadFile(filepath.Join(bundle, "plugin.json"))
	if err != nil {
		return nil, ErrInvalid
	}
	var m model.Manifest
	if err = json.Unmarshal(raw, &m); err != nil || !pluginID.MatchString(m.Id) {
		return nil, ErrInvalid
	}
	if err = m.IsValid(); err != nil {
		return nil, err
	}
	dest := filepath.Join(r.options.Directory, m.Id)
	if _, err = os.Stat(dest); err == nil {
		if !force || r.env.IsActive(m.Id) {
			return nil, fmt.Errorf("plugin already installed; disable it before replacing")
		}
		old := filepath.Join(temp, ".replaced")
		if err = os.Rename(dest, old); err != nil {
			return nil, err
		}
		if err = os.Rename(bundle, dest); err != nil {
			_ = os.Rename(old, dest)
			return nil, err
		}
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err = os.Rename(bundle, dest); err != nil {
			return nil, err
		}
	}
	return &m, nil
}
func (r *Runtime) ServeHTTP(w http.ResponseWriter, request *http.Request, userID string) {
	parts := strings.SplitN(strings.TrimPrefix(request.URL.Path, "/plugins/"), "/", 2)
	if len(parts) != 2 || !pluginID.MatchString(parts[0]) {
		http.NotFound(w, request)
		return
	}
	hooks, err := r.env.HooksForPlugin(parts[0])
	if err != nil {
		http.NotFound(w, request)
		return
	}
	req := request.Clone(request.Context())
	req.Header = request.Header.Clone()
	req.Header.Del("Mattermost-User-Id")
	req.Header.Del("Mattermost-Session-Id")
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	if userID != "" {
		req.Header.Set("Mattermost-User-Id", userID)
	}
	req.URL.Path = "/" + parts[1]
	req.URL.RawPath = ""
	req.Body = http.MaxBytesReader(w, req.Body, 2<<20)
	hooks.ServeHTTP(&plugin.Context{}, w, req)
}
func (r *Runtime) Commands(teamID string) []*model.Command {
	r.commandMu.RLock()
	defer r.commandMu.RUnlock()
	out := []*model.Command{}
	for _, c := range r.commands {
		if r.env.IsActive(c.PluginID) && (c.Command.TeamId == "" || c.Command.TeamId == teamID) {
			out = append(out, c.Command)
		}
	}
	return out
}
func (r *Runtime) Execute(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	words := strings.Fields(args.Command)
	if len(words) == 0 {
		return nil, appError(ErrInvalid)
	}
	trigger := strings.TrimPrefix(words[0], "/")
	r.commandMu.RLock()
	c, ok := r.commands[args.TeamId+":"+trigger]
	if !ok {
		c, ok = r.commands[":"+trigger]
	}
	r.commandMu.RUnlock()
	if !ok {
		return nil, model.NewAppError("command", "tms.command.not_found", nil, "Unknown command", 404)
	}
	hooks, err := r.env.HooksForPluginWithRPCErr(c.PluginID)
	if err != nil {
		return nil, appError(err)
	}
	result, appErr, rpcErr := hooks.ExecuteCommandWithRPCErr(&plugin.Context{}, args)
	if rpcErr != nil {
		return nil, appError(rpcErr)
	}
	return result, appErr
}
func appError(err error) *model.AppError {
	if err == nil {
		return nil
	}
	return model.NewAppError("TMSPluginAPI", "tms.plugin.api_error", nil, err.Error(), 500)
}

func (r *Runtime) Configure(id string, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !pluginID.MatchString(id) {
		return ErrInvalid
	}
	if _, err := r.env.GetManifest(id); err != nil {
		return err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return ErrInvalid
	}
	if err = r.writeConfig(id, config); err != nil {
		return err
	}
	if r.env.IsActive(id) {
		hooks, err := r.env.HooksForPluginWithRPCErr(id)
		if err != nil {
			return err
		}
		appErr, rpcErr := hooks.OnConfigurationChangeWithRPCErr()
		if rpcErr != nil {
			return rpcErr
		}
		if appErr != nil {
			return appErr
		}
	}
	return nil
}

func (r *Runtime) stateKey(kind, id string) string {
	return "mmplugin." + kind + ":" + r.options.Scope + ":" + id
}
func (r *Runtime) enabled(id string) (bool, error) {
	if r.options.ReadEnabled != nil {
		return r.options.ReadEnabled(id)
	}
	raw, ok, err := r.options.Store.GetSetting(context.Background(), r.stateKey("enabled", id))
	return ok && raw == "true", err
}
func (r *Runtime) writeEnabled(id string, value bool) error {
	if r.options.WriteEnabled != nil {
		return r.options.WriteEnabled(id, value)
	}
	return r.options.Store.PutSetting(context.Background(), r.stateKey("enabled", id), strconv.FormatBool(value))
}
func (r *Runtime) readConfig(id string) (map[string]any, error) {
	if r.options.LoadConfiguration != nil {
		return r.options.LoadConfiguration(id)
	}
	raw, ok, err := r.options.Store.GetSetting(context.Background(), r.stateKey("config", id))
	if err != nil || !ok {
		return map[string]any{}, err
	}
	var config map[string]any
	err = json.Unmarshal([]byte(raw), &config)
	return config, err
}
func (r *Runtime) writeConfig(id string, config map[string]any) error {
	if r.options.SaveConfiguration != nil {
		return r.options.SaveConfiguration(id, config)
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return r.options.Store.PutSetting(context.Background(), r.stateKey("config", id), string(raw))
}
