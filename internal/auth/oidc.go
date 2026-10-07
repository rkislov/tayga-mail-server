package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

var ErrOIDCNotConfigured = errors.New("oidc not configured for domain")

// OIDCClient handles authorization-code login per domain.
type OIDCClient struct {
	cfg    config.OIDCConfig
	store  storage.Driver
	tokens *TokenService
	http   *http.Client
	mu     sync.Mutex
	states map[string]oidcState
}

type oidcState struct {
	Domain    string
	ExpiresAt time.Time
}

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type oidcTokenResp struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

type oidcUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// NewOIDCClient builds an OIDC helper. Safe when no domains are enabled.
func NewOIDCClient(store storage.Driver, tokens *TokenService, cfg config.OIDCConfig) *OIDCClient {
	return &OIDCClient{
		cfg:    cfg,
		store:  store,
		tokens: tokens,
		http:   &http.Client{Timeout: 15 * time.Second},
		states: make(map[string]oidcState),
	}
}

// EnabledFor reports whether OIDC is configured for the domain.
func (c *OIDCClient) EnabledFor(domain string) bool {
	d, ok := c.cfg.Domains[strings.ToLower(domain)]
	return ok && d.Enabled
}

// AuthURL starts the OIDC flow and returns the IdP authorize URL.
func (c *OIDCClient) AuthURL(ctx context.Context, domain string) (string, error) {
	dc, ok := c.cfg.Domains[strings.ToLower(domain)]
	if !ok || !dc.Enabled {
		return "", ErrOIDCNotConfigured
	}
	disc, err := c.discover(ctx, dc.Issuer)
	if err != nil {
		return "", err
	}
	state, err := randomToken(24)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.states[state] = oidcState{Domain: strings.ToLower(domain), ExpiresAt: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()

	scopes := dc.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}
	q := url.Values{}
	q.Set("client_id", dc.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", dc.RedirectURL)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	return disc.AuthorizationEndpoint + "?" + q.Encode(), nil
}

// HandleCallback exchanges the code, JIT-provisions the user, and issues tokens.
func (c *OIDCClient) HandleCallback(ctx context.Context, state, code string) (*TokenPair, *storage.User, error) {
	c.mu.Lock()
	st, ok := c.states[state]
	if ok {
		delete(c.states, state)
	}
	c.mu.Unlock()
	if !ok || time.Now().After(st.ExpiresAt) {
		return nil, nil, ErrInvalidCredentials
	}
	dc, ok := c.cfg.Domains[st.Domain]
	if !ok || !dc.Enabled {
		return nil, nil, ErrOIDCNotConfigured
	}
	disc, err := c.discover(ctx, dc.Issuer)
	if err != nil {
		return nil, nil, err
	}
	tr, err := c.exchangeCode(ctx, disc.TokenEndpoint, dc, code)
	if err != nil {
		return nil, nil, err
	}
	info, err := c.fetchUserInfo(ctx, disc.UserinfoEndpoint, tr.AccessToken)
	if err != nil {
		// Fallback: decode email from id_token payload without signature verify
		// when userinfo is unavailable (MVP; production should verify JWT).
		info, err = emailFromIDToken(tr.IDToken)
		if err != nil {
			return nil, nil, err
		}
	}
	email := strings.ToLower(strings.TrimSpace(info.Email))
	if email == "" || !strings.HasSuffix(email, "@"+st.Domain) {
		return nil, nil, ErrInvalidCredentials
	}
	u, err := c.ensureOIDCUser(ctx, email, info.Name)
	if err != nil {
		return nil, nil, err
	}
	pair, err := c.tokens.IssueTokens(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	return pair, u, nil
}

func (c *OIDCClient) discover(ctx context.Context, issuer string) (*oidcDiscovery, error) {
	issuer = strings.TrimRight(issuer, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery: status %d", resp.StatusCode)
	}
	var d oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return nil, errors.New("oidc discovery: missing endpoints")
	}
	return &d, nil
}

func (c *OIDCClient) exchangeCode(ctx context.Context, tokenURL string, dc config.OIDCDomainConfig, code string) (*oidcTokenResp, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", dc.RedirectURL)
	form.Set("client_id", dc.ClientID)
	form.Set("client_secret", dc.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc token: status %d: %s", resp.StatusCode, string(body))
	}
	var tr oidcTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func (c *OIDCClient) fetchUserInfo(ctx context.Context, endpoint, accessToken string) (*oidcUserInfo, error) {
	if endpoint == "" {
		return nil, errors.New("no userinfo endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo status %d", resp.StatusCode)
	}
	var info oidcUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func emailFromIDToken(idToken string) (*oidcUserInfo, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Sub   string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return &oidcUserInfo{Sub: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
}

func (c *OIDCClient) ensureOIDCUser(ctx context.Context, email, displayName string) (*storage.User, error) {
	u, err := c.store.GetUserByEmail(ctx, email)
	if err == nil {
		if !u.Enabled {
			return nil, ErrUserDisabled
		}
		if displayName != "" && u.DisplayName != displayName {
			_ = c.store.UpdateUserProfile(ctx, u.ID, displayName)
			u.DisplayName = displayName
		}
		return u, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return nil, ErrInvalidCredentials
	}
	local, domainName := email[:at], email[at+1:]
	dom, err := c.store.GetDomainByName(ctx, domainName)
	if err != nil {
		return nil, fmt.Errorf("oidc jit: domain %s: %w", domainName, err)
	}
	if displayName == "" {
		displayName = local
	}
	return c.store.CreateUser(ctx, &storage.User{
		TenantID:     dom.TenantID,
		DomainID:     dom.ID,
		Email:        email,
		LocalPart:    local,
		DisplayName:  displayName,
		PasswordHash: "",
		AuthSource:   "oidc",
		QuotaBytes:   0,
		Enabled:      true,
	})
}
