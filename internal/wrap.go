package internal

import (
	"strings"

	"net/http"
)

// WrapWithIP wraps a http.Handler to inject the client IP into the context
func WrapWithIP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get IP address
		ip := func() string {
			headers := []string{
				http.CanonicalHeaderKey("X-Forwarded-For"),
				http.CanonicalHeaderKey("X-Real-IP"),
			}
			for _, header := range headers {
				if addr := r.Header.Get(header); addr != "" {
					return strings.SplitN(addr, ", ", 2)[0]
				}
			}
			return strings.SplitN(r.RemoteAddr, ":", 2)[0]
		}()

		ctx := r.Context()
		ctx = SetIPToContext(ctx, ip)

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
