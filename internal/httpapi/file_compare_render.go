package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Bound conversion work across users; no persistent copy or token-bearing output URL is stored.
var fileRenderSlots = make(chan struct{}, 4)
var fileRenderClient = &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func visualCompareFormat(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf", ".docx", ".doc", ".odt", ".rtf", ".txt", ".xlsx", ".ods", ".pptx", ".odp":
		return true
	}
	return false
}

func (s *Server) serveComparisonPDF(w http.ResponseWriter, r *http.Request, sess fileCompareSession, token string, source *os.File) {
	select {
	case fileRenderSlots <- struct{}{}:
		defer func() { <-fileRenderSlots }()
	default:
		http.Error(w, "rendering busy; retry", 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var data []byte
	var err error
	if strings.EqualFold(filepath.Ext(sess.Path), ".pdf") {
		data, err = io.ReadAll(io.LimitReader(source, maxCompareFileBytes+1))
	} else if sess.Provider == "collabora" || sess.Provider == "" {
		data, err = convertCollaboraComparisonPDF(ctx, sess, source)
	} else {
		data, err = s.convertComparisonPDF(ctx, sess, token, r.URL.Query().Get("side"))
	}
	if err != nil {
		http.Error(w, "could not render PDF; check office conversion service", 502)
		return
	}
	if len(data) > maxCompareFileBytes {
		http.Error(w, "rendered PDF exceeds 32 MiB", 413)
		return
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		http.Error(w, "renderer did not return PDF", 422)
		return
	}
	if status := s.comparisonAfterWork(ctx, sess, token); status != 200 {
		http.Error(w, "comparison expired or version changed; reopen", status)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `inline; filename="comparison.pdf"`)
	http.ServeContent(w, r, "comparison.pdf", time.Time{}, bytes.NewReader(data))
}

func (s *Server) comparisonAfterWork(ctx context.Context, sess fileCompareSession, token string) int {
	s.fileMu.Lock()
	active, ok := s.fileComparisons[token]
	cfg := s.cfgLive().Files
	valid := ok && active == sess && time.Now().Before(sess.Expires) && sess.Provider == cfg.OfficeProvider && sess.OfficeURL == strings.TrimRight(cfg.OfficeURL, "/") && sess.SecretHash == sha256.Sum256([]byte(cfg.OfficeJWTSecret))
	user, e := s.store.GetUserByID(ctx, sess.Owner)
	valid = valid && e == nil && user != nil && user.Enabled
	status := 401
	if valid {
		for _, side := range []struct{ id, tag string }{{sess.Left, sess.LeftETag}, {sess.Right, sess.RightETag}} {
			file, tag, e := s.compareFile(sess.Owner, sess.Path, side.id)
			if e != nil {
				valid = false
				status = 404
				break
			}
			file.Close()
			if tag != side.tag {
				valid = false
				status = 409
				break
			}
		}
	}
	s.fileMu.Unlock()
	if !valid {
		return status
	}
	return 200
}

func (s *Server) convertComparisonPDF(ctx context.Context, sess fileCompareSession, token, side string) ([]byte, error) {
	suffix := ""
	if sess.Mode == "motion" {
		suffix = "&storyboard=1"
	}
	key := sha256.Sum256([]byte("tayga-render\x00" + token + "\x00" + side + suffix))
	payload := map[string]any{"async": false, "key": hex.EncodeToString(key[:]), "filetype": strings.TrimPrefix(strings.ToLower(filepath.Ext(sess.Path)), "."), "outputtype": "pdf", "url": s.publicBase() + "/office/compare/content?access_token=" + token + "&side=" + side + suffix}
	signed, err := signDocumentConfig(payload, s.cfgLive().Files.OfficeJWTSecret)
	if err != nil {
		return nil, err
	}
	payload["token"] = signed
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", sess.OfficeURL+"/converter", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := fileRenderClient.Do(req)
	if err != nil {
		return nil, err
	}
	var result struct {
		EndConvert bool   `json:"endConvert"`
		FileURL    string `json:"fileUrl"`
		Error      int    `json:"error"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || !result.EndConvert || result.Error != 0 {
		return nil, fmt.Errorf("conversion failed")
	}
	download, err := url.Parse(result.FileURL)
	configured, _ := url.Parse(sess.OfficeURL)
	if err != nil || download.Scheme != "https" || !strings.EqualFold(download.Host, configured.Host) || download.User != nil || download.Fragment != "" {
		return nil, fmt.Errorf("invalid converter origin")
	}
	req, err = http.NewRequestWithContext(ctx, "GET", download.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err = fileRenderClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("PDF download failed")
	}
	return io.ReadAll(io.LimitReader(response.Body, maxCompareFileBytes+1))
}

// Multipart framing surrounds a bounded file reader, without buffering a second source copy.
func convertCollaboraComparisonPDF(ctx context.Context, sess fileCompareSession, source *os.File) ([]byte, error) {
	var frame bytes.Buffer
	writer := multipart.NewWriter(&frame)
	if _, err := writer.CreateFormFile("data", "comparison"+strings.ToLower(filepath.Ext(sess.Path))); err != nil {
		return nil, err
	}
	prefix := append([]byte(nil), frame.Bytes()...)
	frame.Reset()
	if err := writer.Close(); err != nil {
		return nil, err
	}
	info, err := source.Stat()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", sess.OfficeURL+"/cool/convert-to/pdf", io.MultiReader(bytes.NewReader(prefix), io.NewSectionReader(source, 0, info.Size()), bytes.NewReader(frame.Bytes())))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(prefix)+frame.Len()) + info.Size()
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/pdf")
	response, err := fileRenderClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Collabora PDF conversion failed")
	}
	return io.ReadAll(io.LimitReader(response.Body, maxCompareFileBytes+1))
}
