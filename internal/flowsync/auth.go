package flowsync

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/storage"
)

type ctxKey int

const userKey ctxKey = 1

func withUser(ctx context.Context, u *storage.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func userFrom(ctx context.Context) (*storage.User, bool) {
	u, ok := ctx.Value(userKey).(*storage.User)
	return u, ok
}

func authMiddleware(authn *auth.Layer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := authenticate(r, authn)
		if err != nil || u == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="FlowSync", Bearer`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), u)))
	})
}

func authenticate(r *http.Request, authn *auth.Layer) (*storage.User, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return nil, auth.ErrInvalidCredentials
	}
	ctx := r.Context()
	lower := strings.ToLower(h)
	if strings.HasPrefix(lower, "bearer ") {
		return authn.AuthenticateToken(ctx, "", auth.ParseBearerToken(h))
	}
	if strings.HasPrefix(lower, "basic ") {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(h[6:]))
		if err != nil {
			return nil, auth.ErrInvalidCredentials
		}
		parts := strings.SplitN(string(raw), ":", 2)
		if len(parts) != 2 {
			return nil, auth.ErrInvalidCredentials
		}
		return authn.Authenticate(ctx, parts[0], parts[1])
	}
	return nil, auth.ErrInvalidCredentials
}
