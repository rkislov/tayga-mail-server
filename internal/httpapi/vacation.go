package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/sieve"
)

const vacationScriptName = "tayga-vacation"

func (s *Server) handleVacation(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		sc, err := s.store.GetSieveScript(r.Context(), au.ID, vacationScriptName)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "subject": "", "body": ""})
			return
		}
		active, _ := s.store.GetActiveSieveScript(r.Context(), au.ID)
		enabled := active != nil && active.Name == vacationScriptName
		subj, body := parseVacationScript(sc.Script)
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": enabled, "subject": subj, "body": body, "script": sc.Script,
		})

	case http.MethodPut:
		var req struct {
			Enabled bool   `json:"enabled"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
			Start   string `json:"start"`
			End     string `json:"end"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if !req.Enabled {
			_ = s.store.DeleteSieveScript(r.Context(), au.ID, vacationScriptName)
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
			return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body required"})
			return
		}
		script := buildVacationScript(req.Subject, req.Body, req.Start, req.End)
		if err := sieve.CheckScript(script); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sieve: " + err.Error()})
			return
		}
		if _, err := s.store.PutSieveScript(r.Context(), au.ID, vacationScriptName, script); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.SetActiveSieveScript(r.Context(), au.ID, vacationScriptName); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "subject": req.Subject})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func buildVacationScript(subject, body, start, end string) string {
	subject = strings.ReplaceAll(subject, `"`, `'`)
	if subject == "" {
		subject = "Out of office"
	}
	body = strings.ReplaceAll(body, `\`, `\\`)
	body = strings.ReplaceAll(body, `"`, `\"`)
	var b strings.Builder
	b.WriteString(`require ["vacation"];` + "\n")
	if start != "" || end != "" {
		// best-effort date markers as comments; engine may ignore
		fmt.Fprintf(&b, "/* tayga-vacation start=%s end=%s */\n", start, end)
	}
	fmt.Fprintf(&b, `vacation :days 1 :subject "%s" "%s";`+"\n", subject, body)
	_ = time.Now()
	return b.String()
}

func parseVacationScript(script string) (subject, body string) {
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "vacation") {
			continue
		}
		if i := strings.Index(line, `:subject "`); i >= 0 {
			rest := line[i+len(`:subject "`):]
			if j := strings.Index(rest, `"`); j >= 0 {
				subject = rest[:j]
				rest = strings.TrimSpace(rest[j+1:])
				rest = strings.TrimPrefix(rest, `"`)
				if k := strings.LastIndex(rest, `"`); k > 0 {
					body = rest[:k]
				}
			}
		}
	}
	return subject, body
}
