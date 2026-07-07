package core

import "context"

// requestIDCtxKey carries the per-request correlation id (X-Request-ID) from the
// HTTP edge down into the service/repository layer so it can be stamped onto
// every audit_event without threading it through every function signature.
type requestIDCtxKey struct{}

// WithRequestID returns a context carrying the given request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// RequestIDFromContext returns the request id stored in ctx, or "" if none.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return id
	}
	return ""
}
