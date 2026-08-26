package edge

import "context"

type bodyKey struct{}

func withBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, bodyKey{}, body)
}

func bodyFrom(ctx context.Context) ([]byte, bool) {
	body, ok := ctx.Value(bodyKey{}).([]byte)
	return body, ok
}
