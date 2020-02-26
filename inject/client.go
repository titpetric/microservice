package inject

import (
	"net"
	"net/http"
	"time"
)

// NewHTTPClient produces a configured http.Client
func NewHTTPClient() *http.Client {
	timeout := 10 * time.Second

	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: timeout,
		}).Dial,
		TLSHandshakeTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
