// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.
package flowsync

import (
	"context"
	"github.com/tayga/tms/internal/storage"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type traceKey struct{}
type requestTrace struct{ User, Command, Result, Folders string }
type loggedResponse struct {
	http.ResponseWriter
	status, bytes int
	capture       *traceBuffer
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
	if w.capture != nil {
		w.capture.Write(p[:n])
	}
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
func logRequests(log *slog.Logger, protocol string, next http.Handler, debug ...bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := storage.NewID()
		var requestData, responseData traceBuffer
		debugEnabled := len(debug) > 0 && debug[0]
		if debugEnabled && r.Body != nil {
			r.Body = &traceBody{Reader: io.TeeReader(r.Body, &requestData), Closer: r.Body}
		}
		t := &requestTrace{Command: boundedLog(r.URL.Query().Get("Cmd"))}
		rw := &loggedResponse{ResponseWriter: w}
		if debugEnabled {
			rw.capture = &responseData
		}
		w.Header().Set("X-FlowSync-Request-ID", requestID)
		defer func() {
			status := rw.status
			if status == 0 {
				status = 200
			}
			level := slog.LevelInfo
			if status >= 400 || t.Result != "" {
				level = slog.LevelWarn
			}
			log.Log(r.Context(), level, "flowsync request", "request_id", requestID, "protocol", protocol, "method", r.Method, "command", t.Command, "user_id", t.User, "device_id", boundedLog(r.URL.Query().Get("DeviceId")), "client", boundedLog(r.UserAgent()), "peer", r.RemoteAddr, "version", boundedLog(r.Header.Get("MS-ASProtocolVersion")), "status", status, "result", t.Result, "folders", t.Folders, "response_bytes", rw.bytes, "duration_ms", time.Since(start).Milliseconds())
			if debugEnabled {
				log.Info("flowsync protocol trace", "request_id", requestID, "protocol", protocol, "command", t.Command, "request_content_type", boundedLog(r.Header.Get("Content-Type")), "response_content_type", boundedLog(w.Header().Get("Content-Type")), "request_structure", protocolStructure(requestData.data), "response_structure", protocolStructure(responseData.data), "request_truncated", requestData.truncated, "response_truncated", responseData.truncated)
			}
		}()
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), traceKey{}, t)))
	})
}
