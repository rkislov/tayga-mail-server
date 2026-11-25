package mattermost

// Opt-in, loopback-only UI acceptance stand. All identities and model responses
// are disposable test fixtures; no production tokens or connectors are loaded.
import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentsBrowserStand(t *testing.T) {
	if os.Getenv("TMS_TEST_AGENTS_BROWSER") != "1" {
		t.Skip("opt-in local browser fixture")
	}
	f, _, _ := agentsFixture(t)
	if os.Getenv("TMS_TEST_AGENTS_BROWSER_EMPTY") == "1" {
		if err := f.api.agentSave(t.Context(), agentConfigKey(f.users[0]), emptyAgentConfig()); err != nil {
			t.Fatal(err)
		}
	}

	if os.Getenv("TMS_TEST_AGENTS_BROWSER_EMPTY") != "1" {
		c := emptyAgentConfig()
		if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &c); err != nil {
			t.Fatal(err)
		}
		c.Agents[0].Name = "Помощник команды"
		c.Agents[0].Description = "Обсуждайте идеи, разбирайте вопросы и находите решения вместе."
		c.Agents = append(c.Agents, agentDefinition{ID: "editor", Name: "Редактор", Description: "Превращает черновики в ясные письма, заметки и документы.", ProviderID: "model", Enabled: true}, agentDefinition{ID: "analyst", Name: "Аналитик", Description: "Помогает структурировать информацию и сравнивать варианты.", ProviderID: "model", Enabled: true})
		if err := f.api.agentSave(t.Context(), agentConfigKey(f.users[0]), c); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join("..", "..")

	mux := http.NewServeMux()
	mux.Handle("/api/v4/", f.api)
	mux.HandleFunc("/agents.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, filepath.Join(root, "frontend", "src", "mattermost-agents.js"))
	})
	mux.HandleFunc("/agents.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		http.ServeFile(w, r, filepath.Join(root, "internal", "frontend", "dist", "assets", "app.css"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Agents · изолированный стенд</title><link rel="stylesheet" href="/agents.css"><style>.fixture-brand{font-size:20px;font-weight:600}.fixture-rail{margin-top:22px;display:grid;gap:14px}.fixture-rail button{height:36px}.workspace-header{justify-content:space-between}.workspace-header>input{width:45%%;background:#ffffffed;color:#52604d;border-radius:9px;padding:8px 16px;font-size:13px}.workspace-main{padding:0}.workspace-header>span:last-child{border:1px solid #ffffff66;border-radius:50%%;padding:7px;font-size:11px}#view-account.workspace-shell{background:#263c2c}.fixture-rail .nav-ico{font-size:18px}.agents-fixture-toggle{font-size:12px;padding:6px;border:1px solid #ffffff40;border-radius:6px}</style><div id="view-account" class="workspace-shell sidebar-collapsed"><header class="workspace-header"><span class="fixture-brand">Tayga Mail</span><input placeholder="Поиск писем" aria-label="Поиск писем"><button class="agents-fixture-toggle" onclick="document.getElementById('view-account').classList.toggle('sidebar-collapsed')">Меню</button><span>АЛ</span></header><aside class="workspace-nav"><nav class="fixture-rail"><button class="nav-btn" data-app="chat"><span class="nav-ico" aria-hidden="true">▤</span><span>Чат</span></button></nav></aside><main class="workspace-main"><section id="app-chat" class="app-page"></section></main></div><script>const state={email:'alice@example.com',tokens:{access_token:%q}};const APPS=['chat'];let showApp=name=>{document.querySelectorAll('main>section').forEach(el=>el.classList.toggle('hidden',el.id!=='app-'+name))};async function api(url,options={}){options._guard?.();const res=await fetch(url,{...options,headers:{Authorization:'Bearer '+state.tokens.access_token,'Content-Type':'application/json'}});const data=await res.json();options._guard?.();if(!res.ok){const e=Error(data.message);e.data=data;throw e}return data}</script><script src="/agents.js"></script><script>showApp('agents')</script></html>`, f.tokens[0])
	})
	listener, err := net.Listen("tcp", "127.0.0.1:59865")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Agents fixture: http://127.0.0.1:59865 (expires after 10 minutes)")
	timer := time.NewTimer(10 * time.Minute)
	defer timer.Stop()
	select {
	case <-t.Context().Done():
	case <-timer.C:
	}
}
