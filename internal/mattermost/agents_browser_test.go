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
	root := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(root, "frontend", "src", "mattermost-agents.js"))
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile(filepath.Join(root, "internal", "frontend", "dist", "assets", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/v4/", f.api)
	mux.HandleFunc("/agents.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(script)
	})
	mux.HandleFunc("/agents.css", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "text/css"); w.Write(css) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Agents · изолированный стенд</title><link rel="stylesheet" href="/agents.css"><style>body{margin:0;padding:20px;background:#f4f5f8;font:14px Arial,sans-serif}nav{display:flex;gap:12px;margin-bottom:16px}nav button{padding:10px}.hidden{display:none!important}</style><nav><button data-app="chat">Чат</button></nav><main><section id="app-chat"></section></main><script>const state={email:'alice@example.com',tokens:{access_token:%q}};const APPS=['chat'];let showApp=name=>{document.querySelectorAll('main>section').forEach(el=>el.classList.toggle('hidden',el.id!=='app-'+name))};async function api(url,options={}){options._guard?.();const res=await fetch(url,{...options,headers:{Authorization:'Bearer '+state.tokens.access_token,'Content-Type':'application/json'}});const data=await res.json();options._guard?.();if(!res.ok){const e=Error(data.message);e.data=data;throw e}return data}</script><script src="/agents.js"></script><script>showApp('agents')</script></html>`, f.tokens[0])
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
