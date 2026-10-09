// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.
package flowsync

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type traceKey struct{}
type requestTrace struct{ User, Command, Result string }
type loggedResponse struct {
	http.ResponseWriter
	status, bytes int
}

func (w *loggedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *loggedResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *loggedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func traceRequest(r *http.Request) *requestTrace {
	t, _ := r.Context().Value(traceKey{}).(*requestTrace)
	return t
}
func boundedLog(s string) string {
	if len(s) > 256 {
		return s[:256]
	}
	return s
}

// Log metadata only: never record credentials, query strings, or message bodies.
func logRequests(log *slog.Logger, protocol string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		t := &requestTrace{Command: boundedLog(r.URL.Query().Get("Cmd"))}
		rw := &loggedResponse{ResponseWriter: w}
		defer func() {
			status := rw.status
			if status == 0 {
				status = 200
			}
			level := slog.LevelInfo
			if status >= 400 || t.Result != "" {
				level = slog.LevelWarn
			}
			log.Log(r.Context(), level, "flowsync request", "protocol", protocol, "method", r.Method, "command", t.Command, "user_id", t.User, "device_id", boundedLog(r.URL.Query().Get("DeviceId")), "client", boundedLog(r.UserAgent()), "peer", r.RemoteAddr, "version", boundedLog(r.Header.Get("MS-ASProtocolVersion")), "status", status, "result", t.Result, "response_bytes", rw.bytes, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), traceKey{}, t)))
	})
}
