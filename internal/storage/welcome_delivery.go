package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailcompose"
)

type welcomeMailSuppressedKey struct{}

// WithoutWelcomeMail is for restoring/importing existing accounts. Ordinary
// provisioning (including LDAP/OIDC JIT) must use the unmodified context.
func WithoutWelcomeMail(ctx context.Context) context.Context {
	return context.WithValue(ctx, welcomeMailSuppressedKey{}, true)
}

type WelcomeDeliveryOrigin struct {
	Kind     string `json:"kind"`
	ScopeID  string `json:"scope_id"`
	Revision int64  `json:"revision"`
}

type WelcomeDeliveryStats struct {
	Pending   int `json:"pending"`
	Delivered int `json:"delivered"`
	Retrying  int `json:"retrying"`
}

// queueWelcomeMailTx snapshots policy, recipient substitutions, sender and MIME
// inside the user INSERT transaction. No scan/backfill of existing users occurs.
func (s *Store) queueWelcomeMailTx(ctx context.Context, tx *sql.Tx, u *User) error {
	if suppressed, _ := ctx.Value(welcomeMailSuppressedKey{}).(bool); suppressed || u.AuthSource == "resource" {
		return nil
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT snapshot FROM welcome_templates WHERE (kind='domains' AND scope_id=?) OR (kind='tenants' AND scope_id=?)`), u.DomainID, u.TenantID)
	if err != nil {
		return err
	}
	domainTemplate, tenantTemplate := emptyWelcome("domains", u.DomainID), emptyWelcome("tenants", u.TenantID)
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var v WelcomeTemplate
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			rows.Close()
			return err
		}
		if v.Kind == "domains" {
			domainTemplate = &v
		} else {
			tenantTemplate = &v
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	effective := domainTemplate
	if effective.Mode == "inherit" {
		effective = tenantTemplate
	}
	if effective.Mode != "own" || effective.Published == 0 {
		return nil
	}
	var revision *WelcomeRevision
	for i := range effective.Revisions {
		if effective.Revisions[i].Number == effective.Published {
			revision = &effective.Revisions[i]
			break
		}
	}
	if revision == nil {
		return fmt.Errorf("welcome published revision missing")
	}
	var domainName, tenantName string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT d.name,t.name FROM domains d JOIN tenants t ON t.id=d.tenant_id WHERE d.id=? AND t.id=?`), u.DomainID, u.TenantID).Scan(&domainName, &tenantName)
	if err != nil {
		return err
	}
	name := u.DisplayName
	if name == "" {
		name = u.LocalPart
	}
	values := []string{"{{name}}", name, "{{email}}", u.Email, "{{domain}}", domainName, "{{tenant}}", tenantName}
	textReplacer := strings.NewReplacer(values...)
	escaped := append([]string{}, values...)
	for i := 1; i < len(escaped); i += 2 {
		escaped[i] = html.EscapeString(escaped[i])
	}
	htmlBody := strings.NewReplacer(escaped...).Replace(revision.HTML)
	// Render once. Replacement values are never recursively expanded.
	htmlBody, images, err := mailcompose.PrepareInlineImages(htmlBody)
	if err != nil {
		return err
	}
	id, now := NewID(), time.Now().UTC()
	sender := "postmaster@" + domainName
	subject := strings.NewReplacer("\r", " ", "\n", " ", "\x00", "").Replace(textReplacer.Replace(revision.Subject))
	raw := mailcompose.BuildMessage(sender, tenantName, []string{u.Email}, nil, nil, subject, textReplacer.Replace(revision.Text), htmlBody, "<"+id+"@tayga>")
	raw, err = mailcompose.AttachInlineImages(raw, images)
	if err != nil {
		return err
	}
	origin, _ := json.Marshal(WelcomeDeliveryOrigin{Kind: effective.Kind, ScopeID: effective.ScopeID, Revision: revision.Number})
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_deliveries(id,sender,recipient,recipient_user_id,subject,data,next_attempt,created_at,kind,welcome_user_id,origin_json) VALUES(?,?,?,?,?,?,?,?,'welcome',?,?)`), id, sender, u.Email, u.ID, subject, raw, now.UnixMilli(), now.UnixNano(), u.ID, string(origin))
	return err
}

// Statistics are scoped by the recipient's administrative area, including
// inherited tenant templates, without exposing recipients in other domains.
func (s *Store) WelcomeDeliveryStats(ctx context.Context, kind, id string) (WelcomeDeliveryStats, error) {
	column := "u.tenant_id"
	if kind == "domains" {
		column = "u.domain_id"
	} else if kind != "tenants" {
		return WelcomeDeliveryStats{}, ErrUnauthorized
	}
	var stats WelcomeDeliveryStats
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT
 COALESCE(SUM(CASE WHEN d.state<>'done' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN d.state='done' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN d.state<>'done' AND d.last_error<>'' THEN 1 ELSE 0 END),0)
 FROM calendar_deliveries d JOIN users u ON u.id=d.welcome_user_id
 WHERE d.kind='welcome' AND `+column+`=?`), id).Scan(&stats.Pending, &stats.Delivered, &stats.Retrying)
	return stats, err
}
