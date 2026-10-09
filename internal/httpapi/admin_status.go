package httpapi

import (
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/metrics"
	"github.com/tayga/tms/internal/version"
)

var startedAt = time.Now().UTC()

func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	global, err := s.store.ServerStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tenant, err := s.store.TenantStats(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var tls any
	if s.tls != nil {
		tls = s.tls.Status()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	writeJSON(w, http.StatusOK, map[string]any{
		"hostname":   s.cfg.Server.Hostname,
		"version":    version.Version,
		"commit":     version.Commit,
		"build_date": version.Date,
		"uptime_sec": int(time.Since(startedAt).Seconds()),
		"started_at": startedAt.Format(time.RFC3339),
		"go": map[string]any{
			"version":    runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"heap_alloc": ms.HeapAlloc,
		},
		"protection": metrics.ProtectionSnapshot(),
		"server":     global,
		"tenant":     tenant,
		"tls":        tls,
		"listeners": map[string]string{
			"smtp_mx":         s.cfg.SMTP.MX,
			"smtp_submission": s.cfg.SMTP.Submission,
			"smtps":           s.cfg.SMTP.SMTPS,
			"imap":            s.cfg.IMAP.Listen,
			"imaps":           s.cfg.IMAP.IMAPS,
			"pop3":            s.cfg.POP3.Listen,
			"pop3s":           s.cfg.POP3.POP3S,
			"managesieve":     s.cfg.ManageSieve.Listen,
			"http":            s.cfg.HTTP.Listen,
			"https":           s.cfg.HTTP.TLSListen,
		},
	})
}

func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	_ = admin
	includeMail := r.URL.Query().Get("include_mail") == "1" || r.URL.Query().Get("include_mail") == "true"
	opts := backup.Options{
		IncludeMail: includeMail,
		TenantID:    admin.TenantID, // tenant-scoped for multi-tenant safety
	}
	if s.ms != nil {
		opts.MailRoot = s.ms.Root
	}

	name := "tayga-backup-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := backup.WriteTarGz(r.Context(), s.store, opts, w); err != nil {
		s.log.Error("backup failed", "err", err)
		return
	}
}

func (s *Server) handleAdminBackupRestore(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart form required (file=backup.tar.gz)"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required"})
		return
	}
	defer file.Close()
	includeMail := r.FormValue("include_mail") != "0" && r.FormValue("include_mail") != "false"
	mailRoot := ""
	if s.ms != nil {
		mailRoot = s.ms.Root
	}
	rep, err := backup.RestoreTarGz(r.Context(), s.store, io.LimitReader(file, 512<<20), backup.RestoreOptions{
		MailRoot:    mailRoot,
		IncludeMail: includeMail,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
