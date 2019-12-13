package internal

import (
	"context"
)

type (
	ipAddressCtxKey struct{}
)

// SetIPToContext sets IP value to ctx
func SetIPToContext(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipAddressCtxKey{}, ip)
}

// GetIPFromContext gets IP value from ctx
func GetIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ipAddressCtxKey{}).(string); ok {
		return ip
	}
	return ""
}
