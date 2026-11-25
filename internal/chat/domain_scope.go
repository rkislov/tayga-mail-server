package chat

import "context"

type domainScopeKey struct{}

// WithDomainScope narrows an authenticated operation to a single domain. It
// grants no rights and is never populated from user-supplied query parameters.
func WithDomainScope(ctx context.Context, domainID string) context.Context {
	return context.WithValue(ctx, domainScopeKey{}, domainID)
}
func DomainScope(ctx context.Context) string {
	value, _ := ctx.Value(domainScopeKey{}).(string)
	return value
}

const channelDomainPredicate = ` AND (?='' OR EXISTS(SELECT 1 FROM users scope_owner WHERE scope_owner.id=c.creator_id AND scope_owner.domain_id=?))`
