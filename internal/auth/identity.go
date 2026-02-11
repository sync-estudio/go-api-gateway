package auth

import "context"

type Identity struct {
	UserID string         // sub claim - the user's unique identifier
	Email  string         // email claim
	Roles  []string       // roles claim (if present)
	Claims map[string]any // all claims for flexibility
	Token  string         // original token (preserved for forwarding)
}

type contextKey string

const IdentityContextKey contextKey = "identity"

func FromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(IdentityContextKey).(*Identity)
	return id, ok
}

func ToContext(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, IdentityContextKey, id)
}
