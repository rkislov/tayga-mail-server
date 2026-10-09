package httpapi_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	netsmtp "net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	imapserver "github.com/tayga/tms/internal/imap"
	"github.com/tayga/tms/internal/mailstore"
	smtpserver "github.com/tayga/tms/internal/smtp"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

// BenchmarkProtocols uses real loopback TCP/TLS connections and isolated data.
// No messages or load are sent to production users or external recipients.
func BenchmarkProtocols(b *testing.B) {
	kinds := []string{"HTTPSPage1000", "HTTPSPage20000", "TLSHandshake", "SMTPStartTLS10KiB", "IMAPSTLSFetch50", "FlowSyncHTTPSFolderSync"}
	if os.Getenv("TAYGA_BENCH_SCAN") == "1" {
		kinds = append(kinds, "SMTPStartTLS10KiBScan")
	}
	for _, kind := range kinds {
		b.Run(kind, func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dir := b.TempDir()
			cfg := config.Default()
			cfg.Server.Hostname = "localhost"
			cfg.FlowSync.Enabled = true
			cfg.Scan.Enabled = false
			cfg.Spam.Enabled = false
			cfg.SMTP.OutboundDirect = false
			cfg.SMTP.Queue.Enabled = false
			cfg.SMTP.MX = ""
			cfg.SMTP.SMTPS = ""
			cfg.IMAP.Listen = ""
			cfg.TLS.CertFile = filepath.Join(dir, "cert.pem")
			cfg.TLS.KeyFile = filepath.Join(dir, "key.pem")
			cfg.TLS.CertsDir = filepath.Join(dir, "certs")
			if err := tlsutil.EnsureSelfSigned(cfg.TLS.CertFile, cfg.TLS.KeyFile, []string{"localhost", "127.0.0.1"}); err != nil {
				b.Fatal(err)
			}
			manager, err := tlsutil.NewManager(cfg)
			if err != nil {
				b.Fatal(err)
			}
			certPEM, err := os.ReadFile(cfg.TLS.CertFile)
			if err != nil {
				b.Fatal(err)
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(certPEM) {
				b.Fatal("certificate")
			}
			clientTLS := &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}
			st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
			if err != nil {
				b.Fatal(err)
			}
			defer st.Close()
			layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
			tenant, err := st.CreateTenant(ctx, "bench")
			if err != nil {
				b.Fatal(err)
			}
			domain, err := st.CreateDomain(ctx, tenant.ID, "example.com")
			if err != nil {
				b.Fatal(err)
			}
			hash, err := layer.Hasher.Hash("benchmark-password")
			if err != nil {
				b.Fatal(err)
			}
			user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "bench@example.com", LocalPart: "bench", PasswordHash: hash, AuthSource: "local", Enabled: true})
			if err != nil {
				b.Fatal(err)
			}
			ms := mailstore.New(filepath.Join(dir, "mail"))
			if err = os.MkdirAll(ms.Root, 0750); err != nil {
				b.Fatal(err)
			}
			sample := "From: sender@example.com\r\nTo: bench@example.com\r\nSubject: Benchmark\r\nContent-Type: text/plain\r\n\r\n" + strings.Repeat(strings.Repeat("x", 76)+"\r\n", 132)
			if err = os.WriteFile(filepath.Join(ms.Root, "sample"), []byte(sample), 0640); err != nil {
				b.Fatal(err)
			}
			box, err := st.EnsureMailbox(ctx, user.ID, "INBOX", ms.UserRoot(user.Email))
			if err != nil {
				b.Fatal(err)
			}
			seedCount := 1000
			if kind == "HTTPSPage20000" {
				seedCount = 20000
			}
			for i := 0; i < seedCount; i++ {
				if _, err = st.InsertMessage(ctx, &storage.Message{MailboxID: box.ID, UID: int64(i + 1), Size: int64(len(sample)), FilePath: "sample", InternalDate: time.Now(), MessageID: fmt.Sprintf("<%d@example.com>", i), Subject: "Benchmark", FromAddr: "sender@example.com", ToAddr: user.Email}); err != nil {
					b.Fatal(err)
				}
			}
			token, err := layer.Tokens.IssueTokens(ctx, user.ID)
			if err != nil {
				b.Fatal(err)
			}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			api := httptest.NewUnstartedServer(httpapi.New(cfg, log, st, layer, ms, nil, nil).Handler())
			api.TLS = manager.TLSConfig()
			api.StartTLS()
			defer api.Close()
			transport := &http.Transport{TLSClientConfig: clientTLS, MaxIdleConns: 32, MaxIdleConnsPerHost: 32}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
			address := func() string {
				l, e := net.Listen("tcp", "127.0.0.1:0")
				if e != nil {
					b.Fatal(e)
				}
				a := l.Addr().String()
				l.Close()
				return a
			}
			smtpAddr, imapAddr := address(), address()
			if strings.HasPrefix(kind, "SMTPStartTLS10KiB") {
				if kind == "SMTPStartTLS10KiBScan" {
					cfg.Scan.Enabled = true
					cfg.Scan.Backend = "clamav"
					cfg.Scan.ClamAV.Address = "127.0.0.1:3310"
					cfg.Scan.FailOpen = false
					cfg.Spam.Enabled = true
					cfg.Spam.Backend = "rspamd"
					cfg.Spam.URL = "http://127.0.0.1:11333"
					cfg.Spam.FailOpen = false
				}
				cfg.SMTP.Submission = smtpAddr
				s := smtpserver.New(cfg, log, st, layer, ms, nil, manager, nil, nil)
				if err = s.Start(ctx); err != nil {
					b.Fatal(err)
				}
				defer s.Shutdown(context.Background())
			}
			if kind == "IMAPSTLSFetch50" {
				cfg.IMAP.IMAPS = imapAddr
				s := imapserver.New(cfg, log, st, layer, ms, nil, manager, nil)
				if err = s.Start(ctx); err != nil {
					b.Fatal(err)
				}
				defer s.Shutdown(context.Background())
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var sc *netsmtp.Client
				var ic *imapclient.Client
				if strings.HasPrefix(kind, "SMTPStartTLS10KiB") {
					var e error
					sc, e = netsmtp.Dial(smtpAddr)
					if e == nil {
						e = sc.StartTLS(clientTLS)
					}
					if e == nil {
						e = sc.Auth(netsmtp.PlainAuth("", user.Email, "benchmark-password", "127.0.0.1"))
					}
					if e != nil {
						b.Error(e)
						return
					}
					defer sc.Quit()
				}
				if kind == "IMAPSTLSFetch50" {
					var e error
					ic, e = imapclient.DialTLS(imapAddr, clientTLS)
					if e == nil {
						e = ic.Login(user.Email, "benchmark-password")
					}
					if e == nil {
						_, e = ic.Select("INBOX", true)
					}
					if e != nil {
						b.Error(e)
						return
					}
					defer ic.Logout()
				}
				for pb.Next() {
					switch kind {
					case "TLSHandshake":
						c, e := tls.Dial("tcp", api.Listener.Addr().String(), clientTLS)
						if e != nil {
							b.Error(e)
							return
						}
						c.Close()
					case "SMTPStartTLS10KiB", "SMTPStartTLS10KiBScan":
						e := sc.Mail(user.Email)
						if e == nil {
							e = sc.Rcpt(user.Email)
						}
						if e == nil {
							var w io.WriteCloser
							w, e = sc.Data()
							if e == nil {
								_, e = io.WriteString(w, sample)
								ce := w.Close()
								if e == nil {
									e = ce
								}
							}
						}
						if e != nil {
							b.Error(e)
							return
						}
					case "IMAPSTLSFetch50":
						seq := new(imap.SeqSet)
						seq.AddRange(1, 50)
						messages := make(chan *imap.Message, 50)
						done := make(chan error, 1)
						go func() {
							done <- ic.Fetch(seq, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, imap.FetchRFC822Size}, messages)
						}()
						count := 0
						for range messages {
							count++
						}
						if e := <-done; e != nil || count != 50 {
							b.Errorf("fetch %d %v", count, e)
							return
						}
					default:
						path := "/api/v1/mail/mailboxes/" + box.ID + "/messages?limit=50"
						method := "GET"
						var body io.Reader
						if kind == "FlowSyncHTTPSFolderSync" {
							method = "POST"
							path = "/Microsoft-Server-ActiveSync?Cmd=FolderSync&DeviceId=benchmark&DeviceType=Benchmark"
							body = strings.NewReader("<FolderSync><SyncKey>0</SyncKey></FolderSync>")
						}
						r, e := http.NewRequest(method, api.URL+path, body)
						if e != nil {
							b.Error(e)
							return
						}
						r.Header.Set("Authorization", "Bearer "+token.AccessToken)
						r.Header.Set("Content-Type", "application/xml")
						response, e := client.Do(r)
						if e != nil {
							b.Error(e)
							return
						}
						data, e := io.ReadAll(response.Body)
						response.Body.Close()
						if e != nil || response.StatusCode != 200 || (kind == "FlowSyncHTTPSFolderSync" && !strings.Contains(string(data), "<Status>1</Status>")) {
							b.Errorf("response %d %v %s", response.StatusCode, e, data)
							return
						}
					}
				}
			})
		})
	}
}
