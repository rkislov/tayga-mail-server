package mattermost

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/unicode"
)

const agentDownloadLimit = 8 << 20

type agentDownloadedFile struct {
	name, mediaType string
	data            []byte
}

type agentFiles struct {
	downloads     map[string]agentDownloadedFile
	images        map[string]agentGeneratedImage
	outputs       []agentArtifact
	attempts      int
	imageAttempts int
}

func (f *agentFiles) download(ctx context.Context, raw string) (any, error) {
	if len(raw) > 4096 {
		return nil, errors.New("слишком длинная ссылка")
	}
	if f.attempts >= 3 {
		return nil, errors.New("лимит загрузок исчерпан")
	}
	f.attempts++
	u, err := url.Parse(raw)
	if err != nil || !agentPublicFileURL(u) {
		return nil, errors.New("нужна публичная ссылка HTTP(S)")
	}
	client := agentDownloadClient()
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("некорректная ссылка")
	}
	req.Header.Set("Accept", "*/*")
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("не удалось загрузить файл с публичного адреса")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("источник вернул HTTP " + strconv.Itoa(response.StatusCode))
	}
	if response.ContentLength > agentDownloadLimit {
		return nil, errors.New("файл больше 8 МиБ")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, agentDownloadLimit+1))
	if err != nil {
		return nil, errors.New("загрузка прервана")
	}
	if len(data) > agentDownloadLimit {
		return nil, errors.New("файл больше 8 МиБ")
	}
	if len(data) == 0 {
		return nil, errors.New("источник вернул пустой файл")
	}
	mediaType := strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0]
	name := path.Base(response.Request.URL.Path)
	if name == "." || name == "/" {
		name = "download"
	}
	if f.downloads == nil {
		f.downloads = make(map[string]agentDownloadedFile)
	}
	id := agentID()
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		if decoded, e := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(data); e == nil && len(decoded) <= agentDownloadLimit {
			data = decoded
		}
	}
	f.downloads[id] = agentDownloadedFile{name, mediaType, data}
	result := map[string]any{"file_id": id, "name": name, "content_type": mediaType, "bytes": len(data), "source_url": u.Redacted()}
	if agentReadableText(data) {
		preview, _ := f.read(id, "0")
		for key, value := range preview {
			result[key] = value
		}
	}
	return result, nil
}

func agentReadableText(data []byte) bool { return utf8.Valid(data) && !bytes.ContainsRune(data, 0) }

func (f *agentFiles) read(id, rawOffset string) (map[string]any, error) {
	file, ok := f.downloads[id]
	if !ok {
		return nil, errors.New("файл доступен только в том запросе, где он был скачан")
	}
	offset, err := strconv.Atoi(rawOffset)
	if err != nil || offset < 0 || offset > len(file.data) {
		return nil, errors.New("некорректное смещение")
	}
	if !agentReadableText(file.data) {
		return nil, errors.New("для бинарного файла или этой кодировки нет текстового представления")
	}
	if offset < len(file.data) && !utf8.RuneStart(file.data[offset]) {
		return nil, errors.New("используйте значение next_offset")
	}
	// Keep even JSON-escaped text below the tool response limit.
	end := min(len(file.data), offset+6000)
	for end < len(file.data) && !utf8.RuneStart(file.data[end]) {
		end--
	}
	return map[string]any{"name": file.name, "content": string(file.data[offset:end]), "next_offset": end, "eof": end == len(file.data)}, nil
}

func agentPublicFileURL(u *url.URL) bool {
	if u == nil || u.User != nil || u.Hostname() == "" || u.Opaque != "" {
		return false
	}
	return (u.Scheme == "https" && (u.Port() == "" || u.Port() == "443")) ||
		(u.Scheme == "http" && (u.Port() == "" || u.Port() == "80"))
}

var agentPublicIPv6 = netip.MustParsePrefix("2000::/3")
var agentSpecialAddresses = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("3fff::/20"),
}

func agentPublicFileIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || (ip.Is6() && !agentPublicIPv6.Contains(ip)) {
		return false
	}
	for _, prefix := range agentSpecialAddresses {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func agentDownloadClient() *http.Client {
	return &http.Client{
		Timeout: 25 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
			TLSHandshakeTimeout:    8 * time.Second,
			ResponseHeaderTimeout:  12 * time.Second,
			MaxResponseHeaderBytes: 32 << 10,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
				if err != nil || len(addresses) == 0 {
					return nil, errors.New("host has no public address")
				}
				local, err := net.InterfaceAddrs()
				if err != nil {
					return nil, err
				}
				for _, ip := range addresses {
					if !agentPublicFileIP(ip) {
						return nil, errors.New("destination is not public")
					}
					for _, addr := range local {
						if prefix, err := netip.ParsePrefix(addr.String()); err == nil && prefix.Addr().Unmap() == ip.Unmap() {
							return nil, errors.New("destination is on this server")
						}
					}
				}
				dial := &net.Dialer{Timeout: 7 * time.Second}
				for _, ip := range addresses {
					conn, err := dial.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
					if err == nil {
						return conn, nil
					}
				}
				return nil, errors.New("public destination unavailable")
			},
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 4 || !agentPublicFileURL(r.URL) ||
				(via[len(via)-1].URL.Scheme == "https" && r.URL.Scheme != "https") {
				return errors.New("redirect not allowed")
			}
			return nil
		},
	}
}
