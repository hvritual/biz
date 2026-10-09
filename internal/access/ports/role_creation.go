package ports

import "context"

// Only the trusted execution adapter sets this after a completed transport
// claim. A historical completion without a durable receipt must fail closed.
type roleCreationReplayKey struct{}

func WithRoleCreationReplay(ctx context.Context) context.Context {
	return context.WithValue(ctx, roleCreationReplayKey{}, true)
}

func IsRoleCreationReplay(ctx context.Context) bool {
	value, _ := ctx.Value(roleCreationReplayKey{}).(bool)
	return value
}
