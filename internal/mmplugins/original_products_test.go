package mmplugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func TestOriginalProductBundles(t *testing.T) {
	dir := os.Getenv("TMS_TEST_ORIGINAL_PLUGINS")
	dsn := os.Getenv("TMS_TEST_PLUGIN_POSTGRES")
	if dir == "" || dsn == "" {
		t.Skip("set verified bundle directory and isolated PostgreSQL DSN")
	}
	parsed, e := url.Parse(dsn)
	if e != nil || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		t.Fatal("isolated localhost PostgreSQL required")
	}
	for _, name := range []string{"boards", "playbooks"} {
		t.Run(name, func(t *testing.T) {
			version, id := "10.1.0", "focalboard"
			if name == "playbooks" {
				version, id = "2.9.7", "playbooks"
			}
			pins := map[string]string{
				"boards/darwin-arm64":   "7c8aa587ebb42038d89e5aaee89730a277b9804f6c6aade24b33af778c632e0d",
				"boards/linux-arm64":    "9169c4ff4aa27ace8461df2e57e5250562a05eddcc40f0a2e86ecbb029c1a4cc",
				"boards/linux-amd64":    "26382259874a1aa297150ab767f13bf48400387f1f70daed454d6ed1b69c71f7",
				"playbooks/linux-arm64": "23c983dda20c2335e2f5e5d1d0b3f7a5a904d7489d54c701e1c0323f8fda4ded",
				"playbooks/linux-amd64": "337b165b4aaf3a8201adbe67e9cbc0f78d2a97f88a83c06481a800d759f4510c",
			}
			expected := pins[name+"/"+runtime.GOOS+"-"+runtime.GOARCH]
			path := filepath.Join(dir, fmt.Sprintf("mattermost-plugin-%s-v%s-%s-%s.tar.gz", name, version, runtime.GOOS, runtime.GOARCH))
			if name == "playbooks" && os.Getenv("TMS_TEST_PLAYBOOKS_FORK") != "" {
				path = os.Getenv("TMS_TEST_PLAYBOOKS_FORK")
				expected = os.Getenv("TMS_TEST_PLAYBOOKS_FORK_SHA256")
				if len(expected) != 64 {
					t.Fatal("fork bundle requires SHA-256 pin")
				}
			}
			if expected == "" {
				t.Skip("no published native original bundle")
			}
			data, e := os.ReadFile(path)
			if os.IsNotExist(e) {
				t.Fatal("required pinned bundle missing", path)
			}
			if e != nil {
				t.Fatal(e)
			}
			digest := sha256.Sum256(data)
			if expected == "" || hex.EncodeToString(digest[:]) != expected {
				t.Fatal("original bundle hash mismatch")
			}
			opts := testOptions(t)
			ctx := context.Background()
			tenant, e := opts.Store.CreateTenant(ctx, "Original products")
			if e != nil {
				t.Fatal(e)
			}
			domain, e := opts.Store.CreateDomain(ctx, tenant.ID, "fixture.example.test")
			if e != nil {
				t.Fatal(e)
			}
			u, e := opts.Store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "owner@fixture.example.test", LocalPart: "owner", DisplayName: "Owner", Enabled: true})
			if e != nil {
				t.Fatal(e)
			}
			siblingDomain, e := opts.Store.CreateDomain(ctx, tenant.ID, "sibling.example.test")
			if e != nil {
				t.Fatal(e)
			}
			sibling, e := opts.Store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: siblingDomain.ID, Email: "sibling@sibling.example.test", LocalPart: "sibling", Enabled: true})
			if e != nil {
				t.Fatal(e)
			}
			opts.Scope = "domain:" + tenant.ID + ":" + domain.ID
			opts.TenantID, opts.DomainID = tenant.ID, domain.ID
			opts.SQLDriverName = "postgres"
			opts.Development = name == "playbooks" && os.Getenv("TMS_TEST_PLAYBOOKS_FORK") == ""
			var logs bytes.Buffer
			opts.Logger = slog.New(slog.NewTextHandler(&logs, nil))
			t.Cleanup(func() { t.Log(logs.String()) })
			admin, e := sql.Open("postgres", dsn)
			if e != nil {
				t.Fatal(e)
			}
			defer admin.Close()
			dbName := ProductDatabaseName(opts.Scope, id)
			password := strings.ReplaceAll(uuid.NewString(), "-", "")
			if _, e = admin.Exec(`CREATE ROLE "` + dbName + `" LOGIN PASSWORD '` + password + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`); e != nil {
				t.Fatal(e)
			}
			defer func() { _, _ = admin.Exec(`DROP ROLE "` + dbName + `"`) }()
			if _, e = admin.Exec(`CREATE DATABASE "` + dbName + `" OWNER "` + dbName + `"`); e != nil {
				t.Fatal(e)
			}
			defer func() { _, _ = admin.Exec(`DROP DATABASE "` + dbName + `" WITH (FORCE)`) }()
			target := *parsed
			target.Path = "/" + dbName
			target.User = url.UserPassword(dbName, password)
			bindings := &DatabaseBindings{entries: map[string]string{opts.Scope + "\x00" + id: target.String()}}
			bindings.Configure(&opts)

			host, e := New(opts)
			if e != nil {
				t.Fatal(e)
			}
			defer host.Close()
			if _, e = host.Install(bytes.NewReader(data), false); e != nil {
				t.Fatal(e)
			}
			if _, e = host.Enable(id); e != nil {
				t.Fatal("original activation:", e)
			}
			if len(host.Webapp()) != 1 {
				t.Fatal("original webapp manifest missing")
			}
			httpPath := "/plugins/focalboard/api/v2/boards"
			if name == "playbooks" {
				httpPath = "/plugins/playbooks/api/v0/playbooks"
			}
			foreignRequest := httptest.NewRequest("GET", httpPath, nil)
			foreignResponse := httptest.NewRecorder()
			host.ServeHTTP(foreignResponse, foreignRequest, chat.ID(sibling.ID))
			if foreignResponse.Code != 403 {
				t.Fatal("cross-domain plugin access", foreignResponse.Code)
			}
			adapter := &hostAPI{unsupportedAPI: &unsupportedAPI{log: opts.Logger, pluginID: id}, runtime: host, manifest: &model.Manifest{Id: id}}
			group, e := adapter.RegisterPropertyGroup("fixture_properties")
			if e != nil {
				t.Fatal(e)
			}
			field, e := adapter.CreatePropertyField(&model.PropertyField{GroupID: group.ID, Name: "Fixture attribute", Type: model.PropertyFieldType("text"), TargetType: "playbook", TargetID: chat.ID(uuid.NewString())})
			if e != nil {
				t.Fatal("create attribute", e)
			}
			value, e := adapter.UpsertPropertyValue(&model.PropertyValue{GroupID: group.ID, FieldID: field.ID, TargetType: "run", TargetID: chat.ID(uuid.NewString()), Value: json.RawMessage(`"initial"`)})
			if e != nil {
				t.Fatal("set attribute", e)
			}
			value.Value = json.RawMessage(`"updated"`)
			updated, e := adapter.UpsertPropertyValue(value)
			if e != nil || updated.ID != value.ID {
				t.Fatal("attribute identity changed", e)
			}
			values, e := adapter.SearchPropertyValues(group.ID, model.PropertyValueSearchOpts{FieldID: field.ID, TargetIDs: []string{value.TargetID}, PerPage: 1})
			if e != nil || len(values) != 1 || string(values[0].Value) != `"updated"` {
				t.Fatal("read attribute", values, e)
			}
			count, e := adapter.CountPropertyFieldsForTarget(group.ID, field.TargetType, field.TargetID, false)
			if e != nil || count != 1 {
				t.Fatal("count attributes", count, e)
			}

			// The upstream pinned SDK sends CreateAt-only cursors even when
			// querying deltas. Exercise a second page with the real database.
			second, e := adapter.UpsertPropertyValue(&model.PropertyValue{GroupID: group.ID, FieldID: field.ID, TargetType: "run", TargetID: chat.ID(uuid.NewString()), Value: json.RawMessage(`"second"`)})
			if e != nil {
				t.Fatal(e)
			}
			firstPage, e := adapter.SearchPropertyValues(group.ID, model.PropertyValueSearchOpts{SinceUpdateAt: 1, PerPage: 1})
			if e != nil || len(firstPage) != 1 {
				t.Fatal("delta first page", e)
			}
			last := firstPage[0]
			next, e := adapter.SearchPropertyValues(group.ID, model.PropertyValueSearchOpts{SinceUpdateAt: 1, PerPage: 1, Cursor: model.PropertyValueSearchCursor{CreateAt: last.CreateAt, PropertyValueID: last.ID}})
			if e != nil || len(next) != 1 || next[0].ID == last.ID || (next[0].ID != value.ID && next[0].ID != second.ID) {
				t.Fatal("legacy delta cursor", next, e)
			}
			if e = adapter.DeletePropertyValue(group.ID, second.ID); e != nil {
				t.Fatal(e)
			}
			tombstones, e := adapter.SearchPropertyValues(group.ID, model.PropertyValueSearchOpts{SinceUpdateAt: 1, IDs: []string{second.ID}})
			if e != nil || len(tombstones) != 1 || tombstones[0].DeleteAt == 0 {
				t.Fatal("delta deletion omitted", e)
			}
			if name == "playbooks" {
				req := httptest.NewRequest("POST", "/plugins/playbooks/api/v0/playbooks", strings.NewReader(`{"title":"Original Playbook fixture","team_id":"`+chat.ID(tenant.ID)+`","public":true,"reminder_timer_default_seconds":3600,"checklists":[{"title":"Procedure","items":[{"title":"Verify"}]}]}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 201 {
					t.Fatal("create playbook", w.Code, w.Body.String())
				}
				var playbook struct {
					ID string `json:"id"`
				}
				if e = json.Unmarshal(w.Body.Bytes(), &playbook); e != nil || playbook.ID == "" {
					t.Fatal(w.Body.String(), e)
				}
				req = httptest.NewRequest("GET", "/plugins/playbooks/api/v0/playbooks/"+playbook.ID, nil)
				w = httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 200 || !strings.Contains(w.Body.String(), "Verify") {
					t.Fatal("read playbook/checklist", w.Code, w.Body.String())
				}
				req = httptest.NewRequest("POST", "/plugins/playbooks/api/v0/runs", strings.NewReader(`{"name":"Fixture procedure run","owner_user_id":"`+chat.ID(u.ID)+`","team_id":"`+chat.ID(tenant.ID)+`","playbook_id":"`+playbook.ID+`","create_public_run":true}`))
				req.Header.Set("Content-Type", "application/json")
				w = httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 201 {
					t.Fatal("start playbook run", w.Code, w.Body.String())
				}
				var run struct {
					ID string `json:"id"`
				}
				if e = json.Unmarshal(w.Body.Bytes(), &run); e != nil || run.ID == "" {
					t.Fatal("run identity", w.Body.String(), e)
				}
				req = httptest.NewRequest("GET", "/plugins/playbooks/api/v0/runs/"+run.ID, nil)
				w = httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 200 {
					t.Fatal("read run", w.Code, w.Body.String())
				}
				req = httptest.NewRequest("PUT", "/plugins/playbooks/api/v0/runs/"+run.ID+"/checklists/0/item/0/state", strings.NewReader(`{"new_state":"closed"}`))
				req.Header.Set("Content-Type", "application/json")
				w = httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 200 {
					t.Fatal("complete run task", w.Code, w.Body.String())
				}
				req = httptest.NewRequest("GET", "/plugins/playbooks/api/v0/runs/"+run.ID, nil)
				w = httptest.NewRecorder()
				host.ServeHTTP(w, req, chat.ID(u.ID))
				if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"closed"`) {
					t.Fatal("persisted completed task", w.Code, w.Body.String())
				}

			}
			if name == "boards" {
				call := func(method, path, body, uid string) *httptest.ResponseRecorder {
					req := httptest.NewRequest(method, "/plugins/focalboard/api/v2"+path, strings.NewReader(body))
					req.Header.Set("X-Requested-With", "XMLHttpRequest")
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					host.ServeHTTP(w, req, uid)
					return w
				}
				uid := chat.ID(u.ID)
				team := chat.ID(tenant.ID)
				created := call("POST", "/boards", `{"teamId":"`+team+`","title":"Original Boards fixture","type":"O","minimumRole":"editor","properties":{}}`, uid)
				if created.Code != 200 {
					t.Fatal("create board", created.Code, created.Body.String())
				}
				var board struct {
					ID string `json:"id"`
				}
				if e = json.Unmarshal(created.Body.Bytes(), &board); e != nil || board.ID == "" {
					t.Fatal(created.Body.String(), e)
				}
				fetched := call("GET", "/boards/"+board.ID, "", uid)
				if fetched.Code != 200 || !strings.Contains(fetched.Body.String(), "Original Boards fixture") {
					t.Fatal("read board", fetched.Code, fetched.Body.String())
				}
				cardID := "c" + chat.ID(uuid.NewString())
				createdCard := call("POST", "/boards/"+board.ID+"/blocks", fmt.Sprintf(`[{"id":%q,"boardId":%q,"type":"card","schema":1,"title":"Original card","createAt":%d,"updateAt":%d,"createdBy":%q,"modifiedBy":%q,"fields":{"properties":{}}}]`, cardID, board.ID, time.Now().UnixMilli(), time.Now().UnixMilli(), uid, uid), uid)
				if createdCard.Code != 200 {
					t.Fatal("create card", createdCard.Code, createdCard.Body.String())
				}
				var inserted []struct {
					ID string `json:"id"`
				}
				if e = json.Unmarshal(createdCard.Body.Bytes(), &inserted); e != nil || len(inserted) != 1 {
					t.Fatal("card result", createdCard.Body.String(), e)
				}
				cardID = inserted[0].ID
				cards := call("GET", "/boards/"+board.ID+"/blocks", "", uid)
				if cards.Code != 200 || !strings.Contains(cards.Body.String(), "Original card") {
					t.Fatal("read cards", cards.Code, cards.Body.String())
				}
				patched := call("PATCH", "/boards/"+board.ID+"/blocks/"+cardID, `{"title":"Updated original card"}`, uid)
				if patched.Code != 200 {
					t.Fatal("update card", patched.Code, patched.Body.String())
				}
				shortcutID := "b" + chat.ID(uuid.NewString())
				viewID := "v" + chat.ID(uuid.NewString())
				statusID := "a" + chat.ID(uuid.NewString())
				now := time.Now().UnixMilli()
				shortcut := call("POST", "/boards-and-blocks", fmt.Sprintf(`{
					"boards":[{"id":%q,"teamId":%q,"title":"Доска из письма","type":"P","minimumRole":"","properties":{},"cardProperties":[{"id":%q,"name":"Статус","type":"select","options":[]}]}],
					"blocks":[{"id":%q,"boardId":%q,"parentId":%q,"schema":1,"type":"view","title":"Доска","createAt":%d,"updateAt":%d,"fields":{"viewType":"board","groupById":%q}}]
				}`, shortcutID, team, statusID, viewID, shortcutID, shortcutID, now, now, statusID), uid)
				if shortcut.Code != 200 || !strings.Contains(shortcut.Body.String(), "Доска из письма") {
					t.Fatal("create private board from a source", shortcut.Code, shortcut.Body.String())
				}
				list := call("GET", "/teams/0/templates", "", uid)
				var templates []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Team  string `json:"teamId"`
				}
				if list.Code != 200 || json.Unmarshal(list.Body.Bytes(), &templates) != nil {
					t.Fatal("list global templates", list.Code, list.Body.String())
				}
				competitor := ""
				for _, template := range templates {
					if template.Team == "0" && strings.TrimSpace(template.Title) == "Competitive Analysis" {
						competitor = template.ID
					}
				}
				if competitor == "" {
					t.Fatal("missing global Competitive Analysis board")
				}
				duplicate := call("POST", "/boards/"+competitor+"/duplicate?asTemplate=false&toTeam="+team, "", uid)
				var draft struct {
					Boards []struct {
						ID string `json:"id"`
					}
				}
				if duplicate.Code != 200 || json.Unmarshal(duplicate.Body.Bytes(), &draft) != nil || len(draft.Boards) != 1 {
					t.Fatal("copy global template", duplicate.Code, duplicate.Body.String())
				}
				localized := call("PATCH", "/boards-and-blocks", fmt.Sprintf(`{"boardIDs":[%q],"boardPatches":[{"title":"Анализ конкурентов"}],"blockIDs":[],"blockPatches":[]}`, draft.Boards[0].ID), uid)
				if localized.Code != 200 || !strings.Contains(localized.Body.String(), "Анализ конкурентов") {
					t.Fatal("save Russian labels to the new board", localized.Code, localized.Body.String())
				}
				denied := call("GET", "/boards/"+board.ID, "", "")
				if denied.Code == 200 {
					t.Fatal("anonymous private state exposed")
				}
			}
		})
	}
}
